// Package repo implements `uq repo ...` commands.
package repo

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

// NewCmd returns the `uq repo` parent command.
func NewCmd() *cobra.Command {
	long := strings.Join([]string{
		output.Desc("un7qi3inc 조직의 GitHub 레포를 다룹니다."),
		"",
		output.Desc("모든 git 작업은 gh 인증 토큰으로 처리되므로 SSH 키 설정이 필요 없습니다."),
		output.Desc("(uq auth login에서 gh auth setup-git 자동 실행)"),
		"",
		output.Heading("예시"),
		output.HelpExample("uq repo list", "전체 레포 목록"),
		output.HelpExample("uq repo list --json | jq '.[].name'", "이름만 추출"),
		output.HelpExample("uq repo clone <name>", "~/un7qi3/<name>에 클론"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "un7qi3inc 조직 레포 작업",
		Long:  long,
	}
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newCloneCmd())
	cmd.AddCommand(newPullCmd())
	return cmd
}
