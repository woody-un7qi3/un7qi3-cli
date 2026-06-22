package auth

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	authpkg "github.com/un7qi3inc/un7qi3-cli/internal/auth"
	"github.com/un7qi3inc/un7qi3-cli/internal/clierr"
	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

func newLoginCmd() *cobra.Command {
	long := strings.Join([]string{
		output.Desc("세 provider(gh / aws / gcloud)에 로그인합니다."),
		"",
		output.Desc("이미 인증된 provider는 스킵하고, gh 로그인 성공 시 ") + output.Cyan("gh auth setup-git") + output.Desc("을"),
		output.Desc("추가 실행하여 git credential helper를 등록합니다 (SSH 키 없이 git push 가능)."),
		output.Desc("한 provider가 실패해도 나머지는 계속 시도하며, 하나라도 실패하면 exit 1."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq auth login", "셋 다"),
		output.HelpExample("uq auth login --gh-only", "gh만"),
		output.HelpExample("uq auth login --aws-only", "aws만"),
		output.HelpExample("uq auth login --gcloud-only", "gcloud만"),
		output.HelpExample("uq auth login --skip-gcloud", "gh + aws (gcloud 제외)"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "login",
		Short: "gh + aws + gcloud 로그인 (전체 또는 선택)",
		Long:  long,
		RunE: func(cmd *cobra.Command, args []string) error {
			providers, err := selectProviders(cmd)
			if err != nil {
				return err
			}

			var failed []string
			for _, name := range providers {
				fmt.Fprintf(os.Stderr, "\n=== %s ===\n", name)
				var lerr error
				switch name {
				case "gh":
					lerr = authpkg.GhLogin(cmd.Context())
				case "aws":
					lerr = authpkg.AwsLogin(cmd.Context())
				case "gcloud":
					lerr = authpkg.GcloudLogin(cmd.Context())
				}
				if lerr != nil {
					fmt.Fprintf(os.Stderr, "%s: %v\n", name, lerr)
					failed = append(failed, name)
				}
			}

			fmt.Fprintln(os.Stderr)
			if len(failed) > 0 {
				// 실패 목록은 이미 stderr 에 찍혔다. cobra 의 "Error: ..." 중복을
				// 막고 런타임 에러만 반환한다(exit code 는 main 의 Classify=1).
				fmt.Fprintf(os.Stderr, "실패한 provider: %v\n", failed)
				cmd.SilenceErrors = true
				return clierr.PreconditionError{Msg: fmt.Sprintf("로그인 실패: %v", failed)}
			}
			fmt.Fprintln(os.Stderr, "모든 provider 로그인 완료")
			return nil
		},
	}
	addProviderFlags(cmd)
	return cmd
}
