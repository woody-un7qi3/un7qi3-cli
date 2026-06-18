package run

import "testing"

// withEnv clears the multiplexer-relevant env vars, applies overrides, and
// restores everything afterward, so each case sees a known starting point.
func withEnv(t *testing.T, overrides map[string]string) {
	t.Helper()
	for _, k := range []string{"TMUX", "CMUX_PANEL_ID", "CMUX_SOCKET_PATH", "TERM_PROGRAM"} {
		t.Setenv(k, "")
	}
	for k, v := range overrides {
		t.Setenv(k, v)
	}
}

func TestDetectMultiplexer(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want Multiplexer
	}{
		{"tmux", map[string]string{"TMUX": "/tmp/tmux-501/default,123,0"}, MuxTmux},
		{"cmux", map[string]string{"CMUX_PANEL_ID": "p1", "CMUX_SOCKET_PATH": "/tmp/cmux.sock"}, MuxCmux},
		// cmux embeds ghostty: its markers must win over TERM_PROGRAM.
		{"cmux-over-ghostty", map[string]string{"CMUX_PANEL_ID": "p1", "CMUX_SOCKET_PATH": "/tmp/cmux.sock", "TERM_PROGRAM": "ghostty"}, MuxCmux},
		{"iterm2", map[string]string{"TERM_PROGRAM": "iTerm.app"}, MuxITerm2},
		{"ghostty-standalone", map[string]string{"TERM_PROGRAM": "ghostty"}, MuxGhostty},
		{"apple-terminal", map[string]string{"TERM_PROGRAM": "Apple_Terminal"}, MuxAppleTerminal},
		{"vscode", map[string]string{"TERM_PROGRAM": "vscode"}, MuxNone},
		{"bare", nil, MuxNone},
		// tmux outranks cmux when both somehow present.
		{"tmux-over-cmux", map[string]string{"TMUX": "x", "CMUX_PANEL_ID": "p1", "CMUX_SOCKET_PATH": "/s"}, MuxTmux},
		// cmux needs BOTH markers; one alone is not enough.
		{"cmux-partial", map[string]string{"CMUX_PANEL_ID": "p1"}, MuxNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withEnv(t, tc.env)
			if got := DetectMultiplexer(); got != tc.want {
				t.Fatalf("DetectMultiplexer() = %q, want %q", got, tc.want)
			}
		})
	}
}
