package env

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newDiffCmd() *cobra.Command {
	var envName string
	cmd := &cobra.Command{
		Use:   "diff <repo>",
		Short: "Diff local secrets against AWS SSM",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "TODO: not yet implemented (Phase 0 stub)")
			return nil
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "environment (e.g. dev, beta, prod)")
	return cmd
}
