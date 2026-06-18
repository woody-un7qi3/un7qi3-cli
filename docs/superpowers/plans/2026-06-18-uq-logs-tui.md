# uq logs 인터랙티브 뷰어(TUI) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `uq logs <repo>` 가 TTY 에서 bubbletea TUI 로 실시간 로그를 보여주며 라이브 필터·일시정지·인스턴스 토글을 지원한다.

**Architecture:** 로그 라인 생산(eb ssh spawn)을 `StreamLines` 생산자로 분리해 `LogLine` 채널을 내보내고, 평문 렌더러(`StreamMerged`)와 bubbletea TUI 가 같은 채널을 소비한다. TUI 는 링버퍼 + 클라이언트 필터로 즉시 화면을 갱신한다.

**Tech Stack:** Go, bubbletea/bubbles(viewport,textinput)/lipgloss(이미 go.mod 간접 의존), 기존 `internal/logs`.

## Global Constraints

- 커밋 메시지 **한글**, AI 표기 금지.
- PostToolUse 훅이 .go 수정 시 `make install` 자동 실행 — 빌드 실패 시 고친 뒤 진행.
- 패키지 충돌: `internal/cmd/logs` 와 `internal/logs` 둘 다 `package logs` — 명령 파일에서 라이브러리는 `eblogs` 별칭으로 import.
- 활성화: `useTUI = term.IsTerminal(stdout) && follow(!noFollow) && !split && !dryRun && !plain`.
- 라이브 필터: 대소문자 무시 **부분문자열**(정규식 아님).
- 링버퍼 상한 `maxBuf = 5000`.
- 색상/식별: 기존 `PrefixLine`(id 앞5글자), `highlightLevel`(ERROR 빨강), `shortID` 재사용.
- 외부 명령 spawn 은 테스트 주입 가능하게(`execCommand` 변수).

---

## File Structure

```
internal/logs/stream.go        [신규] LogLine/LineKind + StreamLines 생산자 + execCommand 주입점
internal/logs/stream_test.go   [신규] StreamLines 가 채널로 LogLine 방출(fake execCommand)
internal/logs/mux.go           [변경] StreamMerged → StreamLines 소비 평문 렌더러 + renderLine
internal/logs/tui.go           [신규] visible/viewContent/appendBuf + bubbletea Model + RunTUI
internal/logs/tui_test.go      [신규] 순수함수 + Model.Update 합성 메시지 상태전이
internal/cmd/logs/logs.go      [변경] --plain, isStdoutTTY, useTUI 분기
```

---

## Task 1: LogLine 타입 + StreamLines 생산자

**Files:**
- Create: `internal/logs/stream.go`
- Test: `internal/logs/stream_test.go`

**Interfaces:**
- Consumes: `Source.TailArgs`(기존), `Target`, `Instance`(기존).
- Produces:
  - `type LineKind int` + `const (KindLog LineKind = iota; KindConnErr; KindEnd)`
  - `type LogLine struct { Num int; ID string; Text string; Kind LineKind }`
  - `func StreamLines(ctx context.Context, src Source, t Target, env string, insts []Instance, follow bool, lines int, grep string) <-chan LogLine`
  - `var execCommand = exec.CommandContext` (테스트 주입점)

- [ ] **Step 1: Write the failing test**

`internal/logs/stream_test.go`:

```go
package logs

import (
	"context"
	"os/exec"
	"testing"
)

// fakeSource 는 TailArgs 가 셸로 두 줄을 출력하게 만들어 StreamLines 를 테스트한다.
type fakeSource struct{}

func (fakeSource) Environments(Target) ([]string, error)        { return nil, nil }
func (fakeSource) Instances(Target, string) ([]Instance, error) { return nil, nil }
func (fakeSource) TailArgs(t Target, env string, inst Instance, follow bool, lines int, grep string) []string {
	return []string{"printf", "line-a\\nline-b\\n"}
}

func TestStreamLinesEmitsLines(t *testing.T) {
	// execCommand 를 sh 로 바꿔 args 를 그대로 실행(첫 인자=프로그램).
	orig := execCommand
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, args[0], args[1:]...)
	}
	defer func() { execCommand = orig }()

	insts := []Instance{{ID: "i-aaa", Num: 1}}
	ch := StreamLines(context.Background(), fakeSource{}, Target{}, "env", insts, false, 10, "")

	var texts []string
	for ln := range ch {
		if ln.Kind == KindLog {
			texts = append(texts, ln.Text)
			if ln.Num != 1 || ln.ID != "i-aaa" {
				t.Errorf("LogLine 메타 틀림: %+v", ln)
			}
		}
	}
	if len(texts) != 2 || texts[0] != "line-a" || texts[1] != "line-b" {
		t.Errorf("방출 라인 틀림: %v", texts)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/logs/ -run TestStreamLines -v`
Expected: 컴파일 실패(`StreamLines`/`LogLine`/`execCommand` 미정의).

- [ ] **Step 3: Implement stream.go**

`internal/logs/stream.go`:

```go
package logs

import (
	"bufio"
	"context"
	"os/exec"
	"sync"
)

// LineKind 는 LogLine 의 종류.
type LineKind int

const (
	KindLog     LineKind = iota // 일반 로그 라인
	KindConnErr                 // 접속 실패
	KindEnd                     // 스트림 종료(eb wait 에러)
)

// LogLine 은 한 인스턴스에서 온 로그 라인 한 줄(또는 상태 메시지).
type LogLine struct {
	Num  int
	ID   string
	Text string
	Kind LineKind
}

// execCommand 는 테스트에서 주입 가능한 명령 생성자.
var execCommand = exec.CommandContext

// StreamLines 는 각 인스턴스의 eb 프로세스를 spawn 해 LogLine 을 채널로 방출한다.
// 모든 인스턴스 스트림이 끝나면 채널을 닫는다. 인스턴스별 실패는 KindConnErr/KindEnd
// 로 격리되어 다른 인스턴스에 영향을 주지 않는다.
func StreamLines(ctx context.Context, src Source, t Target, env string,
	insts []Instance, follow bool, lines int, grep string) <-chan LogLine {

	ch := make(chan LogLine, 256)
	var wg sync.WaitGroup
	for _, in := range insts {
		wg.Add(1)
		go func(in Instance) {
			defer wg.Done()
			args := src.TailArgs(t, env, in, follow, lines, grep)
			cmd := execCommand(ctx, "eb", args...)
			stdout, err := cmd.StdoutPipe()
			cmd.Stderr = cmd.Stdout
			if err == nil {
				err = cmd.Start()
			}
			if err != nil {
				ch <- LogLine{Num: in.Num, ID: in.ID, Text: err.Error(), Kind: KindConnErr}
				return
			}
			sc := bufio.NewScanner(stdout)
			sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for sc.Scan() {
				ch <- LogLine{Num: in.Num, ID: in.ID, Text: sc.Text(), Kind: KindLog}
			}
			if err := cmd.Wait(); err != nil {
				ch <- LogLine{Num: in.Num, ID: in.ID, Text: err.Error(), Kind: KindEnd}
			}
		}(in)
	}
	go func() { wg.Wait(); close(ch) }()
	return ch
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/logs/ -run TestStreamLines -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/logs/stream.go internal/logs/stream_test.go
git commit -m "feat(logs): LogLine + StreamLines 생산자 분리(채널 방출)"
```

---

## Task 2: 평문 렌더러 — StreamMerged 를 StreamLines 소비로 축소

**Files:**
- Modify: `internal/logs/mux.go`

**Interfaces:**
- Consumes: `StreamLines`(Task 1), `LogLine`, `PrefixLine`/`highlightLevel`/`RenderLegend`/`GrepMatch`(기존).
- Produces: `func renderLine(ln LogLine) string` (TUI 도 재사용).

> `StreamMerged` 의 시그니처는 그대로 유지(`ctx, w, src, t, env, insts, follow, lines, grep`). 내부만 교체. 동작은 Phase 4 와 동일해야 한다.

- [ ] **Step 1: Write the failing test**

`internal/logs/mux_test.go` 에 추가:

```go
func TestRenderLine(t *testing.T) {
	got := renderLine(LogLine{Num: 1, ID: "i-0abc123", Text: "hello", Kind: KindLog})
	if !strings.Contains(got, "0abc1") || !strings.Contains(got, "hello") {
		t.Errorf("KindLog 렌더 틀림: %q", got)
	}
	ce := renderLine(LogLine{Num: 2, ID: "i-x", Text: "boom", Kind: KindConnErr})
	if !strings.Contains(ce, "접속 실패") || !strings.Contains(ce, "boom") {
		t.Errorf("KindConnErr 렌더 틀림: %q", ce)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/logs/ -run TestRenderLine -v`
Expected: 컴파일 실패(`renderLine` 미정의).

- [ ] **Step 3: mux.go — renderLine 추가 + StreamMerged 축소**

`internal/logs/mux.go` 의 `StreamMerged` 함수 전체를 아래로 교체하고 `renderLine` 추가:

```go
// renderLine 은 LogLine 을 색상 prefix 가 붙은 한 줄로 렌더한다(평문/TUI 공용).
func renderLine(ln LogLine) string {
	switch ln.Kind {
	case KindConnErr:
		return PrefixLine(ln.Num, ln.ID, output.Red("접속 실패: ")+ln.Text)
	case KindEnd:
		return PrefixLine(ln.Num, ln.ID, output.Dim("스트림 종료: "+ln.Text))
	default:
		return PrefixLine(ln.Num, ln.ID, highlightLevel(ln.Text))
	}
}

// StreamMerged 는 StreamLines 를 소비해 한 화면에 합쳐 출력한다(평문 모드).
// --grep 가 있으면 KindLog 라인에 클라이언트 재필터를 적용해 eb 로컬 경고를 거른다.
func StreamMerged(ctx context.Context, w io.Writer, src Source, t Target, env string,
	insts []Instance, follow bool, lines int, grep string) error {

	var re *regexp.Regexp
	if grep != "" {
		var err error
		if re, err = regexp.Compile(grep); err != nil {
			return fmt.Errorf("--grep 정규식 오류: %w", err)
		}
	}
	fmt.Fprint(w, RenderLegend(insts))
	for ln := range StreamLines(ctx, src, t, env, insts, follow, lines, grep) {
		if ln.Kind == KindLog && !GrepMatch(re, ln.Text) {
			continue
		}
		fmt.Fprintln(w, renderLine(ln))
	}
	return nil
}
```

이제 mux.go 에서 더 이상 쓰지 않는 import(`bufio`, `os/exec`, `sync`)를 제거한다. (`context`, `fmt`, `io`, `regexp`, `strings`, `output` 는 계속 사용.)

- [ ] **Step 4: Run test + 회귀 확인**

Run: `go build ./... && go test ./internal/logs/ -v`
Expected: 전부 PASS (기존 TailArgs/그밖 테스트 + TestRenderLine).

- [ ] **Step 5: 평문 동작 수동 확인**

Run: `make install && uq logs forceteller-api kr beta --no-follow --plain </dev/null 2>&1 | head -5`
Expected: 범례 + `[#k 단축id]` prefix 라인 (Phase 4 와 동일 형태). (단 --plain 은 Task 5 에서 추가되므로 이 단계에서는 `uq logs forceteller-api kr beta --no-follow | head -5` 로 확인.)

- [ ] **Step 6: Commit**

```bash
git add internal/logs/mux.go internal/logs/mux_test.go
git commit -m "refactor(logs): StreamMerged 를 StreamLines 소비 렌더러로 축소"
```

---

## Task 3: TUI 순수 로직 — visible / viewContent / appendBuf

**Files:**
- Create: `internal/logs/tui.go`
- Test: `internal/logs/tui_test.go`

**Interfaces:**
- Consumes: `LogLine`(Task 1), `renderLine`(Task 2).
- Produces:
  - `const maxBuf = 5000`
  - `func visible(ln LogLine, filter string, hidden map[int]bool) bool`
  - `func viewContent(buf []LogLine, filter string, hidden map[int]bool) string`
  - `func appendBuf(buf []LogLine, ln LogLine) []LogLine`

- [ ] **Step 1: Write the failing test**

`internal/logs/tui_test.go`:

```go
package logs

import (
	"strings"
	"testing"
)

func TestVisible(t *testing.T) {
	ln := LogLine{Num: 1, ID: "i-a", Text: "x ERROR y", Kind: KindLog}
	if !visible(ln, "", nil) {
		t.Error("필터 없으면 보여야 함")
	}
	if !visible(ln, "error", nil) {
		t.Error("대소문자 무시 부분문자열 매칭")
	}
	if visible(ln, "zzz", nil) {
		t.Error("미매칭이면 숨김")
	}
	if visible(ln, "", map[int]bool{1: true}) {
		t.Error("숨긴 인스턴스는 안 보임")
	}
}

func TestViewContentFiltersAndJoins(t *testing.T) {
	buf := []LogLine{
		{Num: 1, ID: "i-a", Text: "alpha", Kind: KindLog},
		{Num: 2, ID: "i-b", Text: "beta", Kind: KindLog},
	}
	out := viewContent(buf, "alpha", nil)
	if !strings.Contains(out, "alpha") || strings.Contains(out, "beta") {
		t.Errorf("필터 결과 틀림:\n%s", out)
	}
}

func TestAppendBufCaps(t *testing.T) {
	var buf []LogLine
	for i := 0; i < maxBuf+10; i++ {
		buf = appendBuf(buf, LogLine{Num: 1, Text: "x", Kind: KindLog})
	}
	if len(buf) != maxBuf {
		t.Errorf("상한 %d 기대, 실제 %d", maxBuf, len(buf))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/logs/ -run "TestVisible|TestViewContent|TestAppendBuf" -v`
Expected: 컴파일 실패.

- [ ] **Step 3: tui.go — 순수 함수 구현**

`internal/logs/tui.go` (이 태스크에서는 순수 함수만; Model 은 Task 4):

```go
package logs

import "strings"

const maxBuf = 5000

// visible 은 라인이 현재 필터·인스턴스 토글 기준으로 보여야 하는지.
func visible(ln LogLine, filter string, hidden map[int]bool) bool {
	if hidden[ln.Num] {
		return false
	}
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(ln.Text), strings.ToLower(filter))
}

// viewContent 는 버퍼를 필터링해 렌더된 줄들을 줄바꿈으로 결합한다.
func viewContent(buf []LogLine, filter string, hidden map[int]bool) string {
	var b strings.Builder
	first := true
	for _, ln := range buf {
		if !visible(ln, filter, hidden) {
			continue
		}
		if !first {
			b.WriteByte('\n')
		}
		b.WriteString(renderLine(ln))
		first = false
	}
	return b.String()
}

// appendBuf 는 링버퍼에 추가하되 maxBuf 를 넘으면 앞에서 버린다.
func appendBuf(buf []LogLine, ln LogLine) []LogLine {
	buf = append(buf, ln)
	if len(buf) > maxBuf {
		buf = buf[len(buf)-maxBuf:]
	}
	return buf
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/logs/ -run "TestVisible|TestViewContent|TestAppendBuf" -v`
Expected: PASS (3 테스트)

- [ ] **Step 5: Commit**

```bash
git add internal/logs/tui.go internal/logs/tui_test.go
git commit -m "feat(logs): TUI 순수 로직(visible/viewContent/appendBuf) 추가"
```

---

## Task 4: TUI Model + Update/View + RunTUI

**Files:**
- Modify: `internal/logs/tui.go`
- Modify: `internal/logs/tui_test.go`
- Modify: `go.mod`/`go.sum` (bubbletea/bubbles/lipgloss 간접→직접; `go mod tidy`)

**Interfaces:**
- Consumes: `Instance`(기존), `LogLine`(Task 1), `viewContent`/`appendBuf`(Task 3), bubbletea/bubbles.
- Produces:
  - `type logMsg LogLine`
  - `type model struct {...}`
  - `func newModel(insts []Instance, initialFilter string) model`
  - `func (m model) Init() tea.Cmd` / `Update(tea.Msg) (tea.Model, tea.Cmd)` / `View() string`
  - `func RunTUI(ctx context.Context, ch <-chan LogLine, insts []Instance, initialFilter string) error`

- [ ] **Step 1: Write the failing test**

`internal/logs/tui_test.go` 에 추가:

```go
import (
	tea "github.com/charmbracelet/bubbletea"
)

func sendKey(m model, s string) model {
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	return nm.(model)
}

func TestModelLogMsgAppends(t *testing.T) {
	m := newModel([]Instance{{Num: 1, ID: "i-a"}}, "")
	nm, _ := m.Update(logMsg{Num: 1, ID: "i-a", Text: "hello", Kind: KindLog})
	if len(nm.(model).buf) != 1 {
		t.Fatalf("buf 1 기대, 실제 %d", len(nm.(model).buf))
	}
}

func TestModelSlashEntersEditing(t *testing.T) {
	m := newModel(nil, "")
	m = sendKey(m, "/")
	if !m.editing {
		t.Error("/ 누르면 편집 모드여야 함")
	}
}

func TestModelSpaceTogglesPause(t *testing.T) {
	m := newModel(nil, "")
	m = sendKey(m, " ")
	if !m.paused {
		t.Error("space 누르면 일시정지")
	}
	m = sendKey(m, " ")
	if m.paused {
		t.Error("다시 누르면 재개")
	}
}

func TestModelDigitTogglesInstance(t *testing.T) {
	m := newModel([]Instance{{Num: 2, ID: "i-b"}}, "")
	m = sendKey(m, "2")
	if !m.hidden[2] {
		t.Error("숫자 키로 인스턴스 숨김")
	}
	m = sendKey(m, "2")
	if m.hidden[2] {
		t.Error("다시 누르면 표시")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/logs/ -run TestModel -v`
Expected: 컴파일 실패(`newModel`/`model`/`logMsg` 미정의).

- [ ] **Step 3: tui.go — Model/Update/View/RunTUI 추가**

`internal/logs/tui.go` 상단 import 를 확장하고 아래를 추가:

```go
import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

const (
	headerH = 2
	footerH = 1
)

type logMsg LogLine

type model struct {
	insts   []Instance
	buf     []LogLine
	filter  string
	editing bool
	input   textinput.Model
	hidden  map[int]bool
	paused  bool
	vp      viewport.Model
	ready   bool
}

func newModel(insts []Instance, initialFilter string) model {
	ti := textinput.New()
	ti.Placeholder = "필터 (대소문자 무시 부분문자열)"
	ti.SetValue(initialFilter)
	return model{
		insts:  insts,
		filter: initialFilter,
		input:  ti,
		hidden: map[int]bool{},
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m *model) refresh() {
	if m.ready {
		m.vp.SetContent(viewContent(m.buf, m.filter, m.hidden))
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case logMsg:
		m.buf = appendBuf(m.buf, LogLine(msg))
		m.refresh()
		if !m.paused && m.ready {
			m.vp.GotoBottom()
		}
		return m, nil

	case tea.WindowSizeMsg:
		if !m.ready {
			m.vp = viewport.New(msg.Width, msg.Height-headerH-footerH)
			m.ready = true
		} else {
			m.vp.Width = msg.Width
			m.vp.Height = msg.Height - headerH - footerH
		}
		m.refresh()
		m.vp.GotoBottom()
		return m, nil

	case tea.KeyMsg:
		if m.editing {
			switch msg.Type {
			case tea.KeyEnter, tea.KeyEsc:
				m.editing = false
				m.input.Blur()
				return m, nil
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				m.filter = m.input.Value()
				m.refresh()
				return m, cmd
			}
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "/":
			m.editing = true
			m.input.SetValue(m.filter)
			m.input.CursorEnd()
			return m, m.input.Focus()
		case " ":
			m.paused = !m.paused
			return m, nil
		case "g":
			m.vp.GotoTop()
			return m, nil
		case "G":
			m.vp.GotoBottom()
			return m, nil
		}
		if r := msg.String(); len(r) == 1 && r[0] >= '1' && r[0] <= '9' {
			n := int(r[0] - '0')
			if m.hidden[n] {
				delete(m.hidden, n)
			} else {
				m.hidden[n] = true
			}
			m.refresh()
			return m, nil
		}
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) View() string {
	if !m.ready {
		return "로딩 중…"
	}
	status := "FOLLOW"
	if m.paused {
		status = "PAUSED"
	}
	var toggles strings.Builder
	for _, in := range m.insts {
		mark := "✓"
		if m.hidden[in.Num] {
			mark = "✗"
		}
		fmt.Fprintf(&toggles, "[#%d %s]%s ", in.Num, shortID(in.ID), mark)
	}
	header := lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("uq logs  %s  %s", status, toggles.String()))

	var footer string
	if m.editing {
		footer = "/필터: " + m.input.View()
	} else {
		footer = output.Dim("space=일시정지  1-9=인스턴스  /=필터  g/G=처음/끝  q=종료")
	}
	return header + "\n" + m.vp.View() + "\n" + footer
}

// RunTUI 는 채널의 LogLine 을 bubbletea 프로그램에 주입하며 TUI 를 실행한다.
func RunTUI(ctx context.Context, ch <-chan LogLine, insts []Instance, initialFilter string) error {
	p := tea.NewProgram(newModel(insts, initialFilter), tea.WithAltScreen(), tea.WithContext(ctx))
	go func() {
		for ln := range ch {
			p.Send(logMsg(ln))
		}
	}()
	_, err := p.Run()
	return err
}
```

> 주의: `m.refresh()` 는 포인터 리시버지만 `Update` 는 값 리시버다. 값 리시버 안에서 `m.refresh()` 호출은 `(&m).refresh()` 로 자동 처리되어 로컬 복사본 `m` 의 `vp` 를 갱신하고, 그 `m` 을 반환하므로 정상 동작한다.

- [ ] **Step 4: go mod tidy (간접→직접 승격)**

Run: `go mod tidy && go build ./...`
Expected: `go.mod` 에서 bubbletea/bubbles/lipgloss 의 `// indirect` 주석이 사라지고 빌드 성공.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/logs/ -run TestModel -v`
Expected: PASS (4 테스트)

- [ ] **Step 6: Commit**

```bash
git add internal/logs/tui.go internal/logs/tui_test.go go.mod go.sum
git commit -m "feat(logs): bubbletea TUI 모델(라이브필터·일시정지·인스턴스토글) 추가"
```

---

## Task 5: 명령 분기 — --plain / useTUI

**Files:**
- Modify: `internal/cmd/logs/logs.go`

**Interfaces:**
- Consumes: `eblogs.StreamLines`/`RunTUI`(Task 1,4), `term.IsTerminal`, 기존 오케스트레이션.

- [ ] **Step 1: --plain 플래그 + 변수 추가**

`internal/cmd/logs/logs.go` 의 var 블록에 `plain bool` 추가, 플래그 등록부에 추가:

```go
cmd.Flags().BoolVar(&plain, "plain", false, "TTY 라도 평문 스트리밍 강제 (TUI 끄기)")
```

(var 블록: `instanceNum, grep, noFollow, split, dryRun, linesN, plain`.)

- [ ] **Step 2: isStdoutTTY 헬퍼 + useTUI 분기**

import 에 `"golang.org/x/term"` 와 `"os"` 가 있는지 확인(run.go 가 이미 사용). `runLogs` 의 "6. dry-run / split / merged" 블록에서 merged 직전에 TUI 분기를 넣는다:

```go
	// 6. dry-run / split / TUI / merged
	lines := linesN
	if lines < 1 {
		lines = 1
	}
	if dryRun {
		return printLogsPlan(w, tgt, env, insts, src, !noFollow, lines, grep)
	}
	if split {
		mux := run.DetectMultiplexer()
		if eblogs.SplitSupported(mux) {
			return runLogsSplit(w, src, tgt, env, insts, mux, !noFollow, lines, grep)
		}
		fmt.Fprintln(w, output.Yellow("⚠"), "현재 터미널은 분할 미지원 — merged 로 진행")
	}
	if isStdoutTTY() && !noFollow && !plain {
		ch := eblogs.StreamLines(cmd.Context(), src, tgt, env, insts, true, lines, grep)
		return eblogs.RunTUI(cmd.Context(), ch, insts, grep)
	}
	return eblogs.StreamMerged(cmd.Context(), w, src, tgt, env, insts, !noFollow, lines, grep)
}

// isStdoutTTY 는 표준출력이 터미널인지.
func isStdoutTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}
```

- [ ] **Step 3: 빌드 + 분기 동작 확인**

Run:
```bash
go build ./... && go test ./...
make install
uq logs forceteller-api kr beta --no-follow | head -3       # 평문(비follow)
uq logs forceteller-api kr beta --plain --no-follow </dev/null | head -3   # 평문 강제
uq logs --help | grep -- --plain                             # 플래그 노출
```
Expected: 빌드/테스트 통과, 평문 경로 정상, `--plain` 노출. (TUI 자체는 실제 TTY 에서 수동 확인.)

- [ ] **Step 4: TUI 수동 확인 (실 TTY)**

Run: `uq logs forceteller-api kr prod`
Expected: 풀스크린 TUI, `/` 필터·space 일시정지·숫자 토글·q 종료 동작. (자동 검증 아님 — 수동.)

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/logs/logs.go
git commit -m "feat(logs): TTY 에서 TUI 자동 진입 + --plain 강제 평문"
```

---

## Self-Review 기록

- **Spec coverage**: 활성화/--plain(T5)·생산자분리 StreamLines(T1)·평문 렌더러 보존(T2)·순수필터(T3)·Model 라이브필터·일시정지·토글·RunTUI(T4)·ERROR 빨강/prefix 재사용(T2 renderLine). 제외항목(스크롤백검색·정규식 라이브필터·서버 재질의·마우스·재발견·저장)은 미구현 유지.
- **Placeholder scan**: 각 코드 스텝에 실제 Go 코드 포함. 수동 검증(실 TTY/실 eb)은 명시적으로 분리.
- **Type consistency**: `LogLine{Num,ID,Text,Kind}`/`LineKind`(T1) → T2 renderLine, T3 visible/viewContent, T4 logMsg 에서 동일 사용. `StreamLines` 시그니처(에러 없음) → T2·T5 호출 일치. `renderLine`(T2) → T3 viewContent 재사용. `model` 필드 → T4 테스트와 일치.
- **확인 필요(구현 중)**: ① bubbles `textinput.Focus()`/`viewport` API 버전별 시그니처(go.mod 버전 기준 보정) ② `tea.WithContext` 존재(bubbletea v1.3.6 에 있음) ③ value-receiver Update 의 refresh 포인터 처리(주의 메모 참조).
