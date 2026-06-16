package doctor

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writePkg drops a minimal package.json at path with the given scripts and
// dependency maps. Errors fail the test — fixture setup should never fail.
func writePkg(t *testing.T, path string, scripts, deps, devDeps map[string]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "{"
	first := true
	addSection := func(name string, m map[string]string) {
		if len(m) == 0 {
			return
		}
		if !first {
			body += ","
		}
		first = false
		body += `"` + name + `":{`
		i := 0
		for k, v := range m {
			if i > 0 {
				body += ","
			}
			body += `"` + k + `":"` + v + `"`
			i++
		}
		body += "}"
	}
	addSection("scripts", scripts)
	addSection("dependencies", deps)
	addSection("devDependencies", devDeps)
	body += "}"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindReposUsingTool_MatchesScripts(t *testing.T) {
	tmp := t.TempDir()
	// repo-a uses ng in scripts.
	writePkg(t, filepath.Join(tmp, "repo-a", "package.json"),
		map[string]string{"start": "ng serve --port 4200"}, nil, nil)
	// repo-b has nothing relevant.
	writePkg(t, filepath.Join(tmp, "repo-b", "package.json"),
		map[string]string{"build": "tsc"}, nil, nil)
	got := FindReposUsingTool(tmp, "ng", "@angular/cli", 3)
	if !reflect.DeepEqual(got, []string{"repo-a"}) {
		t.Errorf("got %v, want [repo-a]", got)
	}
}

func TestFindReposUsingTool_MatchesDependencyName(t *testing.T) {
	tmp := t.TempDir()
	// scripts say nothing, but devDependencies list @angular/cli.
	writePkg(t, filepath.Join(tmp, "repo-a", "package.json"),
		map[string]string{"build": "tsc"}, nil,
		map[string]string{"@angular/cli": "^17"})
	got := FindReposUsingTool(tmp, "ng", "@angular/cli", 3)
	if !reflect.DeepEqual(got, []string{"repo-a"}) {
		t.Errorf("got %v, want [repo-a]", got)
	}
}

func TestFindReposUsingTool_NestedPackageJson(t *testing.T) {
	tmp := t.TempDir()
	// forceteller-admin style: front-end/workspace/package.json
	writePkg(t, filepath.Join(tmp, "admin", "front-end", "workspace", "package.json"),
		map[string]string{"start": "ng serve"}, nil, nil)
	got := FindReposUsingTool(tmp, "ng", "@angular/cli", 4)
	if !reflect.DeepEqual(got, []string{"admin"}) {
		t.Errorf("got %v, want [admin]", got)
	}
}

func TestFindReposUsingTool_WordBoundary(t *testing.T) {
	tmp := t.TempDir()
	// "ng" must NOT match "using" / "training" / etc.
	writePkg(t, filepath.Join(tmp, "repo-a", "package.json"),
		map[string]string{"docs": "echo using training"}, nil, nil)
	got := FindReposUsingTool(tmp, "ng", "@angular/cli", 3)
	if len(got) != 0 {
		t.Errorf("got %v, want empty (no whole-word match)", got)
	}
}

func TestFindReposUsingTool_SkipsHeavyDirs(t *testing.T) {
	tmp := t.TempDir()
	// A nested package.json inside node_modules must NOT be considered.
	writePkg(t, filepath.Join(tmp, "repo-a", "node_modules", "ng-lib", "package.json"),
		map[string]string{"x": "ng build"}, nil, nil)
	// And nothing at the top.
	writePkg(t, filepath.Join(tmp, "repo-a", "package.json"),
		map[string]string{"build": "tsc"}, nil, nil)
	got := FindReposUsingTool(tmp, "ng", "@angular/cli", 5)
	if len(got) != 0 {
		t.Errorf("got %v, want empty (node_modules should be skipped)", got)
	}
}

func TestFindReposUsingTool_MultipleMatchesSorted(t *testing.T) {
	tmp := t.TempDir()
	for _, name := range []string{"zebra", "alpha", "mango"} {
		writePkg(t, filepath.Join(tmp, name, "package.json"),
			map[string]string{"start": "ng serve"}, nil, nil)
	}
	writePkg(t, filepath.Join(tmp, "ignored", "package.json"),
		map[string]string{"build": "tsc"}, nil, nil)
	got := FindReposUsingTool(tmp, "ng", "@angular/cli", 3)
	want := []string{"alpha", "mango", "zebra"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFindReposUsingTool_NonexistentWorkspace(t *testing.T) {
	// Should not panic — empty result.
	if got := FindReposUsingTool("/nonexistent/path/that/does/not/exist", "ng", "@angular/cli", 3); len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

// FindFirstLocalBin must locate node_modules/.bin/<tool> at any of the
// canonical depths (repo root, repo/sub, repo/sub/sub) and ignore other
// paths.
func TestFindFirstLocalBin_AtVariousDepths(t *testing.T) {
	tmp := t.TempDir()
	for _, sub := range []string{"a", "b/c", "d/e/f"} {
		dir := filepath.Join(tmp, "repo-"+strings.ReplaceAll(sub, "/", "-"), sub, "node_modules", ".bin")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "ng"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := FindFirstLocalBin(tmp, "ng")
	if got == "" {
		t.Fatal("expected to find ng somewhere")
	}
	if !strings.HasSuffix(got, "node_modules/.bin/ng") {
		t.Errorf("got %q, expected ...node_modules/.bin/ng", got)
	}
}

func TestFindFirstLocalBin_Missing(t *testing.T) {
	tmp := t.TempDir()
	// One repo, but no node_modules/.bin/ng inside.
	if err := os.MkdirAll(filepath.Join(tmp, "repo", "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := FindFirstLocalBin(tmp, "ng"); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// VersionFromBinSymlink resolves the symlink and reads the version from
// the package.json two directories above the link target — the exact
// shape npm install produces.
func TestVersionFromBinSymlink(t *testing.T) {
	tmp := t.TempDir()
	// Simulate node_modules layout:
	//   tmp/node_modules/@angular/cli/package.json
	//   tmp/node_modules/@angular/cli/bin/ng
	//   tmp/node_modules/.bin/ng -> ../@angular/cli/bin/ng
	pkgDir := filepath.Join(tmp, "node_modules", "@angular", "cli")
	if err := os.MkdirAll(filepath.Join(pkgDir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"),
		[]byte(`{"name":"@angular/cli","version":"17.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	binTarget := filepath.Join(pkgDir, "bin", "ng")
	if err := os.WriteFile(binTarget, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(tmp, "node_modules", ".bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	binLink := filepath.Join(binDir, "ng")
	if err := os.Symlink("../@angular/cli/bin/ng", binLink); err != nil {
		t.Fatal(err)
	}
	if got := VersionFromBinSymlink(binLink); got != "17.0.0" {
		t.Errorf("version = %q, want 17.0.0", got)
	}
}

func TestVersionFromBinSymlink_NotASymlink(t *testing.T) {
	tmp := t.TempDir()
	plain := filepath.Join(tmp, "ng")
	if err := os.WriteFile(plain, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := VersionFromBinSymlink(plain); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// makeNgInstall lays down a realistic node_modules/.bin/ng layout with a
// resolvable version. Returns the bin symlink path so tests can re-use it
// directly.
func makeNgInstall(t *testing.T, root string, version string) string {
	t.Helper()
	pkgDir := filepath.Join(root, "node_modules", "@angular", "cli")
	if err := os.MkdirAll(filepath.Join(pkgDir, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "package.json"),
		[]byte(`{"name":"@angular/cli","version":"`+version+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "bin", "ng"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(root, "node_modules", ".bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	binLink := filepath.Join(binDir, "ng")
	if err := os.Symlink("../@angular/cli/bin/ng", binLink); err != nil {
		t.Fatal(err)
	}
	return binLink
}

// FindAllLocalBins must collect every repo with a local install, picking
// just one path per repo (so multi-project repos with several install
// points still show up exactly once), with versions resolved from each
// install's package.json.
func TestFindAllLocalBins_AcrossRepos(t *testing.T) {
	tmp := t.TempDir()
	makeNgInstall(t, filepath.Join(tmp, "alpha"), "17.0.0")
	// "bravo" has the install one level deeper (typical front-end/workspace pattern).
	makeNgInstall(t, filepath.Join(tmp, "bravo", "front-end", "workspace"), "13.0.0")
	// "delta" has TWO local installs (root and a subproject). Only one row expected.
	makeNgInstall(t, filepath.Join(tmp, "delta"), "9.1.15")
	makeNgInstall(t, filepath.Join(tmp, "delta", "projects", "x"), "9.1.15")
	// "noop" has nothing — should not appear.
	if err := os.MkdirAll(filepath.Join(tmp, "noop", "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := FindAllLocalBins(tmp, "ng")
	if len(got) != 3 {
		t.Fatalf("got %d repos, want 3: %#v", len(got), got)
	}
	wantRepos := []string{"alpha", "bravo", "delta"}
	for i, want := range wantRepos {
		if got[i].Repo != want {
			t.Errorf("idx %d: got repo %q, want %q", i, got[i].Repo, want)
		}
	}
	if got[1].Subpath != filepath.Join("front-end", "workspace") {
		t.Errorf("bravo subpath: %q", got[1].Subpath)
	}
	if got[0].Version != "17.0.0" || got[1].Version != "13.0.0" || got[2].Version != "9.1.15" {
		t.Errorf("versions mismatched: %v / %v / %v",
			got[0].Version, got[1].Version, got[2].Version)
	}
}
