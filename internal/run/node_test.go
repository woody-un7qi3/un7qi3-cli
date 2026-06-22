package run

import "testing"

func TestVersionMatches(t *testing.T) {
	cases := []struct {
		got, want string
		ok        bool
	}{
		{"16.20.2", "", true},
		{"16.20.2", "16", true},
		{"16.20.2", "16.20", true},
		{"16.20.2", "16.20.2", true},
		{"16.20.2", "16.20.3", false},
		{"16.20.2", "18", false},
		{"v16.20.2", "16", true},
		{"16", "16.20", false},
		// 최소 버전(범위) 제약 ">=N"
		{"18.0.0", ">=18", true},
		{"20.19.2", ">=18", true},
		{"22.20.0", ">=18", true},
		{"16.20.2", ">=18", false},
		{"18", ">=18", true},
		{"17.9.1", ">=18", false},
		{"v20.1.0", ">=18", true},
		{"20.19.2", ">= 18", true}, // 공백 허용
		{"18.20.0", ">=18.12", true},
		{"18.11.0", ">=18.12", false},
	}
	for _, c := range cases {
		if got := versionMatches(c.got, c.want); got != c.ok {
			t.Errorf("versionMatches(%q, %q) = %v, want %v", c.got, c.want, got, c.ok)
		}
	}
}

func TestIsRange(t *testing.T) {
	cases := map[string]bool{">=18": true, ">= 18": true, "18": false, "": false, "20.5": false}
	for in, want := range cases {
		if got := isRange(in); got != want {
			t.Errorf("isRange(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestVersionLess(t *testing.T) {
	cases := []struct {
		a, b string
		less bool
	}{
		{"16.20.2", "16.20.3", true},
		{"16.20.3", "16.20.2", false},
		{"16.20.2", "16.20.2", false},
		{"16.9.0", "16.10.0", true}, // numeric, not lexical
		{"18.0.0", "16.20.2", false},
		{"v16.20.2", "16.20.3", true},
	}
	for _, c := range cases {
		if got := versionLess(c.a, c.b); got != c.less {
			t.Errorf("versionLess(%q, %q) = %v, want %v", c.a, c.b, got, c.less)
		}
	}
}
