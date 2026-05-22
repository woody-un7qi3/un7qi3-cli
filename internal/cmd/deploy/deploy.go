// Package deploy implements `uq deploy ...` commands.
package deploy

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

// NewCmd returns the `uq deploy` parent command.
func NewCmd() *cobra.Command {
	long := strings.Join([]string{
		output.Desc("레포 루트의 .uq.yml 매니페스트에 정의된 배포 워크플로를 실행합니다."),
		"",
		output.Desc("1차: ./build-<env>.sh 컨벤션. 2차: 매니페스트 cmd/requires/pre/confirm. 현재 stub."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq deploy run forceteller-api --env beta --dry-run", "dry-run"),
		output.HelpExample("uq deploy run forceteller-api --env prod --yes", "실 배포 (확인 스킵)"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: ".uq.yml에 정의된 배포 워크플로 실행",
		Long:  long,
	}
	cmd.AddCommand(newRunCmd())
	return cmd
}
