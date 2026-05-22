// Package auth implements `uq auth ...` commands.
package auth

import "github.com/spf13/cobra"

// NewCmd returns the `uq auth` parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "인증 관리 (gh, aws sso)",
	}
	cmd.AddCommand(newLoginCmd())
	cmd.AddCommand(newLogoutCmd())
	cmd.AddCommand(newStatusCmd())
	return cmd
}
