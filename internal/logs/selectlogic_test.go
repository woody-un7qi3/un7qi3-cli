package logs

import (
	"reflect"
	"testing"
)

func TestSplitArgsPicksCountry(t *testing.T) {
	c, f := SplitArgs([]string{"beta", "kr"}, []string{"kr", "en", "jp"})
	if c != "kr" {
		t.Errorf("country 기대 kr, 실제 %q", c)
	}
	if !reflect.DeepEqual(f, []string{"beta"}) {
		t.Errorf("envFilters 기대 [beta], 실제 %v", f)
	}
}

func TestSplitArgsNoCountry(t *testing.T) {
	c, f := SplitArgs([]string{"beta"}, []string{"kr", "en", "jp"})
	if c != "" {
		t.Errorf("country 없어야 함, 실제 %q", c)
	}
	if !reflect.DeepEqual(f, []string{"beta"}) {
		t.Errorf("envFilters 기대 [beta], 실제 %v", f)
	}
}

func TestMatchEnvsAndSubstring(t *testing.T) {
	envs := []string{"api-beta-kr-j21", "api-prod-kr-j21"}
	got := MatchEnvs(envs, []string{"BETA"})
	if !reflect.DeepEqual(got, []string{"api-beta-kr-j21"}) {
		t.Errorf("기대 [api-beta-kr-j21], 실제 %v", got)
	}
	if len(MatchEnvs(envs, []string{"beta", "prod"})) != 0 {
		t.Error("beta+prod 둘 다 포함하는 환경은 없어야 함")
	}
	if len(MatchEnvs(envs, nil)) != 2 {
		t.Error("필터 없으면 전부")
	}
}
