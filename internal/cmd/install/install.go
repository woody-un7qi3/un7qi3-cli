// Package install implements the `uq install <team>` command.
package install

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewCmd returns the `uq install` command (Phase 0 stub).
func NewCmd() *cobra.Command {
	var (
		all      bool
		selectFs []string
		list     bool
	)
	cmd := &cobra.Command{
		Use:   "install <team>",
		Short: "팀 레포 일괄 클론 (TUI 다중 선택)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "TODO: 아직 구현되지 않음 (Phase 0 stub)")
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "select all repos non-interactively")
	cmd.Flags().StringSliceVar(&selectFs, "select", nil, "comma-separated repo names")
	cmd.Flags().BoolVar(&list, "list", false, "list available repos and exit")
	return cmd
}
