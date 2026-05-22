package auth

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	authpkg "github.com/un7qi3inc/un7qi3-cli/internal/auth"
	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

func newLogoutCmd() *cobra.Command {
	long := strings.Join([]string{
		output.Desc("세 provider(gh / aws / gcloud)에서 로그아웃합니다."),
		"",
		output.Desc("한 provider가 실패해도 나머지는 계속 시도하며, 하나라도 실패하면 exit 1."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq auth logout", "셋 다"),
		output.HelpExample("uq auth logout --gh-only", "gh만"),
		output.HelpExample("uq auth logout --skip-aws", "aws 제외"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "gh + aws + gcloud 로그아웃 (전체 또는 선택)",
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
					lerr = authpkg.GhLogout()
				case "aws":
					lerr = authpkg.AwsLogout()
				case "gcloud":
					lerr = authpkg.GcloudLogout()
				}
				if lerr != nil {
					fmt.Fprintf(os.Stderr, "%s: %v\n", name, lerr)
					failed = append(failed, name)
				}
			}

			fmt.Fprintln(os.Stderr)
			if len(failed) > 0 {
				fmt.Fprintf(os.Stderr, "실패한 provider: %v\n", failed)
				os.Exit(1)
			}
			fmt.Fprintln(os.Stderr, "모든 provider 로그아웃 완료")
			return nil
		},
	}
	addProviderFlags(cmd)
	return cmd
}
