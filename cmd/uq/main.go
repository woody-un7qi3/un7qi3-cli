package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/un7qi3inc/un7qi3-cli/internal/auth"
	"github.com/un7qi3inc/un7qi3-cli/internal/cmd"
)

func main() {
	// Exit code conventions (gh-style):
	//   0 — success
	//   1 — runtime error: commands signal this by calling os.Exit(1) inside
	//       RunE (e.g. doctor failed checks, repo clone target exists)
	//   2 — usage error: cobra returns non-nil for unknown commands, flag
	//       parse failures, Args validator violations, mutually-exclusive flag
	//       conflicts. Any error bubbling out of Execute() falls here unless
	//       classified below.
	//   4 — authentication required (auth.RequiredError returned from RunE)
	err := cmd.Execute()
	if err == nil {
		return
	}

	var authErr *auth.RequiredError
	if errors.As(err, &authErr) {
		fmt.Fprintln(os.Stderr, authErr.Msg)
		os.Exit(4)
	}

	os.Exit(2)
}
