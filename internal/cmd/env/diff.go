package env

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

func newDiffCmd() *cobra.Command {
	var envName string
	long := strings.Join([]string{
		output.Desc("로컬 시크릿 파일과 AWS SSM의 값을 비교해 차이를 표시합니다. 현재 stub."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq env diff forceteller-api --env beta", "beta 환경 비교"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "diff <repo>",
		Short: "로컬 시크릿과 AWS SSM 비교",
		Long:  long,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "TODO: 아직 구현되지 않음 (Phase 0 stub)")
			return nil
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "환경 이름 (dev/beta/prod 등)")
	return cmd
}
