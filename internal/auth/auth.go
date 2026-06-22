// Package auth wraps gh / aws / gcloud authentication probes and flows.
// It is consumed by the `uq auth` command group and re-used by `uq doctor`.
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/un7qi3inc/un7qi3-cli/internal/clierr"
	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
)

// defaultRunner is the Runner the package-level probes (GhStatus, AwsStatus,
// GcloudStatus) use in production. Tests exercise the injectable *Status core
// functions with a fake Runner instead of mutating this.
var defaultRunner uqexec.Runner = uqexec.Default()

// statusProbeTimeout bounds a single provider status probe. Without it a hung
// network or unresponsive auth daemon would make `uq auth status` / `uq doctor`
// block forever; with it the probe fails as "타임아웃" and the command moves on.
const statusProbeTimeout = 10 * time.Second

// init teaches the central error classifier that *RequiredError means exit 4.
// Registering here (rather than importing auth from clierr) keeps clierr free
// of an auth dependency and avoids an import cycle.
func init() {
	clierr.RegisterAuthRequired(func(err error) bool {
		var re *RequiredError
		return errors.As(err, &re)
	})
}

// Status describes the authentication state of a single provider.
type Status struct {
	Name    string `json:"name"` // "gh", "aws", "gcloud"
	OK      bool   `json:"ok"`
	User    string `json:"user,omitempty"`    // gh-only
	Account string `json:"account,omitempty"` // aws account id / gcloud email
	Arn     string `json:"arn,omitempty"`     // aws-only
	Detail  string `json:"detail,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Summary aggregates a report.
type Summary struct {
	OK     int `json:"ok"`
	Failed int `json:"failed"`
}

// Report is the top-level JSON payload for `uq auth status --json`.
type Report struct {
	Providers []Status `json:"providers"`
	Summary   Summary  `json:"summary"`
}

// ProviderNames lists supported providers in canonical order.
var ProviderNames = []string{"gh", "aws", "gcloud"}

// RequiredError signals that an authentication step is required (exit code 4).
// main inspects this via errors.As to set the process exit code without
// leaking exit logic into individual command bodies.
type RequiredError struct {
	Msg string
}

// Error implements the error interface.
func (e *RequiredError) Error() string { return e.Msg }

// StatusOf collects the Status for a single provider name. Unknown names
// return an OK=false Status with Error set. ctx flows into the provider probe
// so callers (auth status, doctor) can bound or cancel it.
func StatusOf(ctx context.Context, name string) Status {
	switch name {
	case "gh":
		return GhStatus(ctx)
	case "aws":
		return AwsStatus(ctx)
	case "gcloud":
		return GcloudStatus(ctx)
	default:
		return Status{Name: name, OK: false, Error: "알 수 없는 provider"}
	}
}

// probeTimeoutMsg maps a probe error to a status-friendly message, replacing a
// context-deadline error with an explicit "타임아웃" note so the report shows
// why the provider was skipped rather than a raw "context deadline exceeded".
func probeTimeoutMsg(ctx context.Context, name string, err error) (string, bool) {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return name + " 상태 확인 타임아웃 (" + statusProbeTimeout.String() + " 초과)", true
	}
	return "", false
}
