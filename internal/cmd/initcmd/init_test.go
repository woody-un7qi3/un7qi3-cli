package initcmd

import (
	"os"
	"path/filepath"
	"testing"
)

// writeRepo creates dir/.git/config with the given remote url fragment.
func writeRepo(t *testing.T, dir, remote string) {
	t.Helper()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := "[remote \"origin\"]\n\turl = " + remote + "\n"
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanRoot_CountsOrgReposByParent(t *testing.T) {
	root := t.TempDir()
	ws := filepath.Join(root, "ws")

	writeRepo(t, filepath.Join(ws, "repo-a"), "https://github.com/un7qi3inc/repo-a.git")
	writeRepo(t, filepath.Join(ws, "repo-b"), "git@github.com:un7qi3inc/repo-b.git")
	// Not ours — different org.
	writeRepo(t, filepath.Join(ws, "stranger"), "https://github.com/someone-else/stranger.git")
	// A repo nested under another repo's node_modules must be skipped.
	writeRepo(t, filepath.Join(ws, "repo-a", "node_modules", "dep"), "https://github.com/un7qi3inc/dep.git")

	counts := map[string]int{}
	scanRoot(root, 4, counts)

	if got := counts[ws]; got != 2 {
		t.Fatalf("expected 2 org repos under %s, got %d (counts=%v)", ws, got, counts)
	}
}

func TestScanRoot_RespectsMaxDepth(t *testing.T) {
	root := t.TempDir()
	// repo sits 6 levels below root — beyond maxDepth=2.
	deep := filepath.Join(root, "a", "b", "c", "d", "e", "repo")
	writeRepo(t, deep, "https://github.com/un7qi3inc/repo.git")

	counts := map[string]int{}
	scanRoot(root, 2, counts)

	if len(counts) != 0 {
		t.Fatalf("expected nothing within depth 2, got %v", counts)
	}
}
