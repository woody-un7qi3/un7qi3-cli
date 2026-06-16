package repo

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	authpkg "github.com/un7qi3inc/un7qi3-cli/internal/auth"
	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

const ghOrg = "un7qi3inc"

type ghRepo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"`
	UpdatedAt   string `json:"updatedAt"`
	IsArchived  bool   `json:"isArchived"`
}

// fetchOrgRepos는 gh repo list 로 조직 레포 전체를 조회한다.
// topic이 비어 있지 않으면 --topic 으로 후보를 좁힌다.
// archived 필터링은 호출자가 담당한다.
func fetchOrgRepos(limit int, topic string) ([]ghRepo, error) {
	ghArgs := []string{"repo", "list", ghOrg,
		"--json", "name,description,visibility,updatedAt,isArchived",
		"--limit", strconv.Itoa(limit),
	}
	if topic != "" {
		ghArgs = append(ghArgs, "--topic", topic)
	}
	out, err := uqexec.Run("gh", ghArgs...)
	if err != nil {
		return nil, err
	}
	var repos []ghRepo
	if err := json.Unmarshal(out, &repos); err != nil {
		return nil, fmt.Errorf("gh 응답 파싱 실패: %w", err)
	}
	return repos, nil
}

func newListCmd() *cobra.Command {
	long := strings.Join([]string{
		output.Desc("un7qi3inc 조직의 레포 목록을 표시합니다."),
		"",
		output.Desc("내부적으로 ") + output.Cyan("gh repo list") + output.Desc(" 를 호출합니다."),
		output.Yellow("--json") + output.Desc(" 으로 머신 친화 출력. gh 인증 필요 (없으면 exit 4)."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq repo list", "활성 레포 전체"),
		output.HelpExample("uq repo list --limit 10", "최근 10개"),
		output.HelpExample("uq repo list --archived", "archived만"),
		output.HelpExample("uq repo list --json | jq 'length'", "개수만"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "list",
		Short: "un7qi3inc 조직 레포 목록",
		Long:  long,
		RunE: func(cmd *cobra.Command, args []string) error {
			limit, _ := cmd.Flags().GetInt("limit")
			archivedOnly, _ := cmd.Flags().GetBool("archived")
			noArchived, _ := cmd.Flags().GetBool("no-archived")
			jsonOut, _ := cmd.Flags().GetBool("json")

			if archivedOnly && noArchived {
				return fmt.Errorf("--archived 와 --no-archived 는 동시에 사용할 수 없습니다")
			}

			// gh 인증 사전 확인 — 안 되어 있으면 즉시 exit 4.
			if s := authpkg.GhStatus(); !s.OK {
				return &authpkg.RequiredError{
					Msg: "gh 인증 안 됨. `uq auth login --gh-only` 실행",
				}
			}

			repos, err := fetchOrgRepos(limit, "")
			if err != nil {
				return err
			}

			// 기본 동작: archived 제외. --archived 단독 지정 시 archived만.
			filtered := make([]ghRepo, 0, len(repos))
			for _, r := range repos {
				switch {
				case archivedOnly && !r.IsArchived:
					continue
				case !archivedOnly && r.IsArchived:
					// noArchived는 기본 동작이므로 둘 다 동일
					continue
				}
				filtered = append(filtered, r)
			}

			if jsonOut {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(filtered)
			}

			printReposHuman(cmd, filtered)
			return nil
		},
	}
	cmd.Flags().Int("limit", 100, "최대 표시 개수")
	cmd.Flags().Bool("archived", false, "archived 레포만 표시")
	cmd.Flags().Bool("no-archived", true, "archived 레포 제외 (기본)")
	return cmd
}

func printReposHuman(cmd *cobra.Command, repos []ghRepo) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVISIBILITY\tUPDATED\tDESCRIPTION")
	for _, r := range repos {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Name, r.Visibility, relativeTime(r.UpdatedAt), r.Description)
	}
	_ = w.Flush()
}

// relativeTime renders an RFC3339 timestamp as "3d ago" / "2mo ago" / "1y ago"
// without bringing in a dependency. Falls back to YYYY-MM-DD on parse failure.
func relativeTime(rfc3339 string) string {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		if len(rfc3339) >= 10 {
			return rfc3339[:10]
		}
		return rfc3339
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "방금"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/24/30))
	default:
		return fmt.Sprintf("%dy ago", int(d.Hours()/24/365))
	}
}
