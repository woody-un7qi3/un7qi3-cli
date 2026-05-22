// Package logs implements the `uq logs <repo>` command.
package logs

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewCmd returns the `uq logs` command (Phase 0 stub).
func NewCmd() *cobra.Command {
	var (
		envName  string
		instance string
		since    string
		grep     string
		noFollow bool
		split    bool
	)
	cmd := &cobra.Command{
		Use:   "logs <repo>",
		Short: "EB 인스턴스 멀티플렉스 로그 스트리밍",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "TODO: 아직 구현되지 않음 (Phase 0 stub)")
			return nil
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "environment (e.g. dev, beta, prod)")
	cmd.Flags().StringVar(&instance, "instance", "", "limit to specific instance id")
	cmd.Flags().StringVar(&since, "since", "", "show logs newer than duration (e.g. 10m)")
	cmd.Flags().StringVar(&grep, "grep", "", "filter log lines by regex")
	cmd.Flags().BoolVar(&noFollow, "no-follow", false, "do not follow; print and exit")
	cmd.Flags().BoolVar(&split, "split", false, "split panes per instance")
	return cmd
}
