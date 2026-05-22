// Package skills implements the `uq skills ...` commands (Phase 3 stub).
package skills

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewCmd returns the `uq skills` parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "Install Claude Code skills bundled with uq",
	}
	cmd.AddCommand(newInstallCmd())
	return cmd
}

func newInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install bundled Claude Code skills into ~/.claude/skills/",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "Phase 3 stub: not yet implemented")
			return nil
		},
	}
}
