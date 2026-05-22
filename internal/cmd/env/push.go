package env

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

func newPushCmd() *cobra.Command {
	var (
		envName string
		dryRun  bool
		yes     bool
	)
	long := strings.Join([]string{
		output.Desc("로컬 시크릿 파일을 AWS SSM Parameter Store에 업로드합니다."),
		output.Desc("기본은 ") + output.Yellow("--dry-run") + output.Desc(" — 변경 사항을 미리 보여주기만 합니다."),
		output.Desc("실제 업로드는 ") + output.Yellow("--dry-run=false --yes") + output.Desc(". 현재 stub."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq env push forceteller-api --env beta", "dry-run 미리보기"),
		output.HelpExample("uq env push forceteller-api --env beta --dry-run=false --yes", "실제 업로드"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "push <repo>",
		Short: "로컬 시크릿을 AWS SSM에 업로드",
		Long:  long,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "TODO: 아직 구현되지 않음 (Phase 0 stub)")
			return nil
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "환경 이름 (dev/beta/prod 등)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", true, "쓰기 없이 변경 미리보기 (기본 true)")
	cmd.Flags().BoolVar(&yes, "yes", false, "확인 프롬프트 스킵")
	return cmd
}
