package env

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newPullCmd() *cobra.Command {
	var envName string
	cmd := &cobra.Command{
		Use:   "pull <repo>",
		Short: "AWS SSM에서 시크릿을 로컬 파일로 가져오기",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "TODO: 아직 구현되지 않음 (Phase 0 stub)")
			return nil
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "environment (e.g. dev, beta, prod)")
	return cmd
}
