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
	header := headerStyle.Render(
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
	rctx, cancel := context.WithCancel(ctx)
	defer cancel()
	p := tea.NewProgram(newModel(insts, initialFilter), tea.WithAltScreen(), tea.WithContext(rctx))
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
