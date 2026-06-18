# 0005 — Phase 5: uq logs 인터랙티브 뷰어 (TUI)

**Status:** 시작전

## Context

Phase 4 에서 `uq logs <repo>` 는 EB 인스턴스 로그를 merged/실시간으로 스트리밍한다. 하지만 보는 도중에는 조작이 불가능하다 — 필터를 바꾸려면 Ctrl+C 후 `--grep` 을 바꿔 재실행해야 하고, 흘러가는 로그를 멈춰 읽거나 특정 인스턴스만 보기가 안 된다.

이번 Phase 는 **"보면서 타이핑"** 경험을 준다: 실시간 스트림을 보면서 `/` 로 필터를 즉석에서 입력·변경하고, 일시정지로 멈춰 읽고, 인스턴스를 토글한다. bubbletea 기반 TUI.

### 조사 결과 — 코드/의존성 전제

- `internal/logs/mux.go` 의 `StreamMerged(ctx, w, src, t, env, insts, follow, lines, grep)` 가 각 인스턴스의 `eb` 프로세스를 고루틴으로 spawn 해 라인을 prefix·강조해 `w` 에 mutex 직렬화로 쓴다. TUI 는 이 라인들을 `w` 대신 모델로 받아야 한다 → **생산자/소비자 분리** 필요.
- `bubbletea`/`bubbles`/`lipgloss` 가 **이미 go.mod 에 (huh 통해 간접) 존재** → 신규 직접 의존성 부담 거의 없음(간접→직접 승격만).
- `PrefixLine(num, id, line)`(색상 prefix, id 앞5글자), `highlightLevel`(ERROR 빨강), `RenderLegend`, `shortID` 가 mux.go 에 있다 → 재사용.
- `term.IsTerminal(int(os.Stdout.Fd()))` 로 TTY 판정(run.go 패턴).

## 설계 원칙 (이번 Phase 추가)

16. **생산자/소비자 분리** — 로그 라인 생산(eb spawn)과 표현(평문/TUI)을 분리해 두 모드가 같은 스트림을 공유한다.
17. **TUI 는 TTY 한정, 평문이 기본 호환** — 파이프·리다이렉트·비follow 에서는 자동으로 평문으로 강등해 스크립트/CI 호환을 깨지 않는다.
18. **모델은 순수하게** — bubbletea Model 의 상태 전이를 합성 메시지로 단위테스트할 수 있게 한다(실 터미널·실 eb 불필요).

## 활성화 & 플래그

```
useTUI = TTY(stdout) && follow && !split && !dryRun && !plain
```

- `useTUI` 면 인터랙티브 뷰어, 아니면 기존 평문 스트리밍.
- 신규 플래그 `--plain` (bool, 기본 false): TTY 라도 평문 스트리밍 강제(파이프 없이 평문으로 보고 싶을 때).
- `--no-follow`/`--dry-run`/`--split` 은 본래대로 평문(TUI 아님).
- 비TTY(파이프/리다이렉트)는 자동 평문.

```
uq logs forceteller-api kr prod          # TTY → TUI
uq logs forceteller-api kr prod | grep X # 파이프 → 평문
uq logs forceteller-api kr prod --plain  # 강제 평문
uq logs forceteller-api kr prod --no-follow   # 평문(스냅샷)
```

## 아키텍처 / 데이터 흐름

```
StreamLines(ctx, src, t, env, insts, follow, lines, grep) (<-chan LogLine)
  └ 인스턴스마다 eb ssh 고루틴 → 라인 스캔 → LogLine 으로 채널 방출
    모든 고루틴 종료 시 채널 close

평문 모드: for ln := range ch { fmt.Fprintln(w, render(ln)) }   (StreamMerged 축소)
TUI 모드:  고루틴이 for ln := range ch { program.Send(logMsg(ln)) } → Model 갱신
```

`LogLine`(internal/logs/stream.go):

```go
type LineKind int

const (
	KindLog     LineKind = iota // 일반 로그 라인
	KindConnErr                 // 접속 실패
	KindEnd                     // 스트림 종료(eb wait 에러)
)

type LogLine struct {
	Num  int    // 인스턴스 번호(1-base)
	ID   string // EC2 인스턴스 id
	Text string // 본문(접속실패/종료 메시지 포함)
	Kind LineKind
}
```

`StreamLines` 는 현재 `StreamMerged` 의 고루틴/`eb` spawn 로직을 그대로 옮기되, `w` 에 쓰는 대신 `LogLine` 을 채널로 보낸다. 서버사이드 grep 은 `TailArgs` 가 이미 처리하므로 생산자는 grep 인자만 전달한다(클라이언트 추가 필터는 소비자/모델 책임).

## 평문 렌더러 (StreamMerged 축소)

```go
func StreamMerged(ctx, w, src, t, env, insts, follow, lines, grep) error {
	ch, err := StreamLines(...)
	if err != nil { return err }
	fmt.Fprint(w, RenderLegend(insts))
	for ln := range ch {
		fmt.Fprintln(w, renderLine(ln)) // Kind 에 따라 PrefixLine/색상
	}
	return nil
}
```

- `renderLine(LogLine)`: KindLog→`PrefixLine(Num,ID,highlightLevel(Text))`, KindConnErr→`PrefixLine(Num,ID,Red("접속 실패: "+Text))`, KindEnd→`PrefixLine(Num,ID,Dim("스트림 종료: "+Text))`.
- 평문 모드는 **Phase 4 와 동일하게** `--grep` 가 있으면 KindLog 라인에 클라이언트 재필터(`GrepMatch`)를 적용해 eb 로컬 경고(WARNING 등)를 함께 거른다. KindConnErr/KindEnd 는 필터 무관하게 항상 표시. (서버사이드 grep 으로 1차로 좁히고, 이 재필터로 로컬 노이즈 제거 — 동작 보존.)

## TUI 모델 (internal/logs/tui.go)

bubbletea Model:

```go
type model struct {
	insts    []Instance
	buf      []LogLine          // 링버퍼 (상한 maxBuf=5000)
	filter   string             // 라이브 필터(대소문자 무시 부분문자열)
	editing  bool               // 필터 입력 모드
	input    textinput.Model    // 필터 입력창
	hidden   map[int]bool       // 숨긴 인스턴스 번호
	paused   bool               // 일시정지(자동 하단스크롤 중지)
	vp       viewport.Model     // 스크롤 영역
}
```

### 메시지 / Update

- `logMsg(LogLine)`: `appendBuf`(상한 초과 시 앞에서 버림) → 보이는 줄이면 뷰포트 콘텐츠 갱신; `!paused` 면 하단으로 스크롤.
- `tea.KeyMsg`:
  - 편집 모드(`editing`): textinput 에 위임, 매 키 입력마다 `filter = input.Value()` 갱신 후 재필터. `enter`/`esc` → `editing=false`.
  - 일반 모드:
    - `/` → `editing=true`, input 포커스(기존 필터 프리필).
    - `space` → `paused` 토글.
    - `1`~`9` → 해당 번호 인스턴스 `hidden` 토글 후 재필터.
    - `up/down/pgup/pgdn` → 뷰포트 스크롤(일시정지 아니어도 가능; 스크롤하면 자동 하단추적 일시 해제).
    - `g`/`G` → 맨위/맨아래.
    - `q`/`ctrl+c` → `tea.Quit`.
- `tea.WindowSizeMsg`: 뷰포트 크기 재계산(헤더/풋터 높이 제외).

### 필터/가시성 (순수 함수)

```go
// visible 은 라인이 현재 필터·인스턴스 토글 기준으로 보여야 하는지.
func visible(ln LogLine, filter string, hidden map[int]bool) bool {
	if hidden[ln.Num] { return false }
	if filter == "" { return true }
	return strings.Contains(strings.ToLower(ln.Text), strings.ToLower(filter))
}

// viewContent 은 버퍼를 필터링해 렌더 문자열(줄바꿈 결합)로 만든다.
func viewContent(buf []LogLine, filter string, hidden map[int]bool) string
```

### View

- 헤더: app/env + 인스턴스 토글 상태(`[#1 09e13]✓ [#3 0ee47]✗`) + 상태(FOLLOW/PAUSED).
- 본문: `vp.View()` (필터된 줄, ERROR 빨강·prefix 색).
- 풋터: 편집 모드면 `/필터: <input>`, 아니면 키 안내(`space=일시정지 1-9=인스턴스 /=필터 q=종료`).
- lipgloss 로 헤더/풋터 스타일.

### RunTUI

```go
func RunTUI(ctx context.Context, ch <-chan LogLine, insts []Instance, initialFilter string) error {
	m := newModel(insts, initialFilter)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithContext(ctx))
	go func() { for ln := range ch { p.Send(logMsg(ln)) } }() // 채널 → 메시지
	_, err := p.Run()
	return err
}
```

- `--grep` 가 주어졌으면 `initialFilter` 로 프리필(서버사이드 grep 이 1차로 좁히고, 같은 값이 라이브 필터에도 노출).

## 명령 분기 (internal/cmd/logs/logs.go)

```go
useTUI := isStdoutTTY() && !noFollow && !split && !dryRun && !plain
...
if useTUI {
	ch, err := eblogs.StreamLines(cmd.Context(), src, tgt, env, insts, true, lines, grep)
	if err != nil { ... os.Exit(1) }
	return eblogs.RunTUI(cmd.Context(), ch, insts, grep)
}
// 기존 평문 경로(StreamMerged / split / dry-run)
```

- `isStdoutTTY` = `term.IsTerminal(int(os.Stdout.Fd()))`.
- 신규 `--plain` 플래그.

## 디렉토리 구조 (Phase 5 에 추가/변경)

```
internal/logs/stream.go        [신규] LogLine/LineKind + StreamLines 생산자
internal/logs/stream_test.go   [신규] (가능 범위) — 생산자는 실 eb 라 제한적; LogLine 헬퍼 위주
internal/logs/tui.go           [신규] bubbletea Model + visible/viewContent + RunTUI
internal/logs/tui_test.go      [신규] Model.Update 합성 메시지 상태전이 + visible/viewContent 단위테스트
internal/logs/mux.go           [변경] StreamMerged 를 StreamLines 소비 렌더러로 축소 + renderLine
internal/cmd/logs/logs.go      [변경] --plain, useTUI 분기, isStdoutTTY
go.mod                         [변경] bubbletea/bubbles/lipgloss 간접→직접
docs/0005-phase5-logs-tui.md   [신규] 본 문서
```

## 에러 코드 / 동작

- TUI 진입 실패(터미널 없음 등)는 발생하지 않게 useTUI 판정에서 거른다. bubbletea `p.Run()` 에러는 logs 명령이 exit 1 로 보고.
- aws/eb 발견 단계 에러는 Phase 4 와 동일(exit 4/1). TUI 는 발견·인스턴스 확정 이후에만 진입.
- 빈 스트림(접속 전부 실패)도 TUI 는 뜨고, 해당 인스턴스의 KindConnErr 라인이 보인다.

## 검증 (Verification)

```bash
go build ./...
go test ./internal/logs/... ./internal/cmd/...

# 분기 동작 (비TTY=평문 자동)
uq logs forceteller-api kr prod --no-follow | head        # 평문(파이프)
uq logs forceteller-api kr prod --plain --no-follow </dev/null   # 평문 강제

# 모델 단위테스트가 핵심 (tui_test.go):
#  - logMsg 주입 → buf 증가, 상한 5000 유지
#  - "/" → editing=true, 타이핑 → filter 갱신 → viewContent 가 매칭 줄만
#  - space → paused 토글
#  - "2" → hidden[2] 토글 → #2 라인 숨김
#  - visible()/viewContent() 표 기반 테스트(대소문자 무시 부분문자열)
```

> 실제 TUI 인터랙션(실 터미널+실 eb)은 자동 검증 제외. Model.Update 합성 메시지 + 순수 함수로 커버. 수동 확인: TTY 에서 `uq logs forceteller-api kr prod` 실행 후 `/`·space·숫자 키.

## 명시적으로 제외 (Phase 5)

- **스크롤백 내 검색(`/`로 위로 탐색)** — `/` 는 라이브 필터 전용. 별도 검색 네비게이션은 범위 밖(일시정지+스크롤로 대체).
- **정규식 라이브 필터** — 라이브 필터는 대소문자 무시 부분문자열. 정규식이 필요하면 초기 `--grep`(서버사이드) 사용.
- **라이브 필터의 서버사이드 재질의** — 타이핑 필터는 버퍼/유입 라인에 클라이언트단으로만 적용. 초기 서버사이드 grep 으로 못 받은 라인은 재실행 필요.
- **마우스 조작 / 라인 복사 UI** — 키보드만. 복사는 터미널 기본 기능.
- **세션 중 인스턴스 재발견** — Phase 4 와 동일하게 시작 시 스냅샷.
- **로그 저장/내보내기** — 범위 밖(평문 모드 + 리다이렉트로 대체).
