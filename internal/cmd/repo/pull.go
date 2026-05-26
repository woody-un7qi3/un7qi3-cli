package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
	"github.com/un7qi3inc/un7qi3-cli/internal/output"
	"github.com/un7qi3inc/un7qi3-cli/internal/repocfg"
)

// dirtyChoice is the user's resolution for a dirty working tree.
type dirtyChoice string

const (
	dirtyStash dirtyChoice = "stash"
	dirtySkip  dirtyChoice = "skip"
	dirtyAbort dirtyChoice = "abort"
)

func newPullCmd() *cobra.Command {
	var (
		branchOverride []string
		currentOnly    bool
	)
	long := strings.Join([]string{
		output.Desc("로컬 클론된 레포의 설정된 브랜치를 최신으로 동기화합니다."),
		"",
		output.Desc("브랜치 목록은 ") + output.Cyan("internal/repocfg/repos.yml") + output.Desc(" 에서 관리합니다."),
		output.Desc("매핑이 없는 레포는 defaults(") + output.Yellow("[main]") + output.Desc(")가 적용됩니다."),
		output.Desc("워킹 트리가 더티면 stash/skip/abort 중 선택할 수 있습니다."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq repo pull forceteller-api", "설정된 브랜치 전부 (develop, master)"),
		output.HelpExample("uq repo pull forceteller-api --current", "현재 브랜치만"),
		output.HelpExample("uq repo pull forceteller-api --branches main", "설정 무시"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "pull <name>",
		Short: "레포의 설정된 브랜치를 최신으로 풀",
		Long:  long,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			home, err := os.UserHomeDir()
			if err != nil {
				fmt.Fprintf(os.Stderr, "홈 디렉토리 확인 실패: %v\n", err)
				os.Exit(1)
			}
			dir := filepath.Join(home, "un7qi3", name)
			if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
				fmt.Fprintf(os.Stderr, "레포가 없습니다: %s\n  먼저 `uq repo clone %s` 또는 `uq install <team>` 실행\n", dir, name)
				os.Exit(1)
			}

			branches, err := resolveBranches(name, branchOverride, currentOnly, dir)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			if len(branches) == 0 {
				fmt.Fprintln(os.Stderr, "풀할 브랜치가 없습니다")
				os.Exit(1)
			}

			if err := pullBranches(cmd, dir, name, branches); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&branchOverride, "branches", nil, "설정 무시하고 지정한 브랜치만")
	cmd.Flags().BoolVar(&currentOnly, "current", false, "현재 체크아웃된 브랜치만")
	cmd.MarkFlagsMutuallyExclusive("branches", "current")
	return cmd
}

func resolveBranches(name string, override []string, currentOnly bool, dir string) ([]string, error) {
	if currentOnly {
		cur, err := currentBranch(dir)
		if err != nil {
			return nil, err
		}
		return []string{cur}, nil
	}
	if len(override) > 0 {
		return override, nil
	}
	cfg, err := repocfg.Load()
	if err != nil {
		return nil, err
	}
	return cfg.BranchesFor(name), nil
}

func currentBranch(dir string) (string, error) {
	out, err := uqexec.RunIn(dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", fmt.Errorf("현재 브랜치 확인 실패: %w", err)
	}
	br := strings.TrimSpace(string(out))
	if br == "HEAD" {
		return "", fmt.Errorf("detached HEAD 상태입니다. --branches 로 명시하세요")
	}
	return br, nil
}

func isDirty(dir string) (bool, error) {
	out, err := uqexec.RunIn(dir, "git", "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(out))) > 0, nil
}

func askDirty(name string) (dirtyChoice, error) {
	var choice string
	err := huh.NewSelect[string]().
		Title(fmt.Sprintf("%s: 커밋되지 않은 변경이 있습니다", name)).
		Options(
			huh.NewOption("stash 후 pull, 끝나면 pop", string(dirtyStash)),
			huh.NewOption("이 레포 건너뛰기", string(dirtySkip)),
			huh.NewOption("전체 중단", string(dirtyAbort)),
		).
		Value(&choice).
		Run()
	if err != nil {
		return "", err
	}
	return dirtyChoice(choice), nil
}

func pullBranches(cmd *cobra.Command, dir, name string, branches []string) error {
	w := cmd.OutOrStderr()

	original, err := currentBranch(dir)
	if err != nil {
		return err
	}

	stashed := false
	dirty, err := isDirty(dir)
	if err != nil {
		return err
	}
	if dirty {
		var choice dirtyChoice
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprintf(w, "%s %s 워킹 트리가 더티 — 비대화형 환경이라 건너뜁니다\n",
				output.Yellow("⚠"), name)
			return nil
		}
		var derr error
		choice, derr = askDirty(name)
		if derr != nil {
			return derr
		}
		switch choice {
		case dirtyAbort:
			fmt.Fprintln(w, "중단됨.")
			return nil
		case dirtySkip:
			fmt.Fprintf(w, "%s 건너뜀.\n", name)
			return nil
		case dirtyStash:
			if _, err := uqexec.RunIn(dir, "git", "stash", "push", "-u", "-m", "uq repo pull"); err != nil {
				return fmt.Errorf("git stash 실패: %w", err)
			}
			stashed = true
			fmt.Fprintf(w, "%s stash 완료\n", output.Dim("(uq)"))
		}
	}

	// 한 번 전체 fetch.
	if _, err := uqexec.RunIn(dir, "git", "fetch", "origin"); err != nil {
		return fmt.Errorf("git fetch 실패: %w", err)
	}

	var succeeded, failed []string
	for _, br := range branches {
		fmt.Fprintf(w, "%s %s\n", output.Cyan("→"), br)
		if _, err := uqexec.RunIn(dir, "git", "checkout", br); err != nil {
			fmt.Fprintf(w, "  %s checkout 실패: %v\n", output.Red("✗"), err)
			failed = append(failed, br)
			continue
		}
		if _, err := uqexec.RunIn(dir, "git", "pull", "--ff-only", "origin", br); err != nil {
			fmt.Fprintf(w, "  %s pull --ff-only 실패 (diverged?): %v\n", output.Red("✗"), err)
			failed = append(failed, br)
			continue
		}
		succeeded = append(succeeded, br)
	}

	// 원래 브랜치로 복귀.
	if _, err := uqexec.RunIn(dir, "git", "checkout", original); err != nil {
		fmt.Fprintf(w, "%s 원래 브랜치(%s)로 복귀 실패: %v\n", output.Red("✗"), original, err)
	}

	if stashed {
		if _, err := uqexec.RunIn(dir, "git", "stash", "pop"); err != nil {
			fmt.Fprintf(w, "%s stash pop 실패: %v\n  `git stash list` 로 직접 복원하세요\n", output.Red("✗"), err)
		} else {
			fmt.Fprintf(w, "%s stash 복원 완료\n", output.Dim("(uq)"))
		}
	}

	fmt.Fprintf(w, "\n%s 성공 %d (%s)  실패 %d (%s)\n",
		output.Bold("요약:"),
		len(succeeded), strings.Join(succeeded, ", "),
		len(failed), strings.Join(failed, ", "),
	)
	if len(failed) > 0 {
		return fmt.Errorf("일부 브랜치 풀 실패")
	}
	return nil
}
