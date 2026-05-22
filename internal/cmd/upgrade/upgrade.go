// Package upgrade implements the `uq upgrade` command (Phase 2 stub).
package upgrade

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewCmd returns the `uq upgrade` command.
func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade uq to the latest release",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "Phase 2: not yet released")
			return nil
		},
	}
}
