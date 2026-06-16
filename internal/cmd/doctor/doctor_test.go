package doctor

import (
	"reflect"
	"testing"
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

func contains(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}
