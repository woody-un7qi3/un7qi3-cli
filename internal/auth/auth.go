// Package auth wraps gh / aws / gcloud authentication probes and flows.
// It is consumed by the `uq auth` command group and re-used by `uq doctor`.
package auth

// Status describes the authentication state of a single provider.
type Status struct {
	Name    string `json:"name"`              // "gh", "aws", "gcloud"
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
// return an OK=false Status with Error set.
func StatusOf(name string) Status {
	switch name {
	case "gh":
		return GhStatus()
	case "aws":
		return AwsStatus()
	case "gcloud":
		return GcloudStatus()
	default:
		return Status{Name: name, OK: false, Error: "알 수 없는 provider"}
	}
}
