package deploy

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var (
		envName string
		dryRun  bool
		yes     bool
	)
	cmd := &cobra.Command{
		Use:   "run <repo>",
		Short: "Run a repo's deploy script (./build-<env>.sh or manifest cmd)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "TODO: not yet implemented (Phase 0 stub)")
			return nil
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "environment (e.g. dev, beta, prod)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the deploy plan without running")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompt")
	return cmd
}
