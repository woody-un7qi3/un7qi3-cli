package logs

import "strings"

// SplitArgs 는 위치인자를 (국가코드, 환경필터) 로 가른다. countryCodes 와
// 일치하는 첫 토큰을 country 로 취하고, 그 외는 모두 envFilters 로 남긴다.
func SplitArgs(filters []string, countryCodes []string) (country string, envFilters []string) {
	codes := make(map[string]bool, len(countryCodes))
	for _, c := range countryCodes {
		codes[c] = true
	}
	for _, f := range filters {
		if country == "" && codes[f] {
			country = f
			continue
		}
		envFilters = append(envFilters, f)
	}
	return country, envFilters
}

// MatchEnvs 는 모든 filter 를 (대소문자 무시) 부분문자열로 포함하는 환경만 반환.
func MatchEnvs(envs, filters []string) []string {
	var out []string
	for _, e := range envs {
		le := strings.ToLower(e)
		ok := true
		for _, f := range filters {
			if !strings.Contains(le, strings.ToLower(f)) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, e)
		}
	}
	return out
}
