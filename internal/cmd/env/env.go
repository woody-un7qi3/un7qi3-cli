// Package env implements `uq env ...` commands for secrets management.
package env

import "github.com/spf13/cobra"

// NewCmd returns the `uq env` parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "AWS SSM 기반 레포 시크릿 관리 (.env / .pem / .json)",
	}
	cmd.AddCommand(newPullCmd())
	cmd.AddCommand(newPushCmd())
	cmd.AddCommand(newDiffCmd())
	return cmd
}
