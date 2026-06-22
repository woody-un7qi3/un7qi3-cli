package repocfg

import (
	"reflect"
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

// joinKeys 는 맵 키를 알파벳 오름차순으로 ", " 로 이어 붙인다 — 에러 메시지에서
// "사용 가능: ..." 목록 순서가 맵 순회 순서와 무관하게 안정적이어야 한다.
func TestJoinKeys(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]Profile
		want string
	}{
		{"empty", map[string]Profile{}, "(없음)"},
		{"nil", nil, "(없음)"},
		{"single", map[string]Profile{"a": {}}, "a"},
		{"sorted", map[string]Profile{"c": {}, "a": {}, "b": {}}, "a, b, c"},
		{"mixed-case", map[string]Profile{"B": {}, "a": {}, "A": {}}, "A, B, a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinKeys(tt.in); got != tt.want {
				t.Errorf("joinKeys() = %q, want %q", got, tt.want)
			}
		})
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

func TestLogsForReturnsConfiguredRepo(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	lc, ok := cfg.LogsFor("forceteller-api")
	if !ok {
		t.Fatal("forceteller-api 가 logs 에 등록돼 있어야 함")
	}
	if lc.PathOrDefault() != DefaultLogPath {
		t.Errorf("기본 경로 기대 %q, 실제 %q", DefaultLogPath, lc.PathOrDefault())
	}
	kr, ok := lc.Countries["kr"]
	if !ok {
		t.Fatal("kr 국가가 있어야 함")
	}
	if kr.App != "kr-forceteller-api" || kr.Region != "ap-northeast-2" {
		t.Errorf("kr 매핑 틀림: %+v", kr)
	}
}

func TestLogsForUnknownRepo(t *testing.T) {
	cfg, _ := Load()
	if _, ok := cfg.LogsFor("forceteller-app"); ok {
		t.Error("logs 미등록 레포는 false 여야 함")
	}
}

func TestLogsForAddedRepos(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	// scala-api(=kr-app/en-app/jp-app) — kr/en/jp 모두 등록
	scala, ok := cfg.LogsFor("scala-api")
	if !ok {
		t.Fatal("scala-api 가 logs 에 등록돼 있어야 함")
	}
	for _, tc := range []struct{ country, wantApp, wantRegion string }{
		{"kr", "kr-app", "ap-northeast-2"},
		{"en", "en-app", "ap-southeast-1"},
		{"jp", "jp-app", "ap-northeast-1"},
	} {
		c, ok := scala.Countries[tc.country]
		if !ok {
			t.Errorf("scala-api: %s 국가 누락", tc.country)
			continue
		}
		if c.App != tc.wantApp || c.Region != tc.wantRegion {
			t.Errorf("scala-api/%s = %+v, want app=%s region=%s", tc.country, c, tc.wantApp, tc.wantRegion)
		}
	}

	// sangdam-api — kr 만 (en/jp 미존재)
	sg, ok := cfg.LogsFor("sangdam-api")
	if !ok {
		t.Fatal("sangdam-api 가 logs 에 등록돼 있어야 함")
	}
	if c, ok := sg.Countries["kr"]; !ok || c.App != "kr-sangdam-api" || c.Region != "ap-northeast-2" {
		t.Errorf("sangdam-api/kr = %+v(ok=%v), want kr-sangdam-api/ap-northeast-2", c, ok)
	}
	if _, ok := sg.Countries["en"]; ok {
		t.Error("sangdam-api 는 en 이 없어야 함")
	}
	if _, ok := sg.Countries["jp"]; ok {
		t.Error("sangdam-api 는 jp 가 없어야 함")
	}
}

func TestLogsReposSorted(t *testing.T) {
	cfg := &Config{Logs: map[string]LogsConfig{
		"charlie": {},
		"alpha":   {},
		"bravo":   {},
	}}
	got := cfg.LogsRepos()
	want := []string{"alpha", "bravo", "charlie"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("LogsRepos() = %v, want %v", got, want)
	}
}

func TestLogsReposEmpty(t *testing.T) {
	cfg := &Config{} // Logs nil
	if got := cfg.LogsRepos(); len(got) != 0 {
		t.Errorf("nil Logs 는 빈 슬라이스여야 함: %v", got)
	}
}
