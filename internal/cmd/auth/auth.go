// Package auth implements `uq auth ...` commands.
package auth

import "github.com/spf13/cobra"

// NewCmd returns the `uq auth` parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication (gh, aws sso)",
	}
	cmd.AddCommand(newLoginCmd())
	cmd.AddCommand(newLogoutCmd())
	cmd.AddCommand(newStatusCmd())
	return cmd
}
