// Package install implements the `uq install <team>` command.
package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	authpkg "github.com/un7qi3inc/un7qi3-cli/internal/auth"
	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

const ghOrg = "un7qi3inc"

type repoEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsArchived  bool   `json:"isArchived"`
}

// NewCmd returns the `uq install` command.
func NewCmd() *cobra.Command {
	var (
		all      bool
		selected []string
		list     bool
	)
	long := strings.Join([]string{
		output.Desc("팀(GitHub topic)별 레포를 다중 선택해 ~/un7qi3/<name>에 일괄 클론합니다."),
		"",
		output.Desc("토픽이 부착된 레포를 ") + output.Cyan("gh repo list --topic <team>") + output.Desc(" 로 조회합니다."),
		output.Desc("이미 존재하는 디렉토리는 건너뜁니다 (덮어쓰지 않음)."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq install backend", "TUI 다중 선택"),
		output.HelpExample("uq install backend --all", "후보 전부"),
		output.HelpExample("uq install backend --select forceteller-api,astro-api", "쉼표 구분"),
		output.HelpExample("uq install backend --list --json", "후보 목록만"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "install <team>",
		Short: "팀 레포 일괄 클론 (TUI 다중 선택)",
		Long:  long,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			team := args[0]
			jsonOut, _ := cmd.Flags().GetBool("json")

			if s := authpkg.GhStatus(); !s.OK {
				return &authpkg.RequiredError{
					Msg: "gh 인증 안 됨. `uq auth login --gh-only` 실행",
				}
			}

			repos, err := listTeamRepos(team)
			if err != nil {
				return err
			}

			if list {
				return printList(cmd, repos, jsonOut)
			}

			if len(repos) == 0 {
				fmt.Fprintf(cmd.OutOrStderr(),
					"%q 토픽이 붙은 레포가 없습니다.\n토픽 추가 방법:\n  %s\n",
					team,
					output.Cyan(fmt.Sprintf("gh repo edit %s/<repo> --add-topic %s", ghOrg, team)),
				)
				return nil
			}

			picks, err := choosePicks(repos, all, selected)
			if err != nil {
				return err
			}
			if len(picks) == 0 {
				fmt.Fprintln(cmd.OutOrStderr(), "선택된 레포가 없습니다.")
				return nil
			}

			return cloneAll(cmd, picks)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "후보 레포 전부 선택 (비대화형)")
	cmd.Flags().StringSliceVar(&selected, "select", nil, "쉼표로 구분된 레포 이름 (비대화형)")
	cmd.Flags().BoolVar(&list, "list", false, "후보 레포 목록만 표시하고 종료")
	cmd.MarkFlagsMutuallyExclusive("all", "select")
	return cmd
}

func listTeamRepos(team string) ([]repoEntry, error) {
	out, err := uqexec.Run("gh", "repo", "list", ghOrg,
		"--topic", team,
		"--json", "name,description,isArchived",
		"--limit", "200",
	)
	if err != nil {
		return nil, fmt.Errorf("gh repo list --topic %s 실패: %w", team, err)
	}
	var repos []repoEntry
	if err := json.Unmarshal(out, &repos); err != nil {
		return nil, fmt.Errorf("gh 응답 파싱 실패: %w", err)
	}
	active := make([]repoEntry, 0, len(repos))
	for _, r := range repos {
		if r.IsArchived {
			continue
		}
		active = append(active, r)
	}
	return active, nil
}

func printList(cmd *cobra.Command, repos []repoEntry, jsonOut bool) error {
	if jsonOut {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(repos)
	}
	w := cmd.OutOrStdout()
	for _, r := range repos {
		fmt.Fprintf(w, "  %s  %s\n", output.Cyan(r.Name), output.Dim(r.Description))
	}
	return nil
}

func choosePicks(repos []repoEntry, all bool, selected []string) ([]repoEntry, error) {
	if all {
		return repos, nil
	}
	if len(selected) > 0 {
		byName := make(map[string]repoEntry, len(repos))
		for _, r := range repos {
			byName[r.Name] = r
		}
		picks := make([]repoEntry, 0, len(selected))
		var unknown []string
		for _, name := range selected {
			r, ok := byName[name]
			if !ok {
				unknown = append(unknown, name)
				continue
			}
			picks = append(picks, r)
		}
		if len(unknown) > 0 {
			return nil, fmt.Errorf("알 수 없는 레포: %s", strings.Join(unknown, ", "))
		}
		return picks, nil
	}
	return pickTUI(repos)
}

func pickTUI(repos []repoEntry) ([]repoEntry, error) {
	options := make([]huh.Option[string], 0, len(repos))
	for _, r := range repos {
		label := r.Name
		if r.Description != "" {
			label = fmt.Sprintf("%s — %s", r.Name, r.Description)
		}
		options = append(options, huh.NewOption(label, r.Name))
	}
	var chosen []string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("클론할 레포를 선택하세요 (space=토글, enter=확정)").
				Options(options...).
				Value(&chosen),
		),
	)
	if err := form.Run(); err != nil {
		return nil, err
	}
	byName := make(map[string]repoEntry, len(repos))
	for _, r := range repos {
		byName[r.Name] = r
	}
	picks := make([]repoEntry, 0, len(chosen))
	for _, name := range chosen {
		if r, ok := byName[name]; ok {
			picks = append(picks, r)
		}
	}
	return picks, nil
}

func cloneAll(cmd *cobra.Command, picks []repoEntry) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("홈 디렉토리 확인 실패: %w", err)
	}
	parent := filepath.Join(home, "un7qi3")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("부모 디렉토리 생성 실패: %w", err)
	}

	var cloned, skipped, failed []string
	w := cmd.OutOrStderr()
	for _, r := range picks {
		dir := filepath.Join(parent, r.Name)
		if _, err := os.Stat(dir); err == nil {
			fmt.Fprintf(w, "%s %s  %s\n", output.Yellow("⊘"), r.Name, output.Dim("이미 존재 — 건너뜀"))
			skipped = append(skipped, r.Name)
			continue
		}
		fmt.Fprintf(w, "%s %s  %s\n", output.Cyan("↓"), r.Name, output.Dim("클론 중..."))
		ref := fmt.Sprintf("%s/%s", ghOrg, r.Name)
		if err := uqexec.RunInteractive("gh", "repo", "clone", ref, dir); err != nil {
			fmt.Fprintf(w, "%s %s  %s\n", output.Red("✗"), r.Name, output.Dim(err.Error()))
			failed = append(failed, r.Name)
			continue
		}
		cloned = append(cloned, r.Name)
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s 클론 %d개  스킵 %d개  실패 %d개\n",
		output.Bold("요약:"), len(cloned), len(skipped), len(failed))
	if len(failed) > 0 {
		return fmt.Errorf("일부 레포 클론 실패: %s", strings.Join(failed, ", "))
	}
	return nil
}
