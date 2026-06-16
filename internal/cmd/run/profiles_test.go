package run

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/un7qi3inc/un7qi3-cli/internal/repocfg"
)

func sampleCfg() *repocfg.Config {
	return &repocfg.Config{
		Runs: map[string]repocfg.RepoRuns{
			"forceteller-app": {
				Default: "app3",
				Profiles: map[string]repocfg.Profile{
					"app3": {
						Cmd:  []string{"yarn", "start:app3"},
						Node: "16",
						URL:  "http://localhost:3100",
						Env:  map[string]string{"YARN_IGNORE_NODE": "1"},
					},
				},
			},
			"forceteller-admin": {
				Default: "local",
				Profiles: map[string]repocfg.Profile{
					"local": {
						Node: "16",
						Procs: []repocfg.Proc{
							{Name: "back", Cwd: "back-end", Cmd: []string{"npm", "run", "local"}, URL: "http://localhost:3000"},
							{Name: "front", Cwd: "front-end/workspace", Cmd: []string{"npm", "run", "local"}, URL: "http://localhost:4200"},
						},
					},
				},
			},
			"sole-profile-no-default": {
				Profiles: map[string]repocfg.Profile{
					"only": {Cmd: []string{"echo", "x"}},
				},
			},
			"multi-no-default": {
				Profiles: map[string]repocfg.Profile{
					"a": {Cmd: []string{"echo", "a"}},
					"b": {Cmd: []string{"echo", "b"}},
				},
			},
		},
	}
}

func TestCollectProfiles_OrderingAndDefault(t *testing.T) {
	got := collectProfiles(sampleCfg(), "/home/u", "")
	// Repos sorted alphabetically.
	wantRepoOrder := []string{
		"forceteller-admin", "forceteller-app", "multi-no-default", "multi-no-default", "sole-profile-no-default",
	}
	if len(got) != len(wantRepoOrder) {
		t.Fatalf("count: got %d, want %d", len(got), len(wantRepoOrder))
	}
	for i, p := range got {
		if p.Repo != wantRepoOrder[i] {
			t.Errorf("idx %d: got repo %q, want %q", i, p.Repo, wantRepoOrder[i])
		}
	}
	// Defaults: forceteller-admin:local, forceteller-app:app3, sole-profile-no-default:only.
	defaults := map[string]bool{}
	for _, p := range got {
		if p.Default {
			defaults[p.Repo+":"+p.Name] = true
		}
	}
	for _, want := range []string{
		"forceteller-admin:local",
		"forceteller-app:app3",
		"sole-profile-no-default:only",
	} {
		if !defaults[want] {
			t.Errorf("expected %s to be default", want)
		}
	}
	// Multi-no-default: neither should be default (ambiguous).
	for _, p := range got {
		if p.Repo == "multi-no-default" && p.Default {
			t.Errorf("multi-no-default:%s should NOT be default (ambiguous)", p.Name)
		}
	}
}

func TestCollectProfiles_AbsoluteCwdAndProcs(t *testing.T) {
	got := collectProfiles(sampleCfg(), "/home/u/un7qi3", "forceteller-admin")
	if len(got) != 1 {
		t.Fatalf("filter should return 1 profile, got %d", len(got))
	}
	p := got[0]
	if p.Cwd != "/home/u/un7qi3/forceteller-admin" {
		t.Errorf("cwd: %q", p.Cwd)
	}
	if len(p.Procs) != 2 {
		t.Fatalf("procs: %d", len(p.Procs))
	}
	if p.Procs[0].Cwd != "/home/u/un7qi3/forceteller-admin/back-end" {
		t.Errorf("back cwd: %q", p.Procs[0].Cwd)
	}
	if p.Procs[1].Cwd != "/home/u/un7qi3/forceteller-admin/front-end/workspace" {
		t.Errorf("front cwd: %q", p.Procs[1].Cwd)
	}
}

func TestCollectProfiles_FilterMiss(t *testing.T) {
	got := collectProfiles(sampleCfg(), "/h", "nonexistent")
	if len(got) != 0 {
		t.Errorf("expected empty, got %d", len(got))
	}
}

// TestProfilesJSONStableShape locks the JSON shape so agents can rely on it.
// If you intentionally change a field name, update the golden here AND
// document the breaking change.
func TestProfilesJSONStableShape(t *testing.T) {
	cfg := &repocfg.Config{
		Runs: map[string]repocfg.RepoRuns{
			"r": {
				Default: "p",
				Profiles: map[string]repocfg.Profile{
					"p": {
						Cmd:  []string{"yarn", "start"},
						Node: "16",
						URL:  "http://localhost:1234",
						Env:  map[string]string{"K": "V"},
					},
				},
			},
		},
	}
	profiles := collectProfiles(cfg, "/h/un7qi3", "")
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(profilesOutput{Profiles: profiles}); err != nil {
		t.Fatal(err)
	}
	want := `{
  "profiles": [
    {
      "repo": "r",
      "name": "p",
      "default": true,
      "cwd": "/h/un7qi3/r",
      "node": "16",
      "env": {
        "K": "V"
      },
      "url": "http://localhost:1234",
      "cmd": [
        "yarn",
        "start"
      ]
    }
  ]
}
`
	if got := buf.String(); got != want {
		t.Errorf("JSON shape drifted.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestPrintProfilesHumanContainsKeyFields(t *testing.T) {
	profiles := collectProfiles(sampleCfg(), "/h", "forceteller-admin")
	var buf bytes.Buffer
	printProfilesHuman(&buf, profiles)
	out := buf.String()
	for _, s := range []string{
		"forceteller-admin:local",
		"2 procs",
		"back=http://localhost:3000",
		"front=http://localhost:4200",
	} {
		if !strings.Contains(out, s) {
			t.Errorf("human output missing %q\n%s", s, out)
		}
	}
}
