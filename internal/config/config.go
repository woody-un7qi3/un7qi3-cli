// Package config loads user configuration from ~/.config/un7qi3/config.yml.
package config

// Config holds user-level settings. Phase 0 stub — fields will be filled in
// as concrete commands need them.
type Config struct{}

// Load reads the config file. Phase 0 stub.
func Load(path string) (*Config, error) {
	return &Config{}, nil
}
