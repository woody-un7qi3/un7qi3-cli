// Package repo implements `uq repo ...` commands.
package repo

import "github.com/spf13/cobra"

// NewCmd returns the `uq repo` parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Work with un7qi3inc org repos",
	}
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newCloneCmd())
	return cmd
}
