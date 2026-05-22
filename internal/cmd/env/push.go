package env

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newPushCmd() *cobra.Command {
	var (
		envName string
		dryRun  bool
		yes     bool
	)
	cmd := &cobra.Command{
		Use:   "push <repo>",
		Short: "Push local secrets up to AWS SSM",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "TODO: not yet implemented (Phase 0 stub)")
			return nil
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "environment (e.g. dev, beta, prod)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", true, "preview changes without writing (default: true)")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompt")
	return cmd
}
