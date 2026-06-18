package repo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	authpkg "github.com/un7qi3inc/un7qi3-cli/internal/auth"
	initcmd "github.com/un7qi3inc/un7qi3-cli/internal/cmd/initcmd"
	"github.com/un7qi3inc/un7qi3-cli/internal/config"
	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

func newCloneCmd() *cobra.Command {
	var (
		dir  string
		team string
		all  bool
		list bool
	)
	long := strings.Join([]string{
		output.Desc("un7qi3inc/<name> 레포를 ~/un7qi3/<name>에 클론합니다."),
		"",
		output.Desc("레포명을 주면 직접 클론하고, 생략하면 후보를 TUI로 다중 선택합니다."),
		output.Yellow("--team") + output.Desc(" 으로 팀(GitHub topic) 단위로 후보를 좁힐 수 있습니다."),
		output.Desc("이미 존재하는 디렉토리는 건너뜁니다 (덮어쓰지 않음). gh 인증이 없으면 exit 4."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq repo clone forceteller-api", "단건 클론"),
		output.HelpExample("uq repo clone astro-api forceteller-api", "여러 개 클론"),
		output.HelpExample("uq repo clone forceteller-api --dir /tmp/work", "위치 지정"),
		output.HelpExample("uq repo clone", "조직 전체에서 TUI 다중 선택"),
		output.HelpExample("uq repo clone --team backend", "팀 레포로 좁혀 TUI 선택"),
		output.HelpExample("uq repo clone --team backend --all", "팀 레포 전부"),
		output.HelpExample("uq repo clone --all", "조직 전체 전부 (비대화형)"),
		output.HelpExample("uq repo clone --list", "후보 목록만"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "clone [name ...]",
		Short: "레포 클론 (인자 없으면 TUI 다중 선택)",
		Long:  long,
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOut, _ := cmd.Flags().GetBool("json")

			if len(args) > 0 {
				if team != "" {
					return fmt.Errorf("--team 과 레포명 인자는 함께 쓸 수 없습니다")
				}
				if all || list {
					return fmt.Errorf("--all / --list 는 레포명 인자와 함께 쓸 수 없습니다")
				}
				if dir != "" && len(args) != 1 {
					return fmt.Errorf("--dir 는 레포명 하나를 클론할 때만 사용할 수 있습니다")
				}
			} else if dir != "" {
				return fmt.Errorf("--dir 는 레포명 하나를 클론할 때만 사용할 수 있습니다")
			}

			// gh 인증 사전 확인 — 안 되어 있으면 즉시 exit 4.
			// 워크스페이스 설정 프롬프트보다 먼저 막아 미인증 시 불필요한
			// 디렉토리 설정을 거치지 않게 한다.
			if s := authpkg.GhStatus(); !s.OK {
				return &authpkg.RequiredError{
					Msg: "gh 인증 안 됨. `uq auth login --gh-only` 실행",
				}
			}

			// 최초 실행이면 워크스페이스 위치를 먼저 정한다 (--dir 오버라이드 시 생략).
			if dir == "" {
				if err := initcmd.EnsureReposDir(cmd.OutOrStdout()); err != nil {
					return err
				}
			}

			if len(args) > 0 {
				return runNamedClone(cmd, args, dir)
			}
			return runBulkClone(cmd, team, all, list, jsonOut)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "클론 위치 오버라이드 (기본 $HOME/un7qi3/<name>, 단건만)")
	cmd.Flags().StringVar(&team, "team", "", "팀(GitHub topic)으로 후보를 좁힘")
	cmd.Flags().BoolVar(&all, "all", false, "후보 레포 전부 선택 (비대화형)")
	cmd.Flags().BoolVar(&list, "list", false, "후보 레포 목록만 표시하고 종료")
	cmd.MarkFlagsMutuallyExclusive("all", "list")
	cmd.Flags().Bool("json", false, "JSON 형식으로 출력")
	return cmd
}

// runNamedClone는 명시한 레포명들을 <reposDir>/<name>(또는 --dir)에 클론한다.
func runNamedClone(cmd *cobra.Command, names []string, dir string) error {
	reposDir, err := config.ReposDir()
	if err != nil {
		return err
	}
	targets := make([]string, len(names))
	for i, name := range names {
		if dir != "" {
			targets[i] = dir
			continue
		}
		targets[i] = filepath.Join(reposDir, name)
	}
	return cloneInto(cmd, names, targets)
}

// runBulkClone는 조직 전체(team="") 또는 topic으로 좁힌 후보를
// TUI 다중 선택(또는 --all/--list)한 뒤 일괄 클론한다.
func runBulkClone(cmd *cobra.Command, team string, all, list, jsonOut bool) error {
	candidates, err := fetchOrgRepos(200, team)
	if err != nil {
		return err
	}
	repos := make([]ghRepo, 0, len(candidates))
	for _, r := range candidates {
		if r.IsArchived {
			continue
		}
		repos = append(repos, r)
	}

	if list {
		return printCandidates(cmd, repos, jsonOut)
	}
	if len(repos) == 0 {
		if team != "" {
			fmt.Fprintf(cmd.OutOrStderr(),
				"%q 토픽이 붙은 레포가 없습니다.\n토픽 추가 방법:\n  %s\n",
				team,
				output.Cyan(fmt.Sprintf("gh repo edit %s/<repo> --add-topic %s", ghOrg, team)),
			)
		} else {
			fmt.Fprintln(cmd.OutOrStderr(), "클론할 레포가 없습니다.")
		}
		return nil
	}

	var picks []ghRepo
	if all {
		picks = repos
	} else {
		picks, err = pickTUI(repos)
		if err != nil {
			return err
		}
	}
	if len(picks) == 0 {
		fmt.Fprintln(cmd.OutOrStderr(), "선택된 레포가 없습니다.")
		return nil
	}

	reposDir, err := config.ReposDir()
	if err != nil {
		return err
	}
	names := make([]string, len(picks))
	targets := make([]string, len(picks))
	for i, r := range picks {
		names[i] = r.Name
		targets[i] = filepath.Join(reposDir, r.Name)
	}
	return cloneInto(cmd, names, targets)
}

func printCandidates(cmd *cobra.Command, repos []ghRepo, jsonOut bool) error {
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

func pickTUI(repos []ghRepo) ([]ghRepo, error) {
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
	byName := make(map[string]ghRepo, len(repos))
	for _, r := range repos {
		byName[r.Name] = r
	}
	picks := make([]ghRepo, 0, len(chosen))
	for _, name := range chosen {
		if r, ok := byName[name]; ok {
			picks = append(picks, r)
		}
	}
	return picks, nil
}

// cloneInto는 names[i] 레포를 targets[i] 디렉토리에 클론한다.
// 이미 존재하는 디렉토리는 건너뛰고, 끝에 요약을 출력한다.
func cloneInto(cmd *cobra.Command, names, targets []string) error {
	var cloned, skipped, failed []string
	w := cmd.OutOrStderr()
	for i, name := range names {
		dir := targets[i]
		if _, err := os.Stat(dir); err == nil {
			fmt.Fprintf(w, "%s %s  %s\n", output.Yellow("⊘"), name, output.Dim("이미 존재 — 건너뜀"))
			skipped = append(skipped, name)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return fmt.Errorf("부모 디렉토리 생성 실패: %w", err)
		}
		fmt.Fprintf(w, "%s %s  %s\n", output.Cyan("↓"), name, output.Dim("클론 중..."))
		ref := fmt.Sprintf("%s/%s", ghOrg, name)
		if err := uqexec.RunInteractive("gh", "repo", "clone", ref, dir); err != nil {
			fmt.Fprintf(w, "%s %s  %s\n", output.Red("✗"), name, output.Dim(err.Error()))
			failed = append(failed, name)
			continue
		}
		cloned = append(cloned, name)
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "%s 클론 %d개  스킵 %d개  실패 %d개\n",
		output.Bold("요약:"), len(cloned), len(skipped), len(failed))
	if len(failed) > 0 {
		return fmt.Errorf("일부 레포 클론 실패: %s", strings.Join(failed, ", "))
	}
	return nil
}
