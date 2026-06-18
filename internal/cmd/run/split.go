package run

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"

	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
	"github.com/un7qi3inc/un7qi3-cli/internal/output"
	"github.com/un7qi3inc/un7qi3-cli/internal/repocfg"
	"github.com/un7qi3inc/un7qi3-cli/internal/run"
)

// runSplit launches each proc in its own terminal panel using whatever native
// splitter wraps the session. When none is available it degrades to the
// background-log-file path (and says so), per the Phase 3 graceful-degradation
// principle.
//
// Each panel is a fresh shell, so it gets the cwd, the node PATH prepend, the
// profile env, and the (already {script}-substituted) cmd packed into one
// `cd … && export … && exec …` string.
func runSplit(w io.Writer, repo, repoDir string, p repocfg.Profile, profileEnv map[string]string, pathAdded string, env []string, splitDirFlag string, isTTY bool) error {
	mux := run.DetectMultiplexer()
	style := styleOf(mux)
	if style == styleNone {
		printNoneFallback(w)
		return runBackground(w, repo, repoDir, p, env)
	}

	// 방향(좌우/상하)은 패널 분할형 터미널에만 의미가 있다. 새 창형은 묻지 않는다.
	dir := splitCol
	if style == stylePane {
		d, err := resolveSplitDir(splitDirFlag, isTTY)
		if err != nil {
			return err
		}
		dir = d
	}

	fmt.Fprintf(w, "%s %s %s\n", output.Dim(splitVerb(mux)+":"), output.Cyan(string(mux)), output.Dim("("+layoutLabel(mux, dir)+")"))
	panels := buildPanels(repoDir, p.Procs, profileEnv, pathAdded)
	for _, pn := range panels {
		if _, err := os.Stat(pn.cwd); err != nil {
			return fmt.Errorf("[%s] cwd 가 없습니다: %s", pn.name, pn.cwd)
		}
		if err := openPanel(mux, pn, dir); err != nil {
			return fmt.Errorf("[%s] %s 실패: %w", pn.name, splitVerb(mux), err)
		}
		fmt.Fprintf(w, "  %s %s\n", output.Cyan("["+pn.name+"]"), output.Dim(pn.cwd))
	}
	if mux == run.MuxTmux {
		_, _ = uqexec.Run("tmux", "select-layout", "tiled")
	}
	return nil
}

// printSplitPlan renders the --dry-run view of what --split would do. It never
// prompts — direction comes from the flag (or default).
func printSplitPlan(w io.Writer, repoDir string, p repocfg.Profile, profileEnv map[string]string, pathAdded, splitDirFlag string) {
	mux := run.DetectMultiplexer()
	dir, _, _ := parseSplitDir(splitDirFlag) // flag already validated upstream
	fmt.Fprintf(w, "  분할 감지:     %s\n", mux)
	fmt.Fprintf(w, "  분할 방식:     %s\n", layoutLabel(mux, dir))
	if styleOf(mux) == styleNone {
		fmt.Fprintln(w, "  "+output.Dim("→ 패널 분할 미지원 → 백그라운드 로그파일로 분리 (fallback)"))
		return
	}
	noun := "패널"
	if styleOf(mux) == styleWindow {
		noun = "창"
	}
	fmt.Fprintf(w, "  %s 계획:\n", noun)
	for _, pn := range buildPanels(repoDir, p.Procs, profileEnv, pathAdded) {
		fmt.Fprintf(w, "    %s  %s\n", output.Cyan("["+pn.name+"]"), pn.shellLine(false))
	}
}

// splitDir is the visual layout of split panels, independent of each
// multiplexer's confusingly-inverted horizontal/vertical terminology.
type splitDir string

const (
	splitCol splitDir = "col" // 좌우 (panes side by side)
	splitRow splitDir = "row" // 상하 (panes stacked)
)

func (d splitDir) label() string {
	if d == splitRow {
		return "상하(row)"
	}
	return "좌우(col)"
}

// splitStyle is how a terminal realizes --split.
type splitStyle int

const (
	styleNone   splitStyle = iota // no native split → background fallback
	stylePane                     // directional pane split within one window
	styleWindow                   // a separate window per proc (no direction)
)

// styleOf classifies a multiplexer. tmux/cmux/iTerm2 split panes (and honor a
// direction); Ghostty/Terminal.app have no scriptable pane split, so each proc
// gets its own window; everything else has no native split.
func styleOf(mux run.Multiplexer) splitStyle {
	switch mux {
	case run.MuxTmux, run.MuxCmux, run.MuxITerm2:
		return stylePane
	case run.MuxGhostty, run.MuxAppleTerminal:
		return styleWindow
	default:
		return styleNone
	}
}

// splitVerb is the header noun for a multiplexer: pane-splitters "패널 분할",
// window-openers "새 창 분리".
func splitVerb(mux run.Multiplexer) string {
	if styleOf(mux) == styleWindow {
		return "새 창 분리"
	}
	return "패널 분할"
}

// layoutLabel describes the resulting layout. Window-style terminals open one
// window per proc, so direction (col/row) doesn't apply.
func layoutLabel(mux run.Multiplexer, d splitDir) string {
	switch styleOf(mux) {
	case styleWindow:
		return "각각 새 창으로"
	case stylePane:
		return d.label()
	default:
		return "분할 미지원"
	}
}

// splitMenuLabel is the run-mode menu line for the split option, worded for what
// the detected terminal will actually do.
func splitMenuLabel(mux run.Multiplexer) string {
	if styleOf(mux) == styleWindow {
		return "포그라운드 · 서버마다 새 창을 따로 띄우기"
	}
	return "포그라운드 · 화면을 패널로 나눠 따로 보기"
}

// parseSplitDir validates the --split-dir flag. The bool reports whether the
// user set it explicitly (so callers know whether to prompt).
func parseSplitDir(flag string) (splitDir, bool, error) {
	switch flag {
	case "":
		return splitCol, false, nil
	case "col":
		return splitCol, true, nil
	case "row":
		return splitRow, true, nil
	default:
		return "", false, fmt.Errorf("알 수 없는 --split-dir '%s' — col(좌우) 또는 row(상하)만 가능", flag)
	}
}

// resolveSplitDir decides the direction for an actual launch: the flag if set,
// else an interactive prompt on a TTY, else the default (col).
func resolveSplitDir(flag string, isTTY bool) (splitDir, error) {
	dir, explicit, err := parseSplitDir(flag)
	if err != nil {
		return "", err
	}
	if explicit || !isTTY {
		return dir, nil
	}
	var choice string
	if err := huh.NewSelect[string]().
		Title("패널을 어떻게 나눌까요?").
		Options(
			huh.NewOption("좌우로 나눔 (왼쪽 | 오른쪽)", "col"),
			huh.NewOption("상하로 나눔 (위 / 아래)", "row"),
		).
		Value(&choice).
		Run(); err != nil {
		return "", err
	}
	if choice == "row" {
		return splitRow, nil
	}
	return splitCol, nil
}

// printNoneFallback explains why a split request became a background launch.
func printNoneFallback(w io.Writer) {
	fmt.Fprintln(w, output.Dim("이 터미널은 패널 분할 미지원 → 백그라운드 로그파일로 분리합니다."))
	fmt.Fprintln(w, output.Dim("tmux/cmux/iTerm2/Ghostty/Terminal 안에서 실행하면 패널(또는 창)이 분리됩니다."))
}

type panel struct {
	name      string
	cwd       string
	pathAdded string
	env       map[string]string
	cmd       []string
}

// shellLine packs cwd + node PATH prepend + profile env + cmd into one
// `sh -c`-safe string. env keys are emitted in sorted order for determinism.
//
// useExec replaces the shell with the command (good for `sh -c` wrappers that
// would otherwise linger). Pass false when typing into an existing interactive
// shell — there we want the shell to survive the command so errors stay visible.
func (p panel) shellLine(useExec bool) string {
	var b strings.Builder
	// 패널 제목 + 컬러 배너로 어느 proc(back/front)인지 한눈에 구분. 이름은
	// printf 인자로 넘겨 따옴표 안전성을 지킨다.
	b.WriteString("printf '\\033]0;%s\\007' " + run.ShellQuote(p.name))
	b.WriteString(" && printf '\\033[1;36m▶ [%s]\\033[0m\\n\\n' " + run.ShellQuote(p.name))
	b.WriteString(" && cd " + run.ShellQuote(p.cwd))
	if p.pathAdded != "" {
		b.WriteString(" && export PATH=" + run.ShellQuote(p.pathAdded) + ":$PATH")
	}
	keys := make([]string, 0, len(p.env))
	for k := range p.env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(" && export " + k + "=" + run.ShellQuote(p.env[k]))
	}
	b.WriteString(" && ")
	if useExec {
		b.WriteString("exec ")
	}
	parts := make([]string, len(p.cmd))
	for i, a := range p.cmd {
		parts[i] = run.ShellQuote(a)
	}
	b.WriteString(strings.Join(parts, " "))
	return b.String()
}

// buildPanels turns each proc into a panel carrying everything a fresh shell
// needs to reproduce uq's environment.
func buildPanels(repoDir string, procs []repocfg.Proc, profileEnv map[string]string, pathAdded string) []panel {
	panels := make([]panel, 0, len(procs))
	for _, pr := range procs {
		cwd := repoDir
		if pr.Cwd != "" {
			cwd = filepath.Join(repoDir, pr.Cwd)
		}
		panels = append(panels, panel{
			name:      pr.Name,
			cwd:       cwd,
			pathAdded: pathAdded,
			env:       profileEnv,
			cmd:       pr.Cmd,
		})
	}
	return panels
}

// toRunPanel converts a local panel to a run.Panel for use with run.OpenPanel.
// useExec controls whether the shell line ends with exec (see shellLine).
func (p panel) toRunPanel(useExec bool) run.Panel {
	return run.Panel{Label: p.name, Command: p.shellLine(useExec)}
}

// toRunDir maps the local splitDir to the shared run.SplitDir enum.
func toRunDir(d splitDir) run.SplitDir {
	if d == splitRow {
		return run.SplitRow
	}
	return run.SplitCol
}

// openPanel spawns one panel via the detected multiplexer.
func openPanel(mux run.Multiplexer, pn panel, dir splitDir) error {
	// cmux/iTerm2: type into an existing interactive shell (useExec=false) so
	// the pane stays open when the command exits or errors.
	// tmux/Ghostty/Terminal: the shared opener uses the Command field directly.
	useExec := mux == run.MuxTmux
	rp := pn.toRunPanel(useExec)
	return run.OpenPanel(mux, rp, toRunDir(dir))
}
