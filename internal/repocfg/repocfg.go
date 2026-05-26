// Package repocfg loads per-repo metadata bundled with the uq binary.
//
// The source of truth is repos.yml in this package, embedded at build time.
// To add or change a repo's branch list, edit that file and rebuild.
package repocfg

import (
	_ "embed"
	"fmt"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed repos.yml
var configBytes []byte

// Config is the parsed shape of repos.yml.
type Config struct {
	Repos    map[string][]string `yaml:"repos"`
	Defaults []string            `yaml:"defaults"`
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
