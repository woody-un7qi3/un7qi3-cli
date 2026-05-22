package env

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newPullCmd() *cobra.Command {
	var envName string
	cmd := &cobra.Command{
		Use:   "pull <repo>",
		Short: "Pull secrets from AWS SSM into local files",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "TODO: not yet implemented (Phase 0 stub)")
			return nil
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "environment (e.g. dev, beta, prod)")
	return cmd
}
