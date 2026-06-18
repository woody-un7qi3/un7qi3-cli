package run

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

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
	b.WriteString("printf '\\033]0;%s\\007' " + shellQuote(p.name))
	b.WriteString(" && printf '\\033[1;36m▶ [%s]\\033[0m\\n\\n' " + shellQuote(p.name))
	b.WriteString(" && cd " + shellQuote(p.cwd))
	if p.pathAdded != "" {
		b.WriteString(" && export PATH=" + shellQuote(p.pathAdded) + ":$PATH")
	}
	keys := make([]string, 0, len(p.env))
	for k := range p.env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString(" && export " + k + "=" + shellQuote(p.env[k]))
	}
	b.WriteString(" && ")
	if useExec {
		b.WriteString("exec ")
	}
	parts := make([]string, len(p.cmd))
	for i, a := range p.cmd {
		parts[i] = shellQuote(a)
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

// openPanel spawns one panel via the detected multiplexer.
//
// Each multiplexer inverts horizontal/vertical differently, so dir is mapped
// to the concrete flag/verb per tool rather than passed through literally:
//
//	dir   tmux            cmux              iTerm2
//	col   split-window -h --direction right split vertically
//	row   split-window -v --direction down  split horizontally
func openPanel(mux run.Multiplexer, pn panel, dir splitDir) error {
	switch mux {
	case run.MuxTmux:
		// -d: create the pane without switching focus to it. sh -c is replaced
		// by the command (exec), so tmux's default close-on-exit is fine.
		flag := "-v"
		if dir == splitCol {
			flag = "-h"
		}
		_, err := uqexec.Run("tmux", "split-window", "-d", flag, "sh", "-c", pn.shellLine(true))
		return err
	case run.MuxCmux:
		return openCmuxPanel(pn, dir)
	case run.MuxITerm2:
		return openITerm2Panel(pn, dir)
	case run.MuxGhostty:
		return openGhosttyPanel(pn)
	case run.MuxAppleTerminal:
		return openAppleTerminalPanel(pn)
	default:
		return fmt.Errorf("분할 미지원: %s", mux)
	}
}

// openCmuxPanel creates a new cmux terminal pane and types the command into the
// pane's interactive shell via `cmux send`.
//
// We deliberately do NOT use `respawn-pane --command`: that REPLACES the pane's
// shell with the command, so the pane closes the instant the command exits
// (including on a fast error). Typing into the live shell instead keeps the
// pane open — the dev server runs in it, and if it errors you still see why.
//
// The bundled CLI path is exported by cmux as CMUX_BUNDLED_CLI_PATH.
func openCmuxPanel(pn panel, dir splitDir) error {
	cli := os.Getenv("CMUX_BUNDLED_CLI_PATH")
	if cli == "" {
		return fmt.Errorf("CMUX_BUNDLED_CLI_PATH 가 비어 있습니다")
	}
	direction := "down"
	if dir == splitCol {
		direction = "right"
	}
	out, err := uqexec.Run(cli, "new-pane", "--type", "terminal", "--direction", direction)
	if err != nil {
		return err
	}
	surface := parseCmuxSurface(string(out))
	if surface == "" {
		return fmt.Errorf("new-pane 출력에서 surface 를 찾지 못했습니다: %q", strings.TrimSpace(string(out)))
	}
	// Give the freshly-spawned shell a moment to start reading its pty before
	// we type into it, so the line isn't swallowed by shell init.
	time.Sleep(500 * time.Millisecond)
	// \n at the end presses Enter. No exec: the interactive shell stays alive.
	_, err = uqexec.Run(cli, "send", "--surface", surface, pn.shellLine(false)+"\n")
	return err
}

// parseCmuxSurface extracts the "surface:N" token from cmux new-pane output,
// e.g. "OK surface:14 pane:16 workspace:4" → "surface:14". respawn-pane wants
// just that ref, not the whole status line.
func parseCmuxSurface(out string) string {
	for _, f := range strings.Fields(out) {
		if strings.HasPrefix(f, "surface:") {
			return f
		}
	}
	return ""
}

// openITerm2Panel splits the current iTerm2 session and types the command into
// the new session via `write text`.
//
// Like the cmux path, we split into a normal shell and write the command in,
// rather than passing it as the pane's launch command. That avoids a nested
// quoting nightmare (the shell line already contains single quotes and \033
// escapes) and keeps the pane open if the command errors. useExec=false so the
// interactive shell survives.
func openITerm2Panel(pn panel, dir splitDir) error {
	line := pn.shellLine(false)
	// iTerm inverts the words: "vertically" = side-by-side (col),
	// "horizontally" = stacked (row).
	verb := "split horizontally"
	if dir == splitCol {
		verb = "split vertically"
	}
	args := []string{
		"-e", `tell application "iTerm2"`,
		"-e", `set s to current session of current window`,
		"-e", `tell s to set ns to (` + verb + ` with default profile)`,
		"-e", `tell ns to write text ` + appleScriptQuote(line),
		"-e", `end tell`,
	}
	_, err := uqexec.Run("osascript", args...)
	return err
}

// openGhosttyPanel opens the command in a NEW Ghostty window.
//
// Ghostty has no scripting IPC and `+new-window` is unsupported on macOS, so we
// launch through `open -na Ghostty.app --args -e <shell>`. Unlike a System
// Events approach this needs no Accessibility permission (we synthesize no
// keystrokes). After the command exits we exec an interactive shell so the
// window stays open and any error remains visible.
func openGhosttyPanel(pn panel) error {
	script := pn.shellLine(false) + "; exec ${SHELL:-/bin/zsh} -i -l"
	out, err := uqexec.RunCombined("open", "-na", "Ghostty.app", "--args", "-e", "/bin/sh", "-c", script)
	if err != nil {
		return fmt.Errorf("Ghostty 새 창 열기 실패: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// openAppleTerminalPanel opens the command in a new Terminal.app window.
//
// Terminal.app has no split panes, so `do script` (which spawns a new window
// and runs text in its shell) is the closest equivalent. This needs only
// Automation consent for controlling Terminal — no Accessibility — because we
// never synthesize keystrokes. The split direction is not applicable to windows.
func openAppleTerminalPanel(pn panel) error {
	args := []string{
		"-e", `tell application "Terminal"`,
		"-e", "activate",
		"-e", "do script " + appleScriptQuote(pn.shellLine(false)),
		"-e", "end tell",
	}
	out, err := uqexec.RunCombined("osascript", args...)
	if err != nil {
		s := strings.TrimSpace(string(out))
		if strings.Contains(s, "-1743") || strings.Contains(s, "Not authorized") || strings.Contains(s, "허용되지") {
			return fmt.Errorf("Terminal 제어(자동화) 권한이 필요합니다 — 시스템 설정 ▸ 개인정보 보호 및 보안 ▸ 자동화 에서 Terminal 을 허용한 뒤 다시 시도하세요")
		}
		return fmt.Errorf("%w: %s", err, s)
	}
	return nil
}

// shellQuote wraps s in single quotes, escaping any embedded single quotes,
// making it safe as a single sh word.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// appleScriptQuote wraps s in double quotes for an AppleScript string literal.
// Backslashes must be escaped before quotes — the shell line carries \033
// escapes that AppleScript would otherwise mangle.
func appleScriptQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
