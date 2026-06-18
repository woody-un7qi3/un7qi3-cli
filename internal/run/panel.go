package run

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
)

// Panel 은 분할 패널 하나 — 제목과 실행할 셸 1줄.
type Panel struct {
	Label   string
	Command string
}

// SplitDir 은 분할 방향.
type SplitDir int

const (
	SplitCol SplitDir = iota // 좌우
	SplitRow                 // 상하
)

var errUnsupportedMux = errors.New("지원하지 않는 멀티플렉서")

// OpenPanel 은 감지된 멀티플렉서로 패널을 연다.
func OpenPanel(mux Multiplexer, p Panel, dir SplitDir) error {
	switch mux {
	case MuxTmux:
		flag := "-v"
		if dir == SplitCol {
			flag = "-h"
		}
		_, err := uqexec.Run("tmux", "split-window", "-d", flag, "sh", "-c", p.Command)
		return err
	case MuxCmux:
		return openCmuxPanel(p, dir)
	case MuxITerm2:
		return openITerm2Panel(p, dir)
	case MuxGhostty:
		return openGhosttyPanel(p)
	case MuxAppleTerminal:
		return openAppleTerminalPanel(p)
	default:
		return errUnsupportedMux
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
func openCmuxPanel(p Panel, dir SplitDir) error {
	cli := os.Getenv("CMUX_BUNDLED_CLI_PATH")
	if cli == "" {
		return fmt.Errorf("CMUX_BUNDLED_CLI_PATH 가 비어 있습니다")
	}
	direction := "down"
	if dir == SplitCol {
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
	_, err = uqexec.Run(cli, "send", "--surface", surface, p.Command+"\n")
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
func openITerm2Panel(p Panel, dir SplitDir) error {
	line := p.Command
	// iTerm inverts the words: "vertically" = side-by-side (col),
	// "horizontally" = stacked (row).
	verb := "split horizontally"
	if dir == SplitCol {
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
func openGhosttyPanel(p Panel) error {
	script := p.Command + "; exec ${SHELL:-/bin/zsh} -i -l"
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
func openAppleTerminalPanel(p Panel) error {
	args := []string{
		"-e", `tell application "Terminal"`,
		"-e", "activate",
		"-e", "do script " + appleScriptQuote(p.Command),
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

// ShellQuote wraps s in single quotes, escaping any embedded single quotes,
// making it safe as a single sh word.
func ShellQuote(s string) string {
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
