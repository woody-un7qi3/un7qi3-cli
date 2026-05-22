// Package logs implements the `uq logs <repo>` command.
package logs

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

// NewCmd returns the `uq logs` command (Phase 0 stub).
func NewCmd() *cobra.Command {
	var (
		envName  string
		instance string
		since    string
		grep     string
		noFollow bool
		split    bool
	)
	long := strings.Join([]string{
		output.Desc("Elastic Beanstalk 다중 인스턴스의 로그를 멀티플렉스로 스트리밍합니다."),
		"",
		output.Desc("기본은 ") + output.Yellow("--env") + output.Desc(" 전체 인스턴스를 한 스트림으로. ") + output.Yellow("--split") + output.Desc(" 으로 인스턴스별 패널 분리. 현재 stub."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq logs forceteller-api --env prod", "prod 전체 인스턴스"),
		output.HelpExample("uq logs forceteller-api --env prod --since 10m --grep ERROR", "최근 10분 ERROR만"),
		output.HelpExample("uq logs forceteller-api --env beta --split", "인스턴스별 패널 분리"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "logs <repo>",
		Short: "EB 인스턴스 멀티플렉스 로그 스트리밍",
		Long:  long,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "TODO: 아직 구현되지 않음 (Phase 0 stub)")
			return nil
		},
	}
	cmd.Flags().StringVar(&envName, "env", "", "환경 이름 (dev/beta/prod 등)")
	cmd.Flags().StringVar(&instance, "instance", "", "특정 인스턴스 ID로 제한")
	cmd.Flags().StringVar(&since, "since", "", "기간 내 로그만 (예: 10m, 1h)")
	cmd.Flags().StringVar(&grep, "grep", "", "정규식으로 라인 필터")
	cmd.Flags().BoolVar(&noFollow, "no-follow", false, "팔로우 없이 출력하고 종료")
	cmd.Flags().BoolVar(&split, "split", false, "인스턴스별 패널 분리")
	return cmd
}
