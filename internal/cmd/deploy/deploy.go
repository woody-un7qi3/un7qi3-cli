// Package deploy implements `uq deploy ...` commands.
package deploy

import "github.com/spf13/cobra"

// NewCmd returns the `uq deploy` parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Run deploy workflows declared in .uq.yml",
	}
	cmd.AddCommand(newRunCmd())
	return cmd
}
