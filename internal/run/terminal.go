// Package run powers `uq run <repo>[:profile]`.
//
// terminal.go detects which terminal multiplexer (if any) wraps the current
// session, so `uq run --split` can open one panel per process using the right
// native mechanism.
package run

import "os"

// Multiplexer names a way to split the current terminal into panels.
type Multiplexer string

const (
	// MuxTmux: inside a tmux session — split via `tmux split-window`.
	MuxTmux Multiplexer = "tmux"
	// MuxCmux: inside a cmux pane — split via the bundled cmux CLI.
	MuxCmux Multiplexer = "cmux"
	// MuxITerm2: iTerm2 terminal — split via AppleScript (osascript).
	MuxITerm2 Multiplexer = "iterm2"
	// MuxGhostty: standalone Ghostty — no IPC, so split via System Events
	// (trigger the native split keybind + type the command). Needs Accessibility.
	MuxGhostty Multiplexer = "ghostty"
	// MuxAppleTerminal: macOS Terminal.app — has no split panes, so each proc
	// opens in its own window via AppleScript `do script`.
	MuxAppleTerminal Multiplexer = "apple-terminal"
	// MuxNone: no supported splitter (vscode, wezterm, unknown). Callers fall
	// back to background log files.
	MuxNone Multiplexer = "none"
)

// DetectMultiplexer walks a priority ladder and returns the first match.
//
// The order matters: cmux embeds ghostty, so a cmux pane reports
// TERM_PROGRAM=ghostty and would be misread as a plain terminal. cmux's own
// env markers are therefore checked before any TERM_PROGRAM heuristic.
//
//	1. tmux     — $TMUX set
//	2. cmux     — $CMUX_PANEL_ID and $CMUX_SOCKET_PATH set
//	3. iTerm2   — $TERM_PROGRAM == "iTerm.app"
//	4. ghostty  — $TERM_PROGRAM == "ghostty" (standalone; cmux already caught above)
//	5. Terminal — $TERM_PROGRAM == "Apple_Terminal"
//	6. none     — everything else
func DetectMultiplexer() Multiplexer {
	if os.Getenv("TMUX") != "" {
		return MuxTmux
	}
	if os.Getenv("CMUX_PANEL_ID") != "" && os.Getenv("CMUX_SOCKET_PATH") != "" {
		return MuxCmux
	}
	switch os.Getenv("TERM_PROGRAM") {
	case "iTerm.app":
		return MuxITerm2
	case "ghostty":
		return MuxGhostty
	case "Apple_Terminal":
		return MuxAppleTerminal
	}
	return MuxNone
}
