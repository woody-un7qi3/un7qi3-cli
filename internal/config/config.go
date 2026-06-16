// Package config loads user configuration from ~/.config/un7qi3/config.yml.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// reposDirEnv overrides the configured repos dir when set (CI/임시 작업용).
const reposDirEnv = "UQ_REPOS_DIR"

// defaultReposDirName is the workspace folder under $HOME used when nothing
// has been configured.
const defaultReposDirName = "un7qi3"

// Config holds user-level settings persisted in config.yml.
type Config struct {
	// ReposDir is the directory under which org repos are cloned
	// (uq repo clone/pull, uq run). Empty means "not configured" —
	// callers fall back to the default.
	ReposDir string `yaml:"repos_dir,omitempty"`
}

// Path returns the config file location:
// $XDG_CONFIG_HOME/un7qi3/config.yml (default ~/.config/un7qi3/config.yml).
func Path() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "un7qi3", "config.yml"), nil
}

// Load reads config.yml. A missing file is not an error — it returns an
// empty Config so callers can apply defaults.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("%s 읽기 실패: %w", path, err)
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("%s 파싱 실패: %w", path, err)
	}
	return &c, nil
}

// Save writes c to config.yml, creating the parent directory if needed.
func Save(c *Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("설정 디렉토리 생성 실패: %w", err)
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("%s 쓰기 실패: %w", path, err)
	}
	return nil
}

// ReposDir resolves the directory under which org repos live, in priority:
//  1. $UQ_REPOS_DIR (explicit override)
//  2. repos_dir from config.yml
//  3. default ~/un7qi3
//
// The returned path is absolute with ~ and $VARS expanded.
func ReposDir() (string, error) {
	if v := os.Getenv(reposDirEnv); v != "" {
		return expandPath(v)
	}
	c, err := Load()
	if err != nil {
		return "", err
	}
	if c.ReposDir != "" {
		return expandPath(c.ReposDir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, defaultReposDirName), nil
}

// IsReposDirConfigured reports whether the repos dir has been explicitly set
// (env or config). Callers use this to decide whether to run onboarding.
func IsReposDirConfigured() bool {
	if os.Getenv(reposDirEnv) != "" {
		return true
	}
	c, err := Load()
	return err == nil && c.ReposDir != ""
}

// expandPath turns ~ and $VARS into an absolute, cleaned path.
func expandPath(p string) (string, error) {
	p = os.ExpandEnv(p)
	switch {
	case p == "~":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = home
	case strings.HasPrefix(p, "~/"):
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, p[2:])
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return abs, nil
}
