package auth

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newLoginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Log in to gh + aws sso",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "TODO: not yet implemented (Phase 0 stub)")
			return nil
		},
	}
}
