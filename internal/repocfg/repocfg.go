// Package repocfg loads per-repo metadata bundled with the uq binary.
//
// The source of truth is repos.yml in this package, embedded at build time.
// To add or change a repo's branch list, edit that file and rebuild.
package repocfg

import (
	_ "embed"
	"fmt"
	"sort"
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

// Switch 는 실행 전 토글 가능한 소스 파일 설정 하나를 기술한다(예: API 서버).
// Scopes 는 먼저 고르는 축(예: 로케일 kr/jp). scope 가 1개면 선택 단계를 건너뛴다.
type Switch struct {
	Name   string        `yaml:"name"`   // 표시 이름 (예: "API 서버")
	File   string        `yaml:"file"`   // repo 루트 기준 상대 경로
	Scopes []SwitchScope `yaml:"scopes"` // 선택 축(예: 로케일). 1개면 선택 생략
}

// SwitchScope 는 파일 안의 한 영역(예: env 의 kr 블록)과 그 영역에서 고를 옵션들.
// Anchor 가 있으면 그 문자열 이후 영역에서만 감지·치환한다(블록 한정 — 같은 값이
// 다른 블록에도 있어도 안전). Anchor 가 비면 파일 전체가 대상이다.
type SwitchScope struct {
	Label   string         `yaml:"label"`            // 표시명 (예: "kr")
	Anchor  string         `yaml:"anchor,omitempty"` // 이 문자열 이후에서만 처리
	Options []SwitchOption `yaml:"options"`          // 상호배타 옵션들
}

// SwitchOption 은 scope 의 한 선택지. Match 는 파일에 들어가는(=현재 감지하는)
// 정확한 문자열이다. Match 에 "{port}" 플레이스홀더가 있으면, 이 옵션을 고를 때
// 사용자에게 포트를 입력받아 치환하고(감지는 localhost:<숫자> 처럼 숫자부) 처리한다.
// Default 는 그 입력 프롬프트의 기본값(현재 파일값이 있으면 그게 우선).
type SwitchOption struct {
	Label   string `yaml:"label"`
	Match   string `yaml:"match"`
	Default string `yaml:"default,omitempty"`
}

// Config is the parsed shape of repos.yml.
type Config struct {
	Repos    map[string][]string   `yaml:"repos"`
	Defaults []string              `yaml:"defaults"`
	Runs     map[string]RepoRuns   `yaml:"runs"`
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
	// Desc 는 프로파일의 짧은 용도 설명(예: 로케일 "kr, jp"). 선택 picker·목록에
	// 표시용으로만 쓰이고 실행에는 영향이 없다.
	Desc string `yaml:"desc,omitempty"`
	// Switches 는 실행 전 대화형으로 토글할 수 있는 소스 파일 설정들(예: API 서버를
	// 원격↔localhost 로). uq run list 흐름에서 프로파일 선택 뒤 단계로 노출된다.
	Switches []Switch `yaml:"switches,omitempty"`
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
	sort.Strings(names) // 알파벳 오름차순 — 에러 메시지 출력 안정화
	return strings.Join(names, ", ")
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

// LogsRepos 는 logs 가 등록된 레포 이름을 정렬해 반환한다(help/에러 안내용).
func (c *Config) LogsRepos() []string {
	repos := make([]string, 0, len(c.Logs))
	for name := range c.Logs {
		repos = append(repos, name)
	}
	sort.Strings(repos)
	return repos
}
