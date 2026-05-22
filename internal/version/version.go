// Package version holds build metadata injected via ldflags.
package version

// These variables are overridden at build time via:
//
//	-ldflags "-X github.com/un7qi3inc/un7qi3-cli/internal/version.Version=..."
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)
