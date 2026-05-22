package repo

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCloneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clone <name>",
		Short: "Clone a repo into ~/un7qi3/<name>",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "TODO: not yet implemented (Phase 0 stub)")
			return nil
		},
	}
}
