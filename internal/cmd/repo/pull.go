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
	dirtyReset dirtyChoice = "reset"
	dirtySkip  dirtyChoice = "skip"
	dirtyAbort dirtyChoice = "abort"
)

func newPullCmd() *cobra.Command {
	var (
		branchOverride []string
		currentOnly    bool
		reset          bool
		yes            bool
	)
	long := strings.Join([]string{
		output.Desc("로컬 클론된 레포의 설정된 브랜치를 최신으로 동기화합니다."),
		"",
		output.Desc("브랜치 목록은 ") + output.Cyan("internal/repocfg/repos.yml") + output.Desc(" 에서 관리합니다."),
		output.Desc("매핑이 없는 레포는 defaults(") + output.Yellow("[main]") + output.Desc(")가 적용됩니다."),
		output.Desc("워킹 트리가 더티면 stash/skip/abort 중 선택할 수 있습니다."),
		"",
		output.Desc(output.Yellow("--reset")+" 은 각 브랜치를 원격 상태로 강제 동기화합니다 (파괴적):"),
		output.Desc("  · ") + output.Cyan("git reset --hard <remote>/<branch>") + output.Desc(" — 로컬 커밋 + 추적 변경 모두 버림"),
		output.Desc("  · ") + output.Cyan("git clean -fd") + output.Desc(" — untracked 파일/디렉토리 삭제 (gitignored는 보존)"),
		output.Desc("  · 실행 전 확인 prompt. ") + output.Yellow("--yes") + output.Desc(" 로 스킵 (스크립트/CI용)"),
		"",
		output.Heading("예시"),
		output.HelpExample("uq repo pull forceteller-api", "설정된 브랜치 전부 (develop, master)"),
		output.HelpExample("uq repo pull forceteller-api --current", "현재 브랜치만"),
		output.HelpExample("uq repo pull forceteller-api --branches main", "설정 무시"),
		output.HelpExample("uq repo pull forceteller-api --reset", "원격 상태로 강제 동기화 (확인)"),
		output.HelpExample("uq repo pull forceteller-api --reset --yes", "확인 없이 즉시"),
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

			if err := pullBranches(cmd, dir, name, branches, reset, yes); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&branchOverride, "branches", nil, "설정 무시하고 지정한 브랜치만")
	cmd.Flags().BoolVar(&currentOnly, "current", false, "현재 체크아웃된 브랜치만")
	cmd.Flags().BoolVar(&reset, "reset", false, "각 브랜치를 원격 상태로 강제 동기화 (파괴적). 로컬 커밋/변경/untracked 모두 삭제. 확인 prompt 있음.")
	cmd.Flags().BoolVar(&yes, "yes", false, "확인 prompt 스킵 (--reset 비대화형용)")
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

// chooseRemote returns "upstream" if that remote exists (fork workflow),
// otherwise "origin". This mirrors the convention where personal forks
// live at origin and the canonical org repo lives at upstream.
func chooseRemote(dir string) string {
	out, err := uqexec.RunIn(dir, "git", "remote")
	if err != nil {
		return "origin"
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == "upstream" {
			return "upstream"
		}
	}
	return "origin"
}

// lastCommitLine returns a one-line summary of ref's tip:
// "<short-hash> <subject> (<author>, <relative-time>)". Empty string on error.
func lastCommitLine(dir, ref string) string {
	out, err := uqexec.RunIn(dir, "git", "log", "-1",
		"--no-decorate",
		"--pretty=format:%h %s (%an, %ar)",
		ref,
	)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// branchSHA returns the short commit hash that ref points to.
func branchSHA(dir, ref string) (string, error) {
	out, err := uqexec.RunIn(dir, "git", "rev-parse", "--short", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// printBranchResult renders one branch's pull result in compact form.
//   - up-to-date  →  "  ✓ <branch>  최신 — <hash> <subj> (<author>, <when>)"
//   - new commits →  "  ✓ <branch>  N개 새 커밋  before..after"
//                    followed by oneline log entries indented further.
func printBranchResult(w interface{ Write(p []byte) (int, error) }, dir, branch, before, after string) {
	const maxLog = 5
	if before == "" || after == "" || before == after {
		tip := lastCommitLine(dir, branch)
		if tip == "" {
			fmt.Fprintf(w, "  %s %s  %s\n", output.Green("✓"), branch, output.Dim("최신"))
		} else {
			fmt.Fprintf(w, "  %s %s  %s\n", output.Green("✓"), branch, output.Dim("최신 — "+tip))
		}
		return
	}
	out, err := uqexec.RunIn(dir, "git", "log",
		"--no-decorate",
		"--pretty=format:%h %s (%an, %ar)",
		fmt.Sprintf("%s..%s", before, after),
	)
	if err != nil {
		fmt.Fprintf(w, "  %s %s  %s\n", output.Yellow("⚠"), branch, output.Dim(fmt.Sprintf("log 조회 실패: %v", err)))
		return
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	total := len(lines)
	fmt.Fprintf(w, "  %s %s  %s\n",
		output.Green("✓"), branch,
		output.Dim(fmt.Sprintf("%d개 새 커밋  %s..%s", total, before, after)),
	)
	shown := lines
	if total > maxLog {
		shown = lines[:maxLog]
	}
	for _, line := range shown {
		fmt.Fprintf(w, "    %s\n", output.Dim(line))
	}
	if total > maxLog {
		fmt.Fprintf(w, "    %s\n", output.Dim(fmt.Sprintf("...외 %d개 더", total-maxLog)))
	}
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

// dirtyCount returns the number of changed entries reported by
// `git status --porcelain`. Zero means clean.
func dirtyCount(dir string) (int, error) {
	out, err := uqexec.RunIn(dir, "git", "status", "--porcelain")
	if err != nil {
		return 0, err
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return 0, nil
	}
	return len(strings.Split(trimmed, "\n")), nil
}

// resolveDirtyChoice picks the action to take based on flags + dirty state.
//   - --reset 명시: reset (TTY에서는 한 번 더 확인, --yes 면 스킵)
//   - clean working tree: stash (no-op 의미 — 그냥 pull 진행)
//   - dirty + TTY: askDirty prompt
//   - dirty + non-TTY: skip (실수 방지)
func resolveDirtyChoice(w interface{ Write(p []byte) (int, error) }, name string, dirtyN int, reset, yes bool) (dirtyChoice, error) {
	if reset {
		if !yes {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				fmt.Fprintf(w, "%s --reset 은 확인 prompt가 필요합니다. 비대화형 환경에서는 --yes 추가\n",
					output.Red("✗"))
				return dirtyAbort, fmt.Errorf("--reset --yes 필요")
			}
			ok, err := confirmReset(name, dirtyN)
			if err != nil {
				return "", err
			}
			if !ok {
				return dirtyAbort, nil
			}
		}
		return dirtyReset, nil
	}
	if dirtyN == 0 {
		return dirtyStash, nil // no-op, just proceed to pull
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintf(w, "  %s %s\n",
			output.Yellow("⚠"),
			output.Dim(fmt.Sprintf("워킹 트리 더티 (%d개) — 비대화형이라 건너뜁니다", dirtyN)))
		return dirtySkip, nil
	}
	return askDirty(name, dirtyN)
}

func confirmReset(name string, dirtyN int) (bool, error) {
	var ok bool
	title := fmt.Sprintf("%s를 원격 상태로 강제 동기화합니다", name)
	desc := "로컬 커밋과 추적 변경 + untracked 파일 모두 삭제됩니다. gitignored는 보존."
	if dirtyN > 0 {
		desc = fmt.Sprintf("로컬 변경 %d개 + 로컬 커밋 모두 삭제됩니다. gitignored는 보존.", dirtyN)
	}
	if err := huh.NewConfirm().
		Title(title).
		Description(desc).
		Affirmative("진행").
		Negative("취소").
		Value(&ok).
		Run(); err != nil {
		return false, err
	}
	return ok, nil
}

func askDirty(name string, dirtyCount int) (dirtyChoice, error) {
	var choice string
	title := fmt.Sprintf("%s: 커밋되지 않은 변경 %d개", name, dirtyCount)
	err := huh.NewSelect[string]().
		Title(title).
		Options(
			huh.NewOption("stash 후 pull, 끝나면 pop (변경 보존)", string(dirtyStash)),
			huh.NewOption("reset — 변경/로컬 커밋 모두 버리고 원격 상태로", string(dirtyReset)),
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

func pullBranches(cmd *cobra.Command, dir, name string, branches []string, reset, yes bool) error {
	w := cmd.OutOrStderr()

	original, err := currentBranch(dir)
	if err != nil {
		return err
	}

	remote := chooseRemote(dir)
	fmt.Fprintf(w, "%s %s\n", name, output.Dim(fmt.Sprintf("← %s", remote)))

	dirtyN, err := dirtyCount(dir)
	if err != nil {
		return err
	}

	choice, err := resolveDirtyChoice(w, name, dirtyN, reset, yes)
	if err != nil {
		return err
	}
	switch choice {
	case dirtyAbort:
		fmt.Fprintln(w, "중단됨.")
		return nil
	case dirtySkip:
		fmt.Fprintf(w, "  %s 건너뜀.\n", output.Dim("(uq)"))
		return nil
	}

	// fetch 한 번.
	if _, err := uqexec.RunIn(dir, "git", "fetch", remote); err != nil {
		return fmt.Errorf("git fetch %s 실패: %w", remote, err)
	}

	stashed := false
	if choice == dirtyStash && dirtyN > 0 {
		if _, err := uqexec.RunIn(dir, "git", "stash", "push", "-u", "-m", "uq repo pull"); err != nil {
			return fmt.Errorf("git stash 실패: %w", err)
		}
		stashed = true
		fmt.Fprintf(w, "  %s\n", output.Dim(fmt.Sprintf("(uq) 변경 %d개 stash됨 — 풀 끝나면 자동 복원", dirtyN)))
	}
	if choice == dirtyReset && dirtyN > 0 {
		// untracked + tracked 모두 제거. gitignored는 보존.
		if _, err := uqexec.RunIn(dir, "git", "reset", "--hard", "HEAD"); err != nil {
			return fmt.Errorf("git reset --hard 실패: %w", err)
		}
		if _, err := uqexec.RunIn(dir, "git", "clean", "-fd"); err != nil {
			return fmt.Errorf("git clean -fd 실패: %w", err)
		}
		fmt.Fprintf(w, "  %s\n", output.Dim(fmt.Sprintf("(uq) 로컬 변경 %d개 삭제됨 (reset --hard + clean -fd)", dirtyN)))
	}

	var succeeded, failed []string
	for _, br := range branches {
		if _, err := uqexec.RunIn(dir, "git", "checkout", br); err != nil {
			fmt.Fprintf(w, "  %s %s  %s\n", output.Red("✗"), br, output.Dim(fmt.Sprintf("checkout 실패: %v", err)))
			failed = append(failed, br)
			continue
		}
		before, _ := branchSHA(dir, br)
		if choice == dirtyReset {
			remoteRef := fmt.Sprintf("%s/%s", remote, br)
			if _, err := uqexec.RunIn(dir, "git", "reset", "--hard", remoteRef); err != nil {
				fmt.Fprintf(w, "  %s %s  %s\n", output.Red("✗"), br, output.Dim(fmt.Sprintf("reset --hard %s 실패: %v", remoteRef, err)))
				failed = append(failed, br)
				continue
			}
		} else {
			if _, err := uqexec.RunIn(dir, "git", "pull", "--ff-only", remote, br); err != nil {
				fmt.Fprintf(w, "  %s %s  %s\n", output.Red("✗"), br, output.Dim("pull --ff-only 실패 (diverged?) — --reset 으로 강제"))
				failed = append(failed, br)
				continue
			}
		}
		after, _ := branchSHA(dir, br)
		printBranchResult(w, dir, br, before, after)
		succeeded = append(succeeded, br)
	}

	// 원래 브랜치로 복귀.
	if _, err := uqexec.RunIn(dir, "git", "checkout", original); err != nil {
		fmt.Fprintf(w, "%s 원래 브랜치(%s)로 복귀 실패: %v\n", output.Red("✗"), original, err)
	}

	if stashed {
		if _, err := uqexec.RunIn(dir, "git", "stash", "pop"); err != nil {
			fmt.Fprintf(w, "  %s stash pop 실패: %v\n  %s\n",
				output.Red("✗"), err,
				output.Dim("`git stash list` 로 직접 복원하세요"))
		} else {
			fmt.Fprintf(w, "  %s\n", output.Dim("(uq) stash 복원 완료"))
		}
	}

	if len(failed) > 0 {
		fmt.Fprintf(w, "\n%s 성공 %d  실패 %d (%s)\n",
			output.Bold("요약:"),
			len(succeeded), len(failed), strings.Join(failed, ", "),
		)
		return fmt.Errorf("일부 브랜치 풀 실패")
	}
	return nil
}
