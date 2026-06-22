package auth_test

import (
	"fmt"
	"testing"

	"github.com/un7qi3inc/un7qi3-cli/internal/auth"
	"github.com/un7qi3inc/un7qi3-cli/internal/clierr"
)

// TestRequiredErrorClassifiesAsAuth verifies the init-time registration in the
// auth package teaches clierr that *auth.RequiredError means exit 4, both bare
// and wrapped. This is the behaviour main.go relies on for the exit-4 path.
func TestRequiredErrorClassifiesAsAuth(t *testing.T) {
	bare := &auth.RequiredError{Msg: "login required"}
	if got := clierr.Classify(bare); got != 4 {
		t.Errorf("Classify(*auth.RequiredError) = %d, want 4", got)
	}
	wrapped := fmt.Errorf("precondition: %w", bare)
	if got := clierr.Classify(wrapped); got != 4 {
		t.Errorf("Classify(wrapped *auth.RequiredError) = %d, want 4", got)
	}
}
