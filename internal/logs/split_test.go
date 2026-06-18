package logs

import (
	"strings"
	"testing"

	"github.com/un7qi3inc/un7qi3-cli/internal/run"
)

func TestSplitSupportedOnlyCmuxIterm2(t *testing.T) {
	if !SplitSupported(run.MuxCmux) || !SplitSupported(run.MuxITerm2) {
		t.Error("cmux/iterm2 는 지원이어야 함")
	}
	for _, m := range []run.Multiplexer{run.MuxGhostty, run.MuxAppleTerminal, run.MuxNone, run.MuxTmux} {
		if SplitSupported(m) {
			t.Errorf("%s 는 미지원이어야 함", m)
		}
	}
}

func TestBuildPanels(t *testing.T) {
	argv := [][]string{{"ssh", "api-beta-kr-j21", "-c", "sudo tail -F /var/log/web.stdout.log"}}
	panels := BuildPanels(argv, []string{"api-beta-kr-j21#1"})
	if len(panels) != 1 {
		t.Fatalf("패널 1개 기대, 실제 %d", len(panels))
	}
	if panels[0].Label != "api-beta-kr-j21#1" {
		t.Errorf("라벨 틀림: %q", panels[0].Label)
	}
	if !strings.HasPrefix(panels[0].Command, "eb ssh ") {
		t.Errorf("명령은 'eb ssh '로 시작해야 함: %q", panels[0].Command)
	}
}
