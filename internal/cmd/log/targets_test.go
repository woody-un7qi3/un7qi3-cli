package log

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/un7qi3inc/un7qi3-cli/internal/repocfg"
)

func sampleLogsCfg() *repocfg.Config {
	return &repocfg.Config{
		Logs: map[string]repocfg.LogsConfig{
			"forceteller-api": {
				Countries: map[string]repocfg.CountryTarget{
					"en": {App: "en-ft-api", Region: "us-east-1"},
					"kr": {App: "kr-ft-api", Region: "ap-northeast-2"},
					"jp": {App: "jp-ft-api", Region: "ap-northeast-1"},
				},
			},
			"sangdam-api": {
				Path: "/custom/log/path",
				Countries: map[string]repocfg.CountryTarget{
					"kr": {App: "kr-sangdam-api", Region: "ap-northeast-2"},
				},
			},
		},
	}
}

func TestCollectLogsTargets_RepoOrderAndCountryOrder(t *testing.T) {
	got := collectLogsTargets(sampleLogsCfg(), "")
	// 레포는 알파벳순: forceteller-api, sangdam-api.
	if len(got) != 2 {
		t.Fatalf("count: got %d, want 2", len(got))
	}
	if got[0].Repo != "forceteller-api" || got[1].Repo != "sangdam-api" {
		t.Fatalf("repo 순서: %q, %q", got[0].Repo, got[1].Repo)
	}
	// 국가는 kr→en→jp 고정 순서(countryCodes).
	var codes []string
	for _, c := range got[0].Countries {
		codes = append(codes, c.Code)
	}
	if strings.Join(codes, ",") != "kr,en,jp" {
		t.Errorf("국가 순서: got %v, want [kr en jp]", codes)
	}
}

func TestCollectLogsTargets_Filter(t *testing.T) {
	got := collectLogsTargets(sampleLogsCfg(), "sangdam-api")
	if len(got) != 1 {
		t.Fatalf("filter: got %d, want 1", len(got))
	}
	if got[0].Path != "/custom/log/path" {
		t.Errorf("path: %q", got[0].Path)
	}
	if got := collectLogsTargets(sampleLogsCfg(), "nonexistent"); len(got) != 0 {
		t.Errorf("miss: got %d, want 0", len(got))
	}
}

// TestLogsTargetsJSONStableShape 는 에이전트가 의존하는 JSON 모양을 고정한다.
// 필드명을 의도적으로 바꾸면 이 골든도 함께 갱신하고 breaking change 를 문서화한다.
func TestLogsTargetsJSONStableShape(t *testing.T) {
	cfg := &repocfg.Config{
		Logs: map[string]repocfg.LogsConfig{
			"r": {
				Path: "/var/log/web.stdout.log",
				Countries: map[string]repocfg.CountryTarget{
					"kr": {App: "kr-r", Region: "ap-northeast-2"},
				},
			},
		},
	}
	targets := collectLogsTargets(cfg, "")
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(logsTargetsOutput{Targets: targets}); err != nil {
		t.Fatal(err)
	}
	want := `{
  "targets": [
    {
      "repo": "r",
      "path": "/var/log/web.stdout.log",
      "countries": [
        {
          "code": "kr",
          "app": "kr-r",
          "region": "ap-northeast-2"
        }
      ]
    }
  ]
}
`
	if got := buf.String(); got != want {
		t.Errorf("JSON shape drifted.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestPrintLogsTargetsHumanContainsKeyFields(t *testing.T) {
	targets := collectLogsTargets(sampleLogsCfg(), "forceteller-api")
	var buf bytes.Buffer
	printLogsTargetsHuman(&buf, targets)
	out := buf.String()
	for _, s := range []string{
		"forceteller-api:kr",
		"kr-ft-api",
		"ap-northeast-2",
		"forceteller-api:jp",
	} {
		if !strings.Contains(out, s) {
			t.Errorf("human output missing %q\n%s", s, out)
		}
	}
}
