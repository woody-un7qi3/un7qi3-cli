// Package auth implements `uq auth ...` commands.
package auth

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

// NewCmd returns the `uq auth` parent command.
func NewCmd() *cobra.Command {
	long := strings.Join([]string{
		output.Desc("gh / aws / gcloud 세 provider의 인증을 관리합니다."),
		"",
		output.Desc("각 명령(login/logout/status)은 기본적으로 셋 다 처리하지만,"),
		output.Desc("플래그로 선택 가능합니다."),
		"",
		output.Heading("플래그"),
		output.HelpFlag("--gh-only / --aws-only / --gcloud-only", "해당 provider만"),
		output.HelpFlag("--skip-gh / --skip-aws / --skip-gcloud", "해당 provider 제외 (중복 가능)"),
		"",
		output.Heading("예시"),
		output.HelpExample("uq auth login", "셋 다 로그인"),
		output.HelpExample("uq auth login --gh-only", "gh만"),
		output.HelpExample("uq auth login --skip-gcloud", "gh + aws"),
		output.HelpExample("uq auth status --aws-only", "aws 상태만"),
	}, "\n")

	cmd := &cobra.Command{
		Use:   "auth",
		Short: "인증 관리 (gh, aws, gcloud)",
		Long:  long,
	}
	cmd.AddCommand(newLoginCmd())
	cmd.AddCommand(newLogoutCmd())
	cmd.AddCommand(newStatusCmd())
	return cmd
}
