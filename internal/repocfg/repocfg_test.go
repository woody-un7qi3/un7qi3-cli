package repocfg

import (
	"strings"
	"testing"
)

// The embedded repos.yml is the authoritative test input — loading it
// exercises the YAML schema (including the new runs: block) end-to-end.
func TestLoadEmbedded(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(c.Repos) == 0 {
		t.Fatal("Repos empty")
	}
	if len(c.Defaults) == 0 {
		t.Fatal("Defaults empty")
	}
}

func TestProfileFor_Default(t *testing.T) {
	c := &Config{
		Runs: map[string]RepoRuns{
			"r": {
				Default: "a",
				Profiles: map[string]Profile{
					"a": {Cmd: []string{"echo", "a"}},
					"b": {Cmd: []string{"echo", "b"}},
				},
			},
		},
	}
	p, name, err := c.ProfileFor("r", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if name != "a" || p.Cmd[1] != "a" {
		t.Fatalf("got profile %s (%v), want a", name, p.Cmd)
	}
}

func TestProfileFor_SingleProfileNoDefault(t *testing.T) {
	c := &Config{
		Runs: map[string]RepoRuns{
			"r": {Profiles: map[string]Profile{"only": {Cmd: []string{"x"}}}},
		},
	}
	_, name, err := c.ProfileFor("r", "")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if name != "only" {
		t.Fatalf("got %s, want only", name)
	}
}

func TestProfileFor_MultipleProfilesNoDefaultErrors(t *testing.T) {
	c := &Config{
		Runs: map[string]RepoRuns{
			"r": {Profiles: map[string]Profile{
				"a": {Cmd: []string{"x"}},
				"b": {Cmd: []string{"y"}},
			}},
		},
	}
	if _, _, err := c.ProfileFor("r", ""); err == nil {
		t.Fatal("expected error when no default and multiple profiles")
	}
}

func TestProfileFor_UnknownProfile(t *testing.T) {
	c := &Config{
		Runs: map[string]RepoRuns{
			"r": {Profiles: map[string]Profile{"a": {Cmd: []string{"x"}}}},
		},
	}
	if _, _, err := c.ProfileFor("r", "zzz"); err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestProfileFor_UnknownRepo(t *testing.T) {
	c := &Config{Runs: map[string]RepoRuns{}}
	if _, _, err := c.ProfileFor("nope", ""); err == nil {
		t.Fatal("expected error for unknown repo")
	}
}

func TestProfileFor_EmptyCmdErrors(t *testing.T) {
	c := &Config{
		Runs: map[string]RepoRuns{
			"r": {Profiles: map[string]Profile{"a": {Cmd: nil}}},
		},
	}
	if _, _, err := c.ProfileFor("r", "a"); err == nil {
		t.Fatal("expected error for empty cmd")
	}
}

func TestProfileFor_Procs(t *testing.T) {
	c := &Config{
		Runs: map[string]RepoRuns{
			"r": {Profiles: map[string]Profile{"a": {Procs: []Proc{
				{Name: "back", Cmd: []string{"yarn", "local"}},
				{Name: "front", Cwd: "front", Cmd: []string{"yarn", "local"}},
			}}}},
		},
	}
	p, _, err := c.ProfileFor("r", "a")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(p.Procs) != 2 || p.Procs[1].Cwd != "front" {
		t.Fatalf("procs not parsed correctly: %+v", p.Procs)
	}
}

func TestProfileFor_CmdAndProcsMutuallyExclusive(t *testing.T) {
	c := &Config{
		Runs: map[string]RepoRuns{
			"r": {Profiles: map[string]Profile{"a": {
				Cmd:   []string{"x"},
				Procs: []Proc{{Name: "p", Cmd: []string{"y"}}},
			}}},
		},
	}
	if _, _, err := c.ProfileFor("r", "a"); err == nil {
		t.Fatal("expected error when both cmd and procs set")
	}
}

func TestProfileFor_ProcMissingName(t *testing.T) {
	c := &Config{
		Runs: map[string]RepoRuns{
			"r": {Profiles: map[string]Profile{"a": {
				Procs: []Proc{{Cmd: []string{"y"}}},
			}}},
		},
	}
	if _, _, err := c.ProfileFor("r", "a"); err == nil {
		t.Fatal("expected error for proc without name")
	}
}

func TestProfileFor_ProcEmptyCmd(t *testing.T) {
	c := &Config{
		Runs: map[string]RepoRuns{
			"r": {Profiles: map[string]Profile{"a": {
				Procs: []Proc{{Name: "back"}},
			}}},
		},
	}
	if _, _, err := c.ProfileFor("r", "a"); err == nil {
		t.Fatal("expected error for proc with empty cmd")
	}
}

func sampleCountries() *Countries {
	return &Countries{
		Default: "kr",
		Options: []Country{
			{Code: "kr", Script: "local", Requires: []string{"back-end/.env"}},
			{Code: "en", Script: "local_en", Requires: []string{"back-end/.env.en"}},
			{Code: "jp", Script: "local_jp", Requires: []string{"back-end/.env.jp"}},
		},
	}
}

func TestCountries_Find(t *testing.T) {
	cs := sampleCountries()
	if c, ok := cs.Find("en"); !ok || c.Script != "local_en" {
		t.Fatalf("Find(en) = %+v, %v", c, ok)
	}
	if _, ok := cs.Find("xx"); ok {
		t.Fatal("Find(xx) should not be found")
	}
}

func TestCountries_Codes(t *testing.T) {
	if got := sampleCountries().Codes(); got != "kr, en, jp" {
		t.Fatalf("Codes() = %q, want \"kr, en, jp\"", got)
	}
}

func TestSubstituteScript_Procs(t *testing.T) {
	p := Profile{Procs: []Proc{
		{Name: "back", Cmd: []string{"npm", "run", "{script}"}},
		{Name: "front", Cmd: []string{"npm", "run", "{script}"}},
	}}
	out := p.SubstituteScript("local_en")
	for _, pr := range out.Procs {
		if got := strings.Join(pr.Cmd, " "); got != "npm run local_en" {
			t.Fatalf("proc %s cmd = %q, want \"npm run local_en\"", pr.Name, got)
		}
	}
	// original untouched
	if p.Procs[0].Cmd[2] != "{script}" {
		t.Fatalf("original mutated: %v", p.Procs[0].Cmd)
	}
}

func TestSubstituteScript_Cmd(t *testing.T) {
	p := Profile{Cmd: []string{"yarn", "{script}"}}
	if got := strings.Join(p.SubstituteScript("start").Cmd, " "); got != "yarn start" {
		t.Fatalf("cmd = %q, want \"yarn start\"", got)
	}
}
