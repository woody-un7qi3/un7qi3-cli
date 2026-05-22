// Package upgrade implements the `uq upgrade` command (Phase 2 stub).
package upgrade

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

// NewCmd returns the `uq upgrade` command.
func NewCmd() *cobra.Command {
	long := strings.Join([]string{
		output.Desc("uq를 GitHub Releases의 최신 버전으로 업그레이드합니다."),
		"",
		output.Desc("아직 릴리즈 인프라가 구축되지 않아 동작하지 않습니다."),
		output.Desc("로컬 빌드 사용 중에는 ") + output.Cyan("make install") + output.Desc(" 으로 재설치하세요."),
	}, "\n")
	return &cobra.Command{
		Use:   "upgrade",
		Short: "uq를 최신 릴리즈로 업그레이드",
		Long:  long,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "아직 릴리즈되지 않음 (별도 후속 Phase에서 구현 예정)")
			return nil
		},
	}
}
