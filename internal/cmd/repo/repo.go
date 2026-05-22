// Package repo implements `uq repo ...` commands.
package repo

import "github.com/spf13/cobra"

// NewCmd returns the `uq repo` parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "un7qi3inc 조직 레포 작업",
	}
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newCloneCmd())
	return cmd
}
