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
		Short: "로컬 시크릿을 AWS SSM에 업로드",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "TODO: 아직 구현되지 않음 (Phase 0 stub)")
			return nil
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "environment (e.g. dev, beta, prod)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", true, "preview changes without writing (default: true)")
	cmd.Flags().BoolVar(&yes, "yes", false, "skip confirmation prompt")
	return cmd
}
