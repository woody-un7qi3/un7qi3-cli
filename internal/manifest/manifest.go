// Package manifest parses per-repo .uq.yml files declaring secrets, deploy,
// and logs metadata.
package manifest

// Manifest mirrors the .uq.yml structure. Phase 0 stub — fields will be added
// alongside the commands that consume them.
type Manifest struct{}

// Load reads a .uq.yml from disk. Phase 0 stub.
func Load(path string) (*Manifest, error) {
	return &Manifest{}, nil
}
