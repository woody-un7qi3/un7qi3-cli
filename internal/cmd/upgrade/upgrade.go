// Package upgrade implements the `uq upgrade` command (Phase 2 stub).
package upgrade

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewCmd returns the `uq upgrade` command.
func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "uq를 최신 릴리즈로 업그레이드",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "Phase 2: 아직 릴리즈되지 않음")
			return nil
		},
	}
}
