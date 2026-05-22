// Package skills implements the `uq skills ...` commands (Phase 3 stub).
package skills

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

// NewCmd returns the `uq skills` parent command.
func NewCmd() *cobra.Command {
	long := strings.Join([]string{
		output.Desc("uq 바이너리에 임베드된 Claude Code 스킬을 설치합니다."),
		"",
		output.Desc("uq 명령 체계와 짝을 이루는 하네스 스킬을 ~/.claude/skills/ 에 동기화."),
		output.Desc("아직 구현되지 않았습니다."),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "uq에 포함된 Claude Code 스킬 설치",
		Long:  long,
	}
	cmd.AddCommand(newInstallCmd())
	return cmd
}

func newInstallCmd() *cobra.Command {
	long := strings.Join([]string{
		output.Desc("임베드된 스킬을 ~/.claude/skills/ 에 복사합니다. 아직 stub."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq skills install", "전체 설치"),
	}, "\n")
	return &cobra.Command{
		Use:   "install",
		Short: "포함된 Claude Code 스킬을 ~/.claude/skills/에 설치",
		Long:  long,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "아직 구현되지 않음 (별도 후속 Phase에서 구현 예정)")
			return nil
		},
	}
}
