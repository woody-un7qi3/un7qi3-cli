// Package run powers `uq run <repo>[:profile]`.
//
// node.go locates a Node.js runtime matching the version requested in a run
// profile (e.g. "16" or "16.20.2"), trying the version managers the user
// already has installed before falling back to PATH.
package run

import (
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
)

// NodeResolution is what ResolveNode returns when it finds a usable runtime.
type NodeResolution struct {
	BinDir  string // directory containing the `node` binary; PATH prepend candidate
	Version string // resolved full version, e.g. "16.20.2"
	Source  string // which detector matched: "fnm", "nvm", "mise", "asdf", "path"
}

// ResolveNode finds a Node runtime matching want.
//
// want may be:
//   - "" — no version constraint; first PATH node wins
//   - a major version like "16"
//   - an exact version like "16.20.2"
//
// Detector order: fnm → nvm → mise → asdf → PATH. The first detector that
// produces a matching version wins. Failure returns an error with install
// hints in Korean.
func ResolveNode(want string) (NodeResolution, error) {
	// 정확/접두 핀("16","20.5")은 매니저 우선(고정 버전 의도). 범위 제약(">=18")은
	// 현재 PATH 의 node 가 충족하면 그대로 쓰는 게 자연스럽다 — 보통 그게 nvm/fnm
	// 의 default 라서 전환 없이 사용자의 활성 node 를 존중한다. PATH 가 부족할 때만
	// 매니저에 설치된 충족 버전 중 가장 높은 것을 고른다.
	detectors := []func(string) (NodeResolution, bool){
		detectFnm, detectNvm, detectMise, detectAsdf, detectPath,
	}
	if isRange(want) {
		detectors = []func(string) (NodeResolution, bool){
			detectPath, detectFnm, detectNvm, detectMise, detectAsdf,
		}
	}
	for _, d := range detectors {
		if r, ok := d(want); ok {
			return r, nil
		}
	}
	if isRange(want) {
		return NodeResolution{}, fmt.Errorf(
			"Node %s 를 만족하는 버전을 찾지 못했습니다. 최신 LTS 를 설치하세요:\n"+
				"  fnm install --lts     # 권장\n"+
				"  nvm install --lts\n"+
				"  mise use -g node@lts",
			want,
		)
	}
	return NodeResolution{}, fmt.Errorf(
		"Node %s 을(를) 찾지 못했습니다. 다음 중 하나로 설치하세요:\n"+
			"  fnm install %s        # 권장\n"+
			"  nvm install %s\n"+
			"  mise install node@%s",
		displayWant(want), displayWant(want), displayWant(want), displayWant(want),
	)
}

// isRange 는 want 가 최소 버전 제약(">=N") 표현인지 판단한다.
func isRange(want string) bool {
	return strings.HasPrefix(want, ">=")
}

func displayWant(want string) string {
	if want == "" {
		return "(아무 버전)"
	}
	return want
}

// versionMatches reports whether got (e.g. "16.20.2") satisfies want.
// want "" matches anything. want "16" matches the major. want "16.20" matches
// major+minor. want "16.20.2" requires exact match.
func versionMatches(got, want string) bool {
	if want == "" {
		return true
	}
	got = strings.TrimPrefix(got, "v")
	// 최소 버전 제약 ">=N": got 이 N 이상이면 충족.
	if min, ok := strings.CutPrefix(want, ">="); ok {
		return !versionLess(got, strings.TrimSpace(min))
	}
	want = strings.TrimPrefix(want, "v")
	gp := strings.Split(got, ".")
	wp := strings.Split(want, ".")
	if len(wp) > len(gp) {
		return false
	}
	for i, w := range wp {
		if gp[i] != w {
			return false
		}
	}
	return true
}

// versionLess sorts version strings ascending by numeric components.
func versionLess(a, b string) bool {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")
	ap := strings.Split(a, ".")
	bp := strings.Split(b, ".")
	n := len(ap)
	if len(bp) < n {
		n = len(bp)
	}
	for i := 0; i < n; i++ {
		ai, _ := strconv.Atoi(ap[i])
		bi, _ := strconv.Atoi(bp[i])
		if ai != bi {
			return ai < bi
		}
	}
	return len(ap) < len(bp)
}

// nodeBinVersion calls `<binDir>/node --version` and returns the trimmed version.
func nodeBinVersion(binDir string) (string, bool) {
	node := filepath.Join(binDir, "node")
	if _, err := os.Stat(node); err != nil {
		return "", false
	}
	out, err := uqexec.Run(node, "--version")
	if err != nil {
		return "", false
	}
	v := strings.TrimSpace(strings.TrimPrefix(string(out), "v"))
	if v == "" {
		return "", false
	}
	return v, true
}

// detectFnm: `fnm list` lists installed versions; we glob the fnm install dir.
// Path layout (default): $XDG_DATA_HOME/fnm/node-versions/v<ver>/installation/bin/node
// or $HOME/.local/share/fnm/... — also $HOME/Library/Application Support/fnm/...
func detectFnm(want string) (NodeResolution, bool) {
	if !uqexec.LookPath("fnm") {
		return NodeResolution{}, false
	}
	roots := fnmCandidates()
	return scanVersionDirs(roots, "v*", filepath.Join("installation", "bin"), "fnm", want)
}

func fnmCandidates() []string {
	home, _ := os.UserHomeDir()
	var roots []string
	if dir := os.Getenv("FNM_DIR"); dir != "" {
		roots = append(roots, filepath.Join(dir, "node-versions"))
	}
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		roots = append(roots, filepath.Join(dir, "fnm", "node-versions"))
	}
	if home != "" {
		roots = append(roots,
			filepath.Join(home, ".local", "share", "fnm", "node-versions"),
			filepath.Join(home, "Library", "Application Support", "fnm", "node-versions"),
			filepath.Join(home, ".fnm", "node-versions"),
		)
	}
	return roots
}

// detectNvm: $NVM_DIR/versions/node/v<ver>/bin/node
func detectNvm(want string) (NodeResolution, bool) {
	home, _ := os.UserHomeDir()
	var roots []string
	if dir := os.Getenv("NVM_DIR"); dir != "" {
		roots = append(roots, filepath.Join(dir, "versions", "node"))
	}
	if home != "" {
		roots = append(roots, filepath.Join(home, ".nvm", "versions", "node"))
	}
	return scanVersionDirs(roots, "v*", "bin", "nvm", want)
}

// detectMise: `mise where node@<want>` returns the install dir; binary is in bin/.
// We also fall back to scanning $MISE_DATA_DIR/installs/node/<ver>.
func detectMise(want string) (NodeResolution, bool) {
	if uqexec.LookPath("mise") {
		query := "node"
		if want != "" {
			query = "node@" + want
		}
		out, err := uqexec.Run("mise", "where", query)
		if err == nil {
			dir := strings.TrimSpace(string(out))
			if dir != "" {
				bin := filepath.Join(dir, "bin")
				if v, ok := nodeBinVersion(bin); ok && versionMatches(v, want) {
					return NodeResolution{BinDir: bin, Version: v, Source: "mise"}, true
				}
			}
		}
	}
	home, _ := os.UserHomeDir()
	var roots []string
	if dir := os.Getenv("MISE_DATA_DIR"); dir != "" {
		roots = append(roots, filepath.Join(dir, "installs", "node"))
	}
	if home != "" {
		roots = append(roots, filepath.Join(home, ".local", "share", "mise", "installs", "node"))
	}
	return scanVersionDirs(roots, "*", "bin", "mise", want)
}

// detectAsdf: $ASDF_DATA_DIR/installs/nodejs/<ver>/bin/node
func detectAsdf(want string) (NodeResolution, bool) {
	home, _ := os.UserHomeDir()
	var roots []string
	if dir := os.Getenv("ASDF_DATA_DIR"); dir != "" {
		roots = append(roots, filepath.Join(dir, "installs", "nodejs"))
	}
	if home != "" {
		roots = append(roots, filepath.Join(home, ".asdf", "installs", "nodejs"))
	}
	return scanVersionDirs(roots, "*", "bin", "asdf", want)
}

// detectPath: whatever `node` is on PATH, if its version satisfies want.
func detectPath(want string) (NodeResolution, bool) {
	if !uqexec.LookPath("node") {
		return NodeResolution{}, false
	}
	out, err := uqexec.Run("node", "--version")
	if err != nil {
		return NodeResolution{}, false
	}
	v := strings.TrimSpace(strings.TrimPrefix(string(out), "v"))
	if !versionMatches(v, want) {
		return NodeResolution{}, false
	}
	binPath, err := osexec.LookPath("node")
	if err != nil {
		return NodeResolution{}, false
	}
	return NodeResolution{BinDir: filepath.Dir(binPath), Version: v, Source: "path"}, true
}

// scanVersionDirs walks one or more parent directories whose immediate children
// are version directories (matching pattern), and returns the highest version
// that satisfies want.
//
//	root/<entry>/<binSubpath>/node
//
// If multiple roots match, the first one with a valid result wins.
func scanVersionDirs(roots []string, pattern, binSubpath, source, want string) (NodeResolution, bool) {
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		var matches []struct {
			version, binDir string
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if pattern == "v*" && !strings.HasPrefix(name, "v") {
				continue
			}
			v := strings.TrimPrefix(name, "v")
			if !versionMatches(v, want) {
				continue
			}
			binDir := filepath.Join(root, name, binSubpath)
			actual, ok := nodeBinVersion(binDir)
			if !ok {
				continue
			}
			// Verify the binary's reported version still matches (handles weird symlinks).
			if !versionMatches(actual, want) {
				continue
			}
			matches = append(matches, struct{ version, binDir string }{actual, binDir})
		}
		if len(matches) == 0 {
			continue
		}
		sort.Slice(matches, func(i, j int) bool {
			return versionLess(matches[i].version, matches[j].version)
		})
		best := matches[len(matches)-1]
		return NodeResolution{BinDir: best.binDir, Version: best.version, Source: source}, true
	}
	return NodeResolution{}, false
}
