// Package env implements `uq env ...` commands for secrets management.
package env

import "github.com/spf13/cobra"

// NewCmd returns the `uq env` parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage repo secrets via AWS SSM (.env / .pem / .json)",
	}
	cmd.AddCommand(newPullCmd())
	cmd.AddCommand(newPushCmd())
	cmd.AddCommand(newDiffCmd())
	return cmd
}
