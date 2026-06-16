// Package initcmd implements `uq init` — first-run onboarding that checks
// essentials (gh auth) and resolves where org repos live.
package initcmd

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	authpkg "github.com/un7qi3inc/un7qi3-cli/internal/auth"
	"github.com/un7qi3inc/un7qi3-cli/internal/config"
	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

// ghOrg is the marker we look for in a repo's git remote to recognize it as
// one of ours during the workspace scan.
const ghOrg = "un7qi3inc"

const customChoice = "__custom__"

// initStep is one phase of `uq init`. To extend the flow, add an entry to the
// steps slice in NewCmd — the section header, numbering, and spacing render
// uniformly for every step.
type initStep struct {
	title string
	run   func(io.Writer) error
}

// NewCmd returns the `uq init` command.
func NewCmd() *cobra.Command {
	long := strings.Join([]string{
		output.Desc("uq 최초 설정을 도와줍니다."),
		"",
		output.Desc("1. 필수 인증(gh) 상태를 점검합니다."),
		output.Desc("2. 레포를 클론해 둘 워크스페이스 위치를 정합니다."),
		output.Desc("   기존 un7qi3 레포가 어디 있는지 자동으로 스캔해 후보로 제시합니다."),
		"",
		output.Desc("선택 결과는 ") + output.Cyan("~/.config/un7qi3/config.yml") + output.Desc(" 의 repos_dir 에 저장됩니다."),
		output.Desc("환경변수 ") + output.Yellow("UQ_REPOS_DIR") + output.Desc(" 가 있으면 그 값이 항상 우선합니다."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq init", "대화형 초기 설정"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "init",
		Short: "uq 최초 설정 (인증 점검 + 워크스페이스 위치)",
		Long:  long,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				return fmt.Errorf("uq init 은 대화형 명령입니다. 비대화형 환경에서는 UQ_REPOS_DIR 환경변수를 사용하세요")
			}
			w := cmd.OutOrStdout()

			steps := []initStep{
				{title: "인증", run: checkEssentials},
				{title: "워크스페이스", run: setupWorkspace},
			}
			for i, s := range steps {
				if i > 0 {
					fmt.Fprintln(w)
				}
				fmt.Fprintln(w, output.Section(fmt.Sprintf("%d. %s", i+1, s.title)))
				if err := s.run(w); err != nil {
					return err
				}
			}

			fmt.Fprintf(w, "\n%s 으로 un7qi3 레포를 받을 수 있어요.\n", output.Cyan("uq repo clone"))
			return nil
		},
	}
	return cmd
}

// EnsureReposDir runs the first-run workspace setup when the repos dir hasn't
// been configured yet. In a non-interactive shell it stays silent and lets the
// caller fall back to the default — so agents/CI are never blocked by a prompt.
func EnsureReposDir(w io.Writer) error {
	if config.IsReposDirConfigured() {
		return nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil
	}
	fmt.Fprintf(w, "%s\n", output.Dim("워크스페이스가 아직 설정되지 않았습니다 — 처음 한 번만 정할게요."))
	if err := setupWorkspace(w); err != nil {
		return err
	}
	fmt.Fprintln(w)
	return nil
}

// checkEssentials verifies gh auth (clone/pull의 전제) and offers to log in.
func checkEssentials(w io.Writer) error {
	s := authpkg.GhStatus()
	if s.OK {
		who := s.User
		if who == "" {
			who = "(unknown)"
		}
		fmt.Fprintf(w, "%s gh 인증됨: %s\n", output.Green("✓"), who)
		return nil
	}

	var login bool
	if err := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("gh 로그인이 필요합니다 — 지금 할까요?").
				Description("clone · pull 에 gh 인증이 필요합니다.\ngh auth login + setup-git 을 실행합니다.").
				Affirmative("로그인").
				Negative("나중에").
				Value(&login),
		),
	).WithTheme(huh.ThemeCharm()).Run(); err != nil {
		return err
	}
	if !login {
		fmt.Fprintf(w, "%s 나중에 %s 로 로그인하세요\n", output.Yellow("⚠"), output.Cyan("uq auth login"))
		return nil
	}
	if err := authpkg.GhLogin(); err != nil {
		return err
	}
	fmt.Fprintf(w, "%s gh 로그인 완료\n", output.Green("✓"))
	return nil
}

// setupWorkspace scans for existing org clones, then runs a single guided form
// (intro → pick → optional custom path → confirm) and saves the result.
func setupWorkspace(w io.Writer) error {
	defaultDir, err := defaultReposDir()
	if err != nil {
		return err
	}

	cands := scanWorkspaces()

	options := make([]huh.Option[string], 0, len(cands)+2)
	seen := map[string]bool{}
	for _, c := range cands {
		seen[c.dir] = true
		label := fmt.Sprintf("%s  (레포 %d개)", c.dir, c.count)
		options = append(options, huh.NewOption(label, c.dir))
	}
	if !seen[defaultDir] {
		options = append(options, huh.NewOption(defaultDir+"  (기본값)", defaultDir))
	}
	options = append(options, huh.NewOption("직접 입력...", customChoice))

	chosen := defaultDir
	if len(cands) > 0 {
		chosen = cands[0].dir
	}
	custom := defaultDir
	save := true

	// resolved reports the directory implied by the current form state.
	resolved := func() string {
		if chosen != customChoice {
			return chosen
		}
		if c := strings.TrimSpace(custom); c != "" {
			return c
		}
		return defaultDir
	}

	intro := "clone · pull · run 이 레포를 두고 찾을 위치입니다."
	if len(cands) > 0 {
		intro = fmt.Sprintf("clone · pull · run 의 기준 위치 — 기존 레포 %d곳을 자동 감지했어요.", len(cands))
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("레포를 어디에 두시겠어요?").
				Description(intro).
				Options(options...).
				Value(&chosen),
		),
		huh.NewGroup(
			huh.NewInput().
				Title("워크스페이스 경로 직접 입력").
				Description("~ 와 환경변수를 쓸 수 있습니다.").
				Placeholder(defaultDir).
				Value(&custom),
		).WithHideFunc(func() bool { return chosen != customChoice }),
		huh.NewGroup(
			huh.NewConfirm().
				Title("이 위치로 저장할까요?").
				DescriptionFunc(resolved, &chosen).
				Affirmative("저장").
				Negative("취소").
				Value(&save),
		),
	).WithTheme(huh.ThemeCharm())

	if err := form.Run(); err != nil {
		return err
	}
	if !save {
		fmt.Fprintf(w, "  %s 저장하지 않았습니다. 기본값(%s)을 사용합니다.\n",
			output.Yellow("⚠"), output.Cyan(defaultDir))
		return nil
	}

	if err := config.Save(&config.Config{ReposDir: resolved()}); err != nil {
		return err
	}
	path, _ := config.Path()
	final, _ := config.ReposDir()

	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s 워크스페이스: %s\n", output.Green("✓"), output.Cyan(final))
	fmt.Fprintf(w, "  %s %s 에 저장됨\n", output.Dim("·"), output.Dim(path))
	return nil
}

func defaultReposDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "un7qi3"), nil
}

// workspaceCand is one candidate workspace directory and how many org repos
// were found directly inside it.
type workspaceCand struct {
	dir   string
	count int
}

// scanWorkspaces walks common dev locations for git repos whose remote points
// at the org, and tallies their parent directories. Returns candidates sorted
// by repo count (desc), then path (asc).
func scanWorkspaces() []workspaceCand {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	roots := []string{
		home,
		filepath.Join(home, "work"),
		filepath.Join(home, "dev"),
		filepath.Join(home, "Developer"),
		filepath.Join(home, "projects"),
		filepath.Join(home, "src"),
		filepath.Join(home, "repos"),
		filepath.Join(home, "Documents"),
		filepath.Join(home, "go", "src"),
	}

	counts := map[string]int{}
	scanned := map[string]bool{}
	for _, root := range roots {
		if scanned[root] {
			continue
		}
		scanned[root] = true
		scanRoot(root, 4, counts)
	}

	cands := make([]workspaceCand, 0, len(counts))
	for dir, n := range counts {
		cands = append(cands, workspaceCand{dir: dir, count: n})
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].count != cands[j].count {
			return cands[i].count > cands[j].count
		}
		return cands[i].dir < cands[j].dir
	})
	return cands
}

// skipDirs are directory names we never descend into during the scan.
var skipDirs = map[string]bool{
	"node_modules": true,
	"Library":      true,
	"Caches":       true,
	".cache":       true,
	".Trash":       true,
	".git":         true,
	"vendor":       true,
	"dist":         true,
	"build":        true,
	".npm":         true,
	".cargo":       true,
}

// scanRoot walks root up to maxDepth levels deep. When it finds a git repo
// (a dir containing .git/config that references the org), it bumps the count
// for that repo's parent directory and stops descending into the repo.
func scanRoot(root string, maxDepth int, counts map[string]int) {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return
	}
	rootDepth := strings.Count(filepath.Clean(root), string(os.PathSeparator))
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if path != root && skipDirs[d.Name()] {
			return fs.SkipDir
		}
		if strings.Count(path, string(os.PathSeparator))-rootDepth > maxDepth {
			return fs.SkipDir
		}
		gitCfg := filepath.Join(path, ".git", "config")
		data, rerr := os.ReadFile(gitCfg)
		if rerr != nil {
			return nil
		}
		if bytes.Contains(data, []byte(ghOrg)) {
			counts[filepath.Dir(path)]++
		}
		// path is a repo — don't descend further into it.
		return fs.SkipDir
	})
}
