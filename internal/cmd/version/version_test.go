package version

import "testing"

func TestFormatBuildDate(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"UTC Z → KST(+9)", "2026-06-22T08:06:29Z", "2026-06-22 17:06:29 KST"},
		{"오프셋 명시도 KST 로 정규화", "2026-06-22T08:06:29+00:00", "2026-06-22 17:06:29 KST"},
		{"자정 넘기는 변환", "2026-06-22T16:30:00Z", "2026-06-23 01:30:00 KST"},
		{"파싱 불가(dev)는 원본 유지", "unknown", "unknown"},
		{"빈 값은 그대로", "", ""},
	}
	for _, c := range cases {
		if got := formatBuildDate(c.in); got != c.want {
			t.Errorf("%s: formatBuildDate(%q) = %q, want %q", c.name, c.in, got, c.want)
		}
	}
}
