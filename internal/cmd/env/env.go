// Package env implements `uq env ...` commands for secrets management.
package env

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

// NewCmd returns the `uq env` parent command.
func NewCmd() *cobra.Command {
	long := strings.Join([]string{
		output.Desc("레포 시크릿(.env / .pem / .json)을 AWS SSM Parameter Store와 동기화합니다."),
		"",
		output.Desc("각 레포의 .uq.yml 매니페스트에서 파라미터 매핑을 읽어 사용합니다."),
		output.Desc("현재 stub — 후속 Phase에서 구현됩니다."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq env pull <repo>", "SSM에서 로컬로"),
		output.HelpExample("uq env push <repo> --dry-run", "변경 미리보기"),
		output.HelpExample("uq env diff <repo>", "로컬 vs SSM 비교"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "env",
		Short: "AWS SSM 기반 레포 시크릿 관리 (.env / .pem / .json)",
		Long:  long,
	}
	cmd.AddCommand(newPullCmd())
	cmd.AddCommand(newPushCmd())
	cmd.AddCommand(newDiffCmd())
	return cmd
}
