package logs

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

const maxBuf = 5000

const (
	headerH = 2
	footerH = 1
)

var headerStyle = lipgloss.NewStyle().Bold(true)

type logMsg LogLine

type model struct {
	insts        []Instance
	buf          []LogLine
	filter       string
	editing      bool
	input        textinput.Model
	solo         int // 0=전체, N=#N 만 표시
	paused       bool
	vp           viewport.Model
	ready        bool
	app          string
	env          string
	userScrolled bool
}

func newModel(insts []Instance, initialFilter, app, env string) model {
	ti := textinput.New()
	ti.Prompt = ""      // 우리가 "/필터: " 라벨을 직접 붙이므로 기본 "> " 프롬프트 제거
	ti.Placeholder = "" // 빈 입력 시 placeholder 가 "/필터: " 뒤에 노출되는 혼동 방지
	ti.SetValue(initialFilter)
	return model{
		insts:  insts,
		filter: initialFilter,
		input:  ti,
		app:    app,
		env:    env,
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m *model) refresh() {
	if m.ready {
		m.vp.SetContent(viewContent(m.buf, m.filter, m.solo))
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case logMsg:
		m.buf = appendBuf(m.buf, LogLine(msg))
		m.refresh()
		if !m.paused && !m.userScrolled && m.ready {
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
			m.userScrolled = true
			return m, nil
		case "G":
			m.vp.GotoBottom()
			m.userScrolled = false // 맨 아래로 = 자동추적 재개
			return m, nil
		case "up", "down", "pgup", "pgdown", "k", "j", "home", "end":
			m.userScrolled = true
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		}
		if r := msg.String(); len(r) == 1 && r[0] >= '1' && r[0] <= '9' {
			n := int(r[0] - '0')
			if m.solo == n {
				m.solo = 0 // 같은 번호 다시 → 전체 복귀
			} else {
				m.solo = n // 해당 인스턴스만
			}
			m.userScrolled = false // 솔로 전환 시 자동추적 재개
			m.refresh()
			if m.ready {
				m.vp.GotoBottom() // 선택 인스턴스의 실시간 하단으로 점프
			}
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
	status := "실시간"
	if m.paused {
		status = "일시정지"
	}
	var toggles strings.Builder
	for _, in := range m.insts {
		mark := "✓"
		if m.solo != 0 && m.solo != in.Num {
			mark = "✗"
		}
		// 각 인스턴스 토글을 그 인스턴스 색으로 — 상단 색과 로그 줄 [#k] 색이 일치.
		fmt.Fprint(&toggles, colorNum(in.Num, fmt.Sprintf("[#%d %s]", in.Num, shortID(in.ID)))+mark+" ")
	}
	header := headerStyle.Render(fmt.Sprintf("uq logs  %s · %s  [%s]  ", m.app, m.env, status)) +
		toggles.String()

	var footer string
	if m.editing {
		footer = "/필터: " + m.input.View()
	} else {
		footer = output.Dim("space=일시정지  1-9=인스턴스  /=필터  g/G=처음/끝  q=종료")
	}
	return header + "\n" + m.vp.View() + "\n" + footer
}

// RunTUI 는 채널의 LogLine 을 bubbletea 프로그램에 주입하며 TUI 를 실행한다.
func RunTUI(ctx context.Context, ch <-chan LogLine, insts []Instance, initialFilter, app, env string) error {
	rctx, cancel := context.WithCancel(ctx)
	defer cancel()
	p := tea.NewProgram(newModel(insts, initialFilter, app, env), tea.WithAltScreen(), tea.WithContext(rctx))
	go func() {
		for {
			select {
			case <-rctx.Done():
				return
			case ln, ok := <-ch:
				if !ok {
					return
				}
				p.Send(logMsg(ln))
			}
		}
	}()
	_, err := p.Run()
	return err
}

// visible 은 라인이 현재 필터·솔로 기준으로 보여야 하는지.
// solo 가 0 이면 전체, N 이면 #N 인스턴스만.
func visible(ln LogLine, filter string, solo int) bool {
	if solo != 0 && ln.Num != solo {
		return false
	}
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(ln.Text), strings.ToLower(filter))
}

// viewContent 는 버퍼를 필터링해 렌더된 줄들을 줄바꿈으로 결합한다.
func viewContent(buf []LogLine, filter string, solo int) string {
	var b strings.Builder
	first := true
	for _, ln := range buf {
		if !visible(ln, filter, solo) {
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
