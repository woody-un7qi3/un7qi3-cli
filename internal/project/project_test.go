package project

import "testing"

func TestSelfRepoDefault(t *testing.T) {
	t.Setenv("UQ_REPO", "")
	if got := SelfRepo(); got != "woody-un7qi3/un7qi3-cli" {
		t.Fatalf("SelfRepo() = %q, want default", got)
	}
}

func TestSelfRepoEnvOverride(t *testing.T) {
	t.Setenv("UQ_REPO", "someone/fork")
	if got := SelfRepo(); got != "someone/fork" {
		t.Fatalf("SelfRepo() = %q, want override", got)
	}
}

func TestOrg(t *testing.T) {
	if got := Org(); got != "un7qi3inc" {
		t.Fatalf("Org() = %q, want un7qi3inc", got)
	}
}
