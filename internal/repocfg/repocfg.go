// Package repocfg loads per-repo metadata bundled with the uq binary.
//
// The source of truth is repos.yml in this package, embedded at build time.
// To add or change a repo's branch list, edit that file and rebuild.
package repocfg

import (
	_ "embed"
	"fmt"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed repos.yml
var configBytes []byte

// DefaultLogPath 는 logs.path 미지정 시 사용할 EB 인스턴스 로그 경로.
const DefaultLogPath = "/var/log/web.stdout.log"

// LogsConfig 는 한 레포의 uq logs 설정. Countries 는 국가코드→(app, region).
type LogsConfig struct {
	Path      string                   `yaml:"path"`
	Countries map[string]CountryTarget `yaml:"countries"`
}

// CountryTarget 은 한 국가의 EB application 이름과 리전.
type CountryTarget struct {
	App    string `yaml:"app"`
	Region string `yaml:"region"`
}

// Config is the parsed shape of repos.yml.
type Config struct {
	Repos    map[string][]string  `yaml:"repos"`
	Defaults []string             `yaml:"defaults"`
	Runs     map[string]RepoRuns  `yaml:"runs"`
	Logs     map[string]LogsConfig `yaml:"logs"`
}

// RepoRuns groups the run profiles registered for a single repo.
type RepoRuns struct {
	Default  string             `yaml:"default"`
	Profiles map[string]Profile `yaml:"profiles"`
}

// Profile describes how to launch one or more commands for a repo.
//
// Exactly one of Cmd or Procs must be set:
//   - Cmd:   single foreground process (repo root as cwd).
//   - Procs: multiple concurrent processes, each with its own cwd inside the
//     repo. Their stdout/stderr are interleaved with a "[name] " prefix.
//
// Node, when set, names a Node runtime version that `uq run` prepends to PATH
// before exec. Env is merged on top of the inherited environment and applies
// to every process (Cmd or all Procs).
type Profile struct {
	Cmd   []string          `yaml:"cmd,omitempty"`
	Procs []Proc            `yaml:"procs,omitempty"`
	Node  string            `yaml:"node,omitempty"`
	Env   map[string]string `yaml:"env,omitempty"`
	// URL is the address the dev server listens on (single-cmd profiles).
	// Printed in the header so users know where to navigate once compile is done.
	URL string `yaml:"url,omitempty"`
	// Countries, when set, declares an axis orthogonal to the profile: the same
	// cmd/proc structure run against a different locale by swapping the npm
	// script name. Each argv's "{script}" token is replaced by the chosen
	// country's Script. Nil means the profile has no country axis (default).
	Countries *Countries `yaml:"countries,omitempty"`
}

// Countries declares the per-country variants of a profile.
//
// Default is the country code used when no --country flag is given and the
// session is non-interactive. Options lists the selectable countries in
// declaration order (the order shown in the TUI).
type Countries struct {
	Default string    `yaml:"default"`
	Options []Country `yaml:"options"`
}

// Country is one locale variant. Script is the npm script name substituted for
// the "{script}" token. Requires lists files (relative to the repo root) that
// must exist before launch — the underlying app crashes on a missing locale
// .env, so uq pre-flights them.
type Country struct {
	Code     string   `yaml:"code"`
	Script   string   `yaml:"script"`
	Requires []string `yaml:"requires,omitempty"`
}

// Find returns the option with the given code.
func (c *Countries) Find(code string) (Country, bool) {
	for _, o := range c.Options {
		if o.Code == code {
			return o, true
		}
	}
	return Country{}, false
}

// Codes returns the option codes in declaration order, joined for messages.
func (c *Countries) Codes() string {
	out := make([]string, 0, len(c.Options))
	for _, o := range c.Options {
		out = append(out, o.Code)
	}
	return strings.Join(out, ", ")
}

// SubstituteScript returns a copy of the profile with every "{script}" token in
// Cmd and each Proc.Cmd replaced by script. The original is left untouched so
// callers can substitute different scripts off one parsed profile.
func (p Profile) SubstituteScript(script string) Profile {
	sub := func(argv []string) []string {
		if argv == nil {
			return nil
		}
		out := make([]string, len(argv))
		for i, a := range argv {
			out[i] = strings.ReplaceAll(a, "{script}", script)
		}
		return out
	}
	p.Cmd = sub(p.Cmd)
	if p.Procs != nil {
		procs := make([]Proc, len(p.Procs))
		for i, pr := range p.Procs {
			pr.Cmd = sub(pr.Cmd)
			procs[i] = pr
		}
		p.Procs = procs
	}
	return p
}

// Proc is one of several concurrent processes in a multi-proc profile.
// Cwd is a path relative to the repo root; empty means the repo root itself.
type Proc struct {
	Name string   `yaml:"name"`
	Cwd  string   `yaml:"cwd,omitempty"`
	Cmd  []string `yaml:"cmd"`
	URL  string   `yaml:"url,omitempty"`
}

var (
	loadOnce sync.Once
	loaded   *Config
	loadErr  error
)

// Load returns the embedded config, parsing it once.
func Load() (*Config, error) {
	loadOnce.Do(func() {
		var c Config
		if err := yaml.Unmarshal(configBytes, &c); err != nil {
			loadErr = fmt.Errorf("repos.yml 파싱 실패: %w", err)
			return
		}
		loaded = &c
	})
	return loaded, loadErr
}

// BranchesFor returns the configured branches for name, falling back to
// Defaults when name has no explicit entry.
func (c *Config) BranchesFor(name string) []string {
	if br, ok := c.Repos[name]; ok && len(br) > 0 {
		return br
	}
	return c.Defaults
}

// ProfileFor resolves a run profile for repo by name.
//
// Selection rules:
//   - name == "" and Default is set → Default
//   - name == "" and Default is empty but exactly one profile exists → that one
//   - otherwise the explicitly-named profile (or an error listing available ones)
//
// The returned string is the resolved profile name (useful for status output).
func (c *Config) ProfileFor(repo, name string) (Profile, string, error) {
	runs, ok := c.Runs[repo]
	if !ok || len(runs.Profiles) == 0 {
		return Profile{}, "", fmt.Errorf("'%s' 에 등록된 실행 프로파일이 없습니다 — repos.yml 의 runs: 블록을 확인하세요", repo)
	}
	if name == "" {
		if runs.Default != "" {
			name = runs.Default
		} else if len(runs.Profiles) == 1 {
			for k := range runs.Profiles {
				name = k
			}
		} else {
			return Profile{}, "", fmt.Errorf("'%s' 에 default 프로파일이 없습니다 — 명시하세요. 사용 가능: %s", repo, joinKeys(runs.Profiles))
		}
	}
	p, ok := runs.Profiles[name]
	if !ok {
		return Profile{}, "", fmt.Errorf("'%s' 에 프로파일 '%s' 가 없습니다 — 사용 가능: %s", repo, name, joinKeys(runs.Profiles))
	}
	hasCmd := len(p.Cmd) > 0
	hasProcs := len(p.Procs) > 0
	switch {
	case !hasCmd && !hasProcs:
		return Profile{}, "", fmt.Errorf("'%s:%s' 프로파일에 cmd 도 procs 도 없습니다", repo, name)
	case hasCmd && hasProcs:
		return Profile{}, "", fmt.Errorf("'%s:%s' 프로파일에 cmd 와 procs 가 동시에 정의됨 — 하나만", repo, name)
	}
	if hasProcs {
		for i, pr := range p.Procs {
			if pr.Name == "" {
				return Profile{}, "", fmt.Errorf("'%s:%s' procs[%d] 의 name 이 비어 있습니다", repo, name, i)
			}
			if len(pr.Cmd) == 0 {
				return Profile{}, "", fmt.Errorf("'%s:%s' procs[%s] 의 cmd 가 비어 있습니다", repo, name, pr.Name)
			}
		}
	}
	return p, name, nil
}

func joinKeys(m map[string]Profile) string {
	if len(m) == 0 {
		return "(없음)"
	}
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	// stable for tests
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && names[j-1] > names[j]; j-- {
			names[j-1], names[j] = names[j], names[j-1]
		}
	}
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}

// PathOrDefault 는 설정된 로그 경로, 없으면 DefaultLogPath.
func (l LogsConfig) PathOrDefault() string {
	if l.Path != "" {
		return l.Path
	}
	return DefaultLogPath
}

// LogsFor 는 레포의 logs 설정을 반환한다. 미등록이면 ok=false.
func (c *Config) LogsFor(repo string) (LogsConfig, bool) {
	lc, ok := c.Logs[repo]
	return lc, ok
}
