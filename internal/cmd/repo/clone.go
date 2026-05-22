package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	authpkg "github.com/un7qi3inc/un7qi3-cli/internal/auth"
	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

func newCloneCmd() *cobra.Command {
	long := strings.Join([]string{
		output.Desc("un7qi3inc/<name> 레포를 클론합니다."),
		"",
		output.Desc("기본 클론 위치는 $HOME/un7qi3/<name> 이며 ") + output.Yellow("--dir") + output.Desc(" 로 오버라이드 가능합니다."),
		output.Desc("이미 디렉토리가 존재하면 exit 1. gh 인증이 없으면 exit 4."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq repo clone forceteller-api", "~/un7qi3/forceteller-api"),
		output.HelpExample("uq repo clone forceteller-api --dir /tmp/work", "지정 위치"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "clone <name>",
		Short: "레포를 ~/un7qi3/<name>으로 클론",
		Long:  long,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			dir, _ := cmd.Flags().GetString("dir")

			if dir == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("홈 디렉토리 확인 실패: %w", err)
				}
				dir = filepath.Join(home, "un7qi3", name)
			}

			if _, err := os.Stat(dir); err == nil {
				fmt.Fprintf(os.Stderr, "이미 존재함: %s\n", dir)
				os.Exit(1)
			}

			// gh 인증 사전 확인.
			if s := authpkg.GhStatus(); !s.OK {
				return &authpkg.RequiredError{
					Msg: "gh 인증 안 됨. `uq auth login --gh-only` 실행",
				}
			}

			// 부모 디렉토리는 미리 만들어 둔다 (gh clone 자체는 mkdir -p 안 함).
			if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
				return fmt.Errorf("부모 디렉토리 생성 실패: %w", err)
			}

			repoRef := fmt.Sprintf("%s/%s", ghOrg, name)
			if err := uqexec.RunInteractive("gh", "repo", "clone", repoRef, dir); err != nil {
				return fmt.Errorf("gh repo clone 실패: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "클론 완료: %s\n", dir)
			return nil
		},
	}
	cmd.Flags().String("dir", "", "클론 위치 오버라이드 (기본 $HOME/un7qi3/<name>)")
	return cmd
}
