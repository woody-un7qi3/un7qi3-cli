package repocfg

import "testing"

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
