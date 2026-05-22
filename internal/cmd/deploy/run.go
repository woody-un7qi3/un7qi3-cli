package deploy

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

func newRunCmd() *cobra.Command {
	var (
		envName string
		dryRun  bool
		yes     bool
	)
	long := strings.Join([]string{
		output.Desc("레포의 배포 스크립트(./build-<env>.sh) 또는 .uq.yml 매니페스트의 cmd 를 실행합니다."),
		output.Desc("프로덕션 환경에서는 확인 프롬프트가 뜨며, ") + output.Yellow("--yes") + output.Desc(" 로 스킵할 수 있습니다. 현재 stub."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq deploy run forceteller-api --env beta --dry-run", "계획만 출력"),
		output.HelpExample("uq deploy run forceteller-api --env prod --yes", "실 배포"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "run <repo>",
		Short: "레포 배포 스크립트 실행 (./build-<env>.sh 또는 매니페스트 cmd)",
		Long:  long,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "TODO: 아직 구현되지 않음 (Phase 0 stub)")
			return nil
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "환경 이름 (dev/beta/prod 등)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "실제 실행 없이 배포 계획만 출력")
	cmd.Flags().BoolVar(&yes, "yes", false, "확인 프롬프트 스킵")
	return cmd
}
