package env

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

func newPullCmd() *cobra.Command {
	var envName string
	long := strings.Join([]string{
		output.Desc("AWS SSM Parameter Store에서 시크릿을 받아 로컬 파일로 떨굽니다."),
		output.Desc("매핑은 레포의 .uq.yml 매니페스트에 정의합니다. 현재 stub."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq env pull forceteller-api --env beta", "beta 환경 시크릿"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "pull <repo>",
		Short: "AWS SSM에서 시크릿을 로컬 파일로 가져오기",
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
