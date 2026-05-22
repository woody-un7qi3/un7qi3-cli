package auth

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	authpkg "github.com/un7qi3inc/un7qi3-cli/internal/auth"
	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

func newStatusCmd() *cobra.Command {
	long := strings.Join([]string{
		output.Desc("gh / aws / gcloud 세 provider의 인증 상태를 점검합니다."),
		"",
		output.Yellow("--json") + output.Desc(" 으로 머신 친화 출력. 하나라도 미인증이면 exit 4."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq auth status", "셋 다"),
		output.HelpExample("uq auth status --gh-only", "gh만"),
		output.HelpExample("uq auth status --skip-gcloud", "gcloud 제외"),
		output.HelpExample("uq auth status --json | jq '.summary.ok'", "통과 개수 추출"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "status",
		Short: "gh + aws + gcloud 인증 상태 (전체 또는 선택)",
		Long:  long,
		RunE: func(cmd *cobra.Command, args []string) error {
			providers, err := selectProviders(cmd)
			if err != nil {
				return err
			}
			jsonOut, _ := cmd.Flags().GetBool("json")

			report := authpkg.Report{Providers: make([]authpkg.Status, 0, len(providers))}
			for _, name := range providers {
				s := authpkg.StatusOf(name)
				report.Providers = append(report.Providers, s)
				if s.OK {
					report.Summary.OK++
				} else {
					report.Summary.Failed++
				}
			}

			if jsonOut {
				if werr := output.WriteJSON(cmd.OutOrStdout(), report); werr != nil {
					return werr
				}
			} else {
				printStatusHuman(cmd, report)
			}

			if report.Summary.Failed > 0 {
				return &authpkg.RequiredError{
					Msg: fmt.Sprintf("%d개 provider 인증 실패. `uq auth login` 실행 권장.", report.Summary.Failed),
				}
			}
			return nil
		},
	}
	addProviderFlags(cmd)
	return cmd
}

func printStatusHuman(cmd *cobra.Command, r authpkg.Report) {
	w := cmd.OutOrStdout()
	for _, s := range r.Providers {
		glyph := output.GlyphOK
		right := ""
		if s.OK {
			switch s.Name {
			case "gh":
				right = fmt.Sprintf("%s 으로 인증됨", s.User)
			case "aws":
				right = s.Account
				if s.Arn != "" {
					right = fmt.Sprintf("%s (%s)", s.Account, s.Arn)
				}
			case "gcloud":
				right = fmt.Sprintf("%s (active)", s.Account)
			default:
				right = s.Detail
			}
		} else {
			glyph = output.GlyphFail
			right = s.Error
			if right == "" {
				right = "인증되지 않음"
			}
		}
		fmt.Fprintf(w, "%s %-10s %s\n", glyph, s.Name, right)
	}
	fmt.Fprintln(w)
	total := r.Summary.OK + r.Summary.Failed
	if r.Summary.Failed == 0 {
		fmt.Fprintf(w, "모든 provider 인증됨. (%d/%d)\n", r.Summary.OK, total)
	}
}
