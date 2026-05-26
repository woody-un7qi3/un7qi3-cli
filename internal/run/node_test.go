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
	}
	for _, c := range cases {
		if got := versionMatches(c.got, c.want); got != c.ok {
			t.Errorf("versionMatches(%q, %q) = %v, want %v", c.got, c.want, got, c.ok)
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
