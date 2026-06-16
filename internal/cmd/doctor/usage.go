package doctor

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// FindReposUsingTool scans workspaceDir's first-level subdirectories for
// package.json files (up to maxDepth folders deep) that reference toolName
// in their scripts or list packageName in their dependencies/devDependencies.
//
// Per-repo walks run in parallel (bounded by GOMAXPROCS) so a 30-repo
// workspace scans well under a second even when several repos have deeply
// nested source trees.
//
// toolName is the CLI command we'd type ("ng", "ionic", "yarn").
// packageName is the npm package that ships it (e.g. "@angular/cli"). Pass
// "" to skip dependency checking and rely on scripts only.
func FindReposUsingTool(workspaceDir, toolName, packageName string, maxDepth int) []string {
	entries, err := os.ReadDir(workspaceDir)
	if err != nil {
		return nil
	}
	wordRe := regexp.MustCompile(`\b` + regexp.QuoteMeta(toolName) + `\b`)

	var (
		mu    sync.Mutex
		found = map[string]bool{}
		wg    sync.WaitGroup
		sem   = make(chan struct{}, max(2, runtime.GOMAXPROCS(0)))
	)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		repoRoot := filepath.Join(workspaceDir, name)
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if repoUsesTool(repoRoot, wordRe, packageName, maxDepth) {
				mu.Lock()
				found[name] = true
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	out := make([]string, 0, len(found))
	for k := range found {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// repoUsesTool walks repoRoot up to maxDepth subfolders looking for any
// package.json that references the tool by script regex or by named
// dependency. Returns true on the first match.
func repoUsesTool(repoRoot string, scriptRe *regexp.Regexp, packageName string, maxDepth int) bool {
	hit := false
	// Skip large directories that never contain a project-defining
	// package.json. Each one we can skip = thousands of stat calls saved.
	skipDirs := map[string]bool{
		"node_modules":     true,
		".git":             true,
		"dist":             true,
		"build":            true,
		".next":            true,
		".nuxt":            true,
		"coverage":         true,
		"out":              true,
		"target":           true, // Java/Rust
		".gradle":          true, // Gradle
		"Pods":             true, // CocoaPods (iOS)
		"vendor":           true, // PHP/Go vendored deps
		"bower_components": true,
		".idea":            true,
		".vscode":          true,
		".cache":           true,
		"tmp":              true,
		"temp":             true,
		"logs":             true,
		".terraform":       true,
		".dart_tool":       true,
		"platforms":        true, // Cordova
	}
	_ = filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Permission errors etc. — skip the offending entry, keep walking.
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if hit {
			return filepath.SkipAll
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			// Limit depth (repoRoot itself is depth 0).
			rel, _ := filepath.Rel(repoRoot, path)
			if rel != "." && strings.Count(rel, string(filepath.Separator))+1 > maxDepth {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "package.json" {
			return nil
		}
		if packageJSONReferencesTool(path, scriptRe, packageName) {
			hit = true
			return filepath.SkipAll
		}
		return nil
	})
	return hit
}

// packageJSONReferencesTool decodes path as JSON and reports whether
// scriptRe matches any script command OR packageName appears in
// dependencies / devDependencies.
func packageJSONReferencesTool(path string, scriptRe *regexp.Regexp, packageName string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var pkg struct {
		Scripts         map[string]string `json:"scripts"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}
	for _, cmd := range pkg.Scripts {
		if scriptRe.MatchString(cmd) {
			return true
		}
	}
	if packageName != "" {
		if _, ok := pkg.Dependencies[packageName]; ok {
			return true
		}
		if _, ok := pkg.DevDependencies[packageName]; ok {
			return true
		}
	}
	return false
}

// LocalBinInfo describes one repo's locally-installed copy of a tool.
type LocalBinInfo struct {
	Repo    string // top-level repo name (e.g. "forceteller-admin")
	Path    string // absolute path to the `.bin/<tool>` entry
	Subpath string // path from repo root to the package.json's dir, "" if at root
	Version string // resolved from the symlink target's package.json; "" if unreadable
}

// FindAllLocalBins enumerates every `node_modules/.bin/<binName>` under
// workspaceDir at depths 1..3, keeping one entry per top-level repo (so a
// repo with the tool installed in several subprojects shows up once).
//
// Glob beats a full walk here: candidate paths are well-known shapes,
//
//	~/un7qi3/<repo>/[<sub>/[<sub>/]]node_modules/.bin/<bin>
//
// and we never have to descend into node_modules subtrees that don't
// match. Each result's Version is read from the symlinked package's
// package.json — see VersionFromBinSymlink.
func FindAllLocalBins(workspaceDir, binName string) []LocalBinInfo {
	patterns := []string{
		filepath.Join(workspaceDir, "*", "node_modules", ".bin", binName),
		filepath.Join(workspaceDir, "*", "*", "node_modules", ".bin", binName),
		filepath.Join(workspaceDir, "*", "*", "*", "node_modules", ".bin", binName),
	}
	seen := map[string]bool{}
	var out []LocalBinInfo
	for _, p := range patterns {
		matches, _ := filepath.Glob(p)
		sort.Strings(matches)
		for _, m := range matches {
			rel, err := filepath.Rel(workspaceDir, m)
			if err != nil {
				continue
			}
			parts := strings.Split(rel, string(filepath.Separator))
			if len(parts) < 4 {
				// expect at least <repo>/node_modules/.bin/<bin>
				continue
			}
			repo := parts[0]
			if seen[repo] {
				continue
			}
			seen[repo] = true
			// m = .../node_modules/.bin/<bin>  → up 3 to package.json's dir
			pkgParent := filepath.Dir(filepath.Dir(filepath.Dir(m)))
			sub, _ := filepath.Rel(filepath.Join(workspaceDir, repo), pkgParent)
			if sub == "." {
				sub = ""
			}
			out = append(out, LocalBinInfo{
				Repo:    repo,
				Path:    m,
				Subpath: sub,
				Version: VersionFromBinSymlink(m),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Repo < out[j].Repo })
	return out
}

// FindFirstLocalBin is a thin convenience over FindAllLocalBins. Retained
// because callers that only need a yes/no answer read better with it.
func FindFirstLocalBin(workspaceDir, binName string) string {
	all := FindAllLocalBins(workspaceDir, binName)
	if len(all) == 0 {
		return ""
	}
	return all[0].Path
}

// VersionFromBinSymlink reads the version of a tool whose binary is the
// canonical npm `.bin/<tool>` symlink — i.e. it points at
// `../<package>/bin/<tool>`. We resolve the link, walk up two directories
// to the package root, and read its package.json.
//
// Returns "" when binPath isn't a symlink, the target doesn't follow the
// expected shape, or package.json is malformed. The version is meant for
// human display, so failing soft (no version printed) is fine.
func VersionFromBinSymlink(binPath string) string {
	target, err := os.Readlink(binPath)
	if err != nil {
		return ""
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(binPath), target)
	}
	target = filepath.Clean(target)
	// target = .../<pkg>/bin/<tool> → up twice → .../<pkg>/
	pkgDir := filepath.Dir(filepath.Dir(target))
	data, err := os.ReadFile(filepath.Join(pkgDir, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	return pkg.Version
}

// un7qi3WorkspaceDir returns the ~/un7qi3 workspace path, or "" when the
// user's home directory can't be determined.
func un7qi3WorkspaceDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, "un7qi3")
}

// un7qi3UsageSummary returns a Usage string ready for sub-line rendering.
// Empty when no ~/un7qi3 workspace exists or no repo references the tool.
func un7qi3UsageSummary(toolName, packageName string) string {
	workspace := un7qi3WorkspaceDir()
	if workspace == "" {
		return ""
	}
	repos := FindReposUsingTool(workspace, toolName, packageName, 4)
	if len(repos) == 0 {
		return ""
	}
	// Cap the list so doctor stays readable even if many repos use the tool.
	const maxList = 5
	if len(repos) > maxList {
		extra := len(repos) - maxList
		return fmt.Sprintf("used by: %s, ...외 %d개", strings.Join(repos[:maxList], ", "), extra)
	}
	return "used by: " + strings.Join(repos, ", ")
}
