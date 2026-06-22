package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/un7qi3inc/un7qi3-cli/internal/auth"
	"github.com/un7qi3inc/un7qi3-cli/internal/clierr"
	"github.com/un7qi3inc/un7qi3-cli/internal/cmd"
)

func main() {
	// Exit codes are decided in one place by clierr.Classify; see its package
	// doc for the full convention. main is the single exit point so that defer
	// cleanup in command bodies runs before the process leaves.
	//   0 — success
	//   1 — runtime error (recognised domain errors)
	//   2 — usage error: cobra unknown commands / flag parse / Args violations,
	//       and any error not otherwise classified (historical default).
	//   4 — authentication required (auth.RequiredError returned from RunE)
	err := cmd.Execute()
	code := clierr.Classify(err)
	if code == 0 {
		return
	}

	// Auth-required errors print their bare message to stderr — the form
	// users have always seen for exit 4. (cobra also prints "Error: ..."
	// since SilenceErrors is false in root.go; this pre-existing double
	// line is preserved here, not introduced by the refactor.)
	var authErr *auth.RequiredError
	if errors.As(err, &authErr) {
		fmt.Fprintln(os.Stderr, authErr.Msg)
	}

	os.Exit(code)
}
