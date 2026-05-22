package main

import (
	"os"

	"github.com/un7qi3inc/un7qi3-cli/internal/cmd"
)

func main() {
	// cobra returns non-nil for unknown commands, flag parse failures, and
	// Args validator violations — all of which are usage errors. RunE bodies
	// either return nil or call os.Exit themselves with the appropriate code.
	// gh convention: 0=ok, 1=runtime error, 2=usage error.
	if err := cmd.Execute(); err != nil {
		os.Exit(2)
	}
}
