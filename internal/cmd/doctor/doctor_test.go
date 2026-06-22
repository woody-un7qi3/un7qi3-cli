package doctor

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
)

func TestValidateRoles(t *testing.T) {
	for _, ok := range [][]string{
		nil,
		{},
		{RoleBackend},
		{RoleFrontend, RoleInfra},
		{RoleBackend, RoleFrontend, RoleInfra},
	} {
		if err := validateRoles(ok); err != nil {
			t.Errorf("validateRoles(%v) should pass, got %v", ok, err)
		}
	}
	for _, bad := range [][]string{
		{"devops"},
		{RoleBackend, "qa"},
		{"BACKEND"}, // case-sensitive
	} {
		if err := validateRoles(bad); err == nil {
			t.Errorf("validateRoles(%v) should fail", bad)
		}
	}
}

func TestFilterByRoles_NoFilterKeepsAll(t *testing.T) {
	checks := []Check{
		{Name: "git"},
		{Name: "java", Roles: []string{RoleBackend}},
		{Name: "ng", Roles: []string{RoleFrontend}},
	}
	got := filterByRoles(checks, nil)
	if len(got) != 3 {
		t.Errorf("len = %d, want 3", len(got))
	}
}

func TestFilterByRoles_CommonAlwaysKept(t *testing.T) {
	checks := []Check{
		{Name: "git"}, // common
		{Name: "java", Roles: []string{RoleBackend}},
		{Name: "ng", Roles: []string{RoleFrontend}},
	}
	got := filterByRoles(checks, []string{RoleFrontend})
	names := namesOf(got)
	if !contains(names, "git") {
		t.Errorf("common 'git' dropped: %v", names)
	}
	if !contains(names, "ng") {
		t.Errorf("frontend 'ng' missing: %v", names)
	}
	if contains(names, "java") {
		t.Errorf("backend 'java' leaked: %v", names)
	}
}

func TestFilterByRoles_MultiRoleMatchesAny(t *testing.T) {
	checks := []Check{
		{Name: "docker", Roles: []string{RoleBackend, RoleInfra}},
	}
	if got := filterByRoles(checks, []string{RoleFrontend}); len(got) != 0 {
		t.Errorf("docker should be filtered out: %v", got)
	}
	if got := filterByRoles(checks, []string{RoleInfra}); len(got) != 1 {
		t.Errorf("docker should appear under infra filter: %v", got)
	}
}

func TestGroupResultsByRole_BucketsCorrectly(t *testing.T) {
	results := []Result{
		{Name: "git"}, // common
		{Name: "java", Roles: []string{RoleBackend}},
		{Name: "ng", Roles: []string{RoleFrontend}},
		{Name: "aws", Roles: []string{RoleInfra}},
		{Name: "docker", Roles: []string{RoleBackend, RoleInfra}}, // first match wins
	}
	groups := groupResultsByRole(results)
	if got := namesOfResults(groups[""]); !reflect.DeepEqual(got, []string{"git"}) {
		t.Errorf("common: %v", got)
	}
	if got := namesOfResults(groups[RoleBackend]); !reflect.DeepEqual(got, []string{"java", "docker"}) {
		t.Errorf("backend: %v", got)
	}
	if got := namesOfResults(groups[RoleFrontend]); !reflect.DeepEqual(got, []string{"ng"}) {
		t.Errorf("frontend: %v", got)
	}
	if got := namesOfResults(groups[RoleInfra]); !reflect.DeepEqual(got, []string{"aws"}) {
		t.Errorf("infra (docker should be backend not infra): %v", got)
	}
}

// Sanity: buildChecks output is consistent — no unknown roles, no name dups,
// every known role has at least one tool.
func TestBuildChecks_Inventory(t *testing.T) {
	known := map[string]bool{RoleBackend: true, RoleFrontend: true, RoleInfra: true}
	seenNames := map[string]bool{}
	roleCounts := map[string]int{}
	for _, c := range buildChecks() {
		if seenNames[c.Name] {
			t.Errorf("duplicate check name: %s", c.Name)
		}
		seenNames[c.Name] = true
		if len(c.Roles) == 0 {
			roleCounts[""]++
			continue
		}
		for _, r := range c.Roles {
			if !known[r] {
				t.Errorf("%s: unknown role %q", c.Name, r)
			}
			roleCounts[r]++
		}
	}
	for _, r := range []string{"", RoleBackend, RoleFrontend, RoleInfra} {
		if roleCounts[r] == 0 {
			t.Errorf("role %q has no tools", r)
		}
	}
}

// eb 는 logs 를 쓰지 않는 역할(프런트 등)에게 강제되면 안 되므로 doctor 에서
// Optional 로 취급한다(미설치여도 하드 실패 아님). 설치 안내는 다른 인프라 도구와
// 동일하게 brew 로 통일한다.
func TestBuildChecks_EBOptionalWithBrewFix(t *testing.T) {
	eb := findCheck(t, "eb")
	if !eb.Optional {
		t.Errorf("eb 는 Optional 이어야 한다(logs 미사용 역할에 강제 금지)")
	}
	if eb.Fix != "brew install aws-elasticbeanstalk" {
		t.Errorf("eb Fix = %q, want %q", eb.Fix, "brew install aws-elasticbeanstalk")
	}
}

// deadlineRunner blocks until the context is cancelled and returns the context
// error — standing in for a wedged `git` whose process exec.CommandContext
// would have to kill on timeout. It lets us drive the scan's per-repo timeout
// path deterministically without a real hung process.
type deadlineRunner struct{}

func (deadlineRunner) Run(ctx context.Context, _ string, _ ...string) (string, string, error) {
	<-ctx.Done()
	return "", "", ctx.Err()
}

// A repo whose `git config` never returns must not hang the whole scan: the
// per-repo timeout fires, that repo drops out (counted as usingGlobal, i.e.
// "no readable override"), and scanUn7qi3LocalEmails still returns. Without the
// timeout this test would block forever. We use a 1ms parent deadline so the
// per-repo WithTimeout fires immediately rather than waiting the full 10s.
func TestScanUn7qi3LocalEmails_TimeoutDoesNotHang(t *testing.T) {
	ws := t.TempDir()
	repo := filepath.Join(ws, "slowrepo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}

	orig := runner
	runner = deadlineRunner{}
	t.Cleanup(func() { runner = orig })

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	var overrides map[string]string
	var usingGlobal []string
	go func() {
		overrides, usingGlobal = scanUn7qi3LocalEmails(ctx, ws, "global@example.com")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("scanUn7qi3LocalEmails hung past the per-repo timeout")
	}

	if len(overrides) != 0 {
		t.Errorf("overrides = %v, want empty (slow repo yields no override)", overrides)
	}
	if !contains(usingGlobal, "slowrepo") {
		t.Errorf("usingGlobal = %v, want it to include the timed-out repo", usingGlobal)
	}
}

var _ uqexec.Runner = deadlineRunner{}

func findCheck(t *testing.T, name string) Check {
	t.Helper()
	for _, c := range buildChecks() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("check %q not found", name)
	return Check{}
}

func namesOf(cs []Check) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}

func namesOfResults(rs []Result) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Name
	}
	return out
}

// TestVisibleLen pins the terminal-width calculation used for column
// alignment. ASCII=1 and Hangul=2 are the *existing* behavior that must be
// preserved (the table relies on Hangul headings like "백엔드" lining up);
// emoji/CJK/ANSI-mixed are the cases the old hand-rolled width miscounted and
// runewidth now handles.
func TestVisibleLen(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want int
	}{
		{"빈 문자열", "", 0},
		{"ASCII", "git", 3},
		{"ASCII 기호 포함", "node-v18.0.0", 12},
		// 행위 보존 핵심: 한글은 기존과 동일하게 음절당 2칸.
		{"한글 음절", "백엔드", 6},
		{"한글+ASCII 혼합", "공통 git", 8}, // 공(2)+통(2)+공백(1)+git(3)
		{"한글 자모", "ㄱㄴㄷ", 6},
		// 새로 올바르게 계산되는 케이스.
		{"기타 CJK 한자", "中文", 4},
		{"이모지", "👀", 2},
		// 글리프(✓)는 width 1 — 기존 default 분기와 동일하게 보존.
		{"체크 글리프", "✓ git", 5},
		// ANSI 이스케이프는 폭 계산 전에 제거된다.
		{"ANSI 래핑", "\033[34m/usr/bin/git\033[0m", 12},
		{"ANSI + 한글", "\033[2m백엔드\033[0m", 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := visibleLen(tc.in); got != tc.want {
				t.Errorf("visibleLen(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func contains(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}
