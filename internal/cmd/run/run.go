// Package run implements `uq run <repo>[:profile]`.
//
// It launches one or more local development commands inside a repo's clone,
// with the Node runtime and environment variables that the repos.yml run
// profile demands. The user's currently-checked-out git branch is never
// changed — `uq run` is for development on whatever branch you happen to
// be on.
package run

import (
	"errors"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
	"github.com/un7qi3inc/un7qi3-cli/internal/output"
	"github.com/un7qi3inc/un7qi3-cli/internal/repocfg"
	"github.com/un7qi3inc/un7qi3-cli/internal/run"
)

// NewCmd returns `uq run`.
func NewCmd() *cobra.Command {
	var (
		dryRun bool
		bgFlag bool
		fgFlag bool
	)
	long := strings.Join([]string{
		output.Desc("레포의 로컬 개발 서버를 통일된 환경에서 실행합니다."),
		"",
		output.Desc("실행 환경(노드 버전, env, 명령)은 ") + output.Cyan("internal/repocfg/repos.yml") + output.Desc(" 의 ") + output.Yellow("runs:") + output.Desc(" 블록에서 관리합니다."),
		output.Desc("현재 체크아웃된 git 브랜치는 ") + output.Bold("건드리지 않습니다") + output.Desc(" — release/x.y, feature/... 어디서든 그대로 실행."),
		"",
		output.Desc("노드 버전은 머신에 설치된 매니저를 자동 탐색합니다: fnm → nvm → mise → asdf → PATH."),
		output.Desc("멀티프로세스 프로파일(예: forceteller-admin)은 ") + output.Cyan("[name]") + output.Desc(" prefix 로 로그를 합쳐 띄웁니다."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq run forceteller-app", "default 프로파일"),
		output.HelpExample("uq run forceteller-app:app3", "프로파일 명시"),
		output.HelpExample("uq run forceteller-admin", "back + front 동시 실행"),
		output.HelpExample("uq run forceteller-app --dry-run", "실제 실행 없이 풀어진 cmd/env 확인"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "run <repo>[:profile]",
		Short: "레포의 로컬 개발 서버 실행",
		Long:  long,
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			repoName, profileName := splitTarget(args[0])

			cfg, err := repocfg.Load()
			if err != nil {
				return err
			}
			profile, resolvedName, err := cfg.ProfileFor(repoName, profileName)
			if err != nil {
				return err
			}

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("홈 디렉토리 확인 실패: %w", err)
			}
			dir := filepath.Join(home, "un7qi3", repoName)
			if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
				return fmt.Errorf("레포가 없습니다: %s\n  먼저 `uq repo clone %s` 실행", dir, repoName)
			}

			var nodeRes *run.NodeResolution
			if profile.Node != "" {
				r, err := run.ResolveNode(profile.Node)
				if err != nil {
					return err
				}
				nodeRes = &r
			}

			env, pathAdded := mergeEnv(profile.Env, nodeRes)
			branch := currentBranch(dir)

			if dryRun {
				printDryRun(c.OutOrStdout(), repoName, resolvedName, dir, branch, profile, pathAdded, nodeRes)
				return nil
			}

			mode, err := chooseMode(fgFlag, bgFlag)
			if err != nil {
				return err
			}

			printHeader(c.OutOrStderr(), repoName, resolvedName, branch, nodeRes, profile)
			if mode == modeBackground {
				return runBackground(c.OutOrStderr(), repoName, dir, profile, env)
			}
			if len(profile.Procs) > 0 {
				return execMulti(dir, profile.Procs, env)
			}
			return execSingle(dir, profile.Cmd, env)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "실제 실행 없이 풀어진 cmd/env/PATH 만 출력")
	cmd.Flags().BoolVar(&bgFlag, "bg", false, "백그라운드 실행 (로그 파일로 분리, 즉시 반환)")
	cmd.Flags().BoolVar(&fgFlag, "fg", false, "포그라운드 실행 (현재 터미널에 로그 출력)")
	cmd.MarkFlagsMutuallyExclusive("bg", "fg")
	return cmd
}

type runMode int

const (
	modeForeground runMode = iota
	modeBackground
)

// chooseMode decides foreground vs. background.
//
//   - --fg / --bg 명시:  그대로
//   - 비TTY:             포그라운드 (스크립트/CI 안전)
//   - TTY:               huh.NewSelect 로 물음
func chooseMode(fg, bg bool) (runMode, error) {
	if fg {
		return modeForeground, nil
	}
	if bg {
		return modeBackground, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return modeForeground, nil
	}
	var choice string
	err := huh.NewSelect[string]().
		Title("어떻게 실행할까요?").
		Description("Ctrl+C 로 즉시 종료할 수 있는 포그라운드 / 로그 파일로 분리되는 백그라운드").
		Options(
			huh.NewOption("포그라운드 — 이 터미널에 로그 출력, Ctrl+C 로 종료", "fg"),
			huh.NewOption("백그라운드 — 로그 파일로 분리, 즉시 복귀", "bg"),
		).
		Value(&choice).
		Run()
	if err != nil {
		return 0, err
	}
	if choice == "bg" {
		return modeBackground, nil
	}
	return modeForeground, nil
}

// splitTarget parses "repo" or "repo:profile" into its parts.
func splitTarget(s string) (repo, profile string) {
	if i := strings.IndexByte(s, ':'); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

// currentBranch returns the working tree's branch, or "(detached)" / "(unknown)".
func currentBranch(dir string) string {
	out, err := uqexec.RunIn(dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "(unknown)"
	}
	b := strings.TrimSpace(string(out))
	if b == "HEAD" {
		return "(detached)"
	}
	return b
}

// mergeEnv builds the child process environment.
//
// Order of precedence (later wins):
//  1. os.Environ() — inherited
//  2. profile.Env  — declared in repos.yml
//  3. PATH         — node bin dir prepended, if a node runtime was resolved
//
// Returns the env slice plus the PATH-prepended directory (empty if none) so
// the caller can show it in the header / dry-run output.
func mergeEnv(profileEnv map[string]string, node *run.NodeResolution) ([]string, string) {
	merged := map[string]string{}
	for _, kv := range os.Environ() {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			merged[kv[:i]] = kv[i+1:]
		}
	}
	for k, v := range profileEnv {
		merged[k] = v
	}
	pathAdded := ""
	if node != nil && node.BinDir != "" {
		pathAdded = node.BinDir
		merged["PATH"] = node.BinDir + string(os.PathListSeparator) + merged["PATH"]
	}
	out := make([]string, 0, len(merged))
	for k, v := range merged {
		out = append(out, k+"="+v)
	}
	return out, pathAdded
}

func printHeader(w io.Writer, repo, profile, branch string, node *run.NodeResolution, p repocfg.Profile) {
	nodeInfo := ""
	if node != nil {
		nodeInfo = fmt.Sprintf(", node=%s(%s)", node.Version, node.Source)
	}
	fmt.Fprintf(w, "%s %s %s\n",
		output.Cyan("▶"),
		output.Bold(repo),
		output.Dim(fmt.Sprintf("(%s) — profile=%s%s", branch, profile, nodeInfo)),
	)
	printURLs(w, p)
}

// printURLs renders the "준비되면 접속" hints. Compile/serve readiness is not
// auto-detected — these are pure declarations from repos.yml. The "준비되면"
// wording stays honest about that.
func printURLs(w io.Writer, p repocfg.Profile) {
	if p.URL != "" {
		fmt.Fprintf(w, "  %s %s\n", output.Dim("준비되면 접속:"), output.Cyan(p.URL))
		return
	}
	any := false
	for _, pr := range p.Procs {
		if pr.URL != "" {
			any = true
			break
		}
	}
	if !any {
		return
	}
	fmt.Fprintf(w, "  %s\n", output.Dim("준비되면 접속:"))
	for _, pr := range p.Procs {
		if pr.URL == "" {
			continue
		}
		fmt.Fprintf(w, "    %s %s\n", output.Cyan("["+pr.Name+"]"), output.Cyan(pr.URL))
	}
}

func printDryRun(w io.Writer, repo, profile, dir, branch string, p repocfg.Profile, pathAdded string, node *run.NodeResolution) {
	fmt.Fprintf(w, "%s %s:%s\n", output.Bold("dry-run"), repo, profile)
	fmt.Fprintf(w, "  실행 디렉토리: %s\n", dir)
	fmt.Fprintf(w, "  현재 브랜치:   %s %s\n", branch, output.Dim("(건드리지 않음)"))
	if node != nil {
		fmt.Fprintf(w, "  Node:          %s  %s\n", node.Version, output.Dim(fmt.Sprintf("(%s @ %s)", node.Source, node.BinDir)))
	}
	if pathAdded != "" {
		fmt.Fprintf(w, "  PATH 추가:     %s\n", pathAdded)
	}
	if len(p.Env) > 0 {
		fmt.Fprintf(w, "  환경 변수:\n")
		for k, v := range p.Env {
			fmt.Fprintf(w, "    %s=%s\n", k, v)
		}
	}
	if len(p.Procs) > 0 {
		fmt.Fprintf(w, "  프로세스:\n")
		for _, pr := range p.Procs {
			cwd := pr.Cwd
			if cwd == "" {
				cwd = "."
			}
			line := fmt.Sprintf("    %s  cwd=%s  cmd=%s",
				output.Cyan("["+pr.Name+"]"), cwd, strings.Join(pr.Cmd, " "))
			if pr.URL != "" {
				line += "  " + output.Dim("→ "+pr.URL)
			}
			fmt.Fprintln(w, line)
		}
	} else {
		fmt.Fprintf(w, "  명령:          %s\n", strings.Join(p.Cmd, " "))
		if p.URL != "" {
			fmt.Fprintf(w, "  URL:           %s\n", p.URL)
		}
	}
}

// execSingle runs cmd[0] cmd[1:] in dir with the given env, sharing the
// current TTY. SIGINT (Ctrl+C) is forwarded to the child's process group so
// dev servers can shut down cleanly. Non-zero exit codes are propagated via
// os.Exit.
func execSingle(dir string, cmd []string, env []string) error {
	c := osexec.Command(cmd[0], cmd[1:]...)
	c.Dir = dir
	c.Env = env
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := c.Start(); err != nil {
		return fmt.Errorf("%s 실행 실패: %w", cmd[0], err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()

	for {
		select {
		case sig := <-sigCh:
			if c.Process != nil {
				_ = syscall.Kill(-c.Process.Pid, sig.(syscall.Signal))
			}
		case err := <-done:
			if err == nil {
				return nil
			}
			var exitErr *osexec.ExitError
			if errors.As(err, &exitErr) {
				os.Exit(exitErr.ExitCode())
			}
			return err
		}
	}
}

// execMulti runs every Proc concurrently with output prefixed as "[name] ".
//
// Each proc gets its own process group so SIGINT can be forwarded cleanly.
// If any proc exits non-zero before all are done, the rest are sent SIGTERM
// — a partial set of dev servers is rarely useful, and leaving them running
// hides the failure. Stdin is not connected (multi-proc + stdin is rarely
// what you want).
func execMulti(repoDir string, procs []repocfg.Proc, env []string) error {
	// Single mutex shared across all prefix writers prevents two procs from
	// interleaving each other's lines on the TTY.
	var ttyMu sync.Mutex
	colors := []func(string) string{output.Cyan, output.Yellow, output.Green, output.Blue}

	children := make([]*osexec.Cmd, 0, len(procs))
	done := make(chan procResult, len(procs))

	for i, p := range procs {
		dir := repoDir
		if p.Cwd != "" {
			dir = filepath.Join(repoDir, p.Cwd)
		}
		if _, err := os.Stat(dir); err != nil {
			killAll(children)
			return fmt.Errorf("proc %s 의 cwd 가 없습니다: %s", p.Name, dir)
		}
		colorize := colors[i%len(colors)]
		prefix := colorize("["+p.Name+"]") + " "

		c := osexec.Command(p.Cmd[0], p.Cmd[1:]...)
		c.Dir = dir
		c.Env = env
		c.Stdout = newPrefixWriter(os.Stdout, prefix, &ttyMu)
		c.Stderr = newPrefixWriter(os.Stderr, prefix, &ttyMu)
		c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		if err := c.Start(); err != nil {
			killAll(children)
			return fmt.Errorf("proc %s 시작 실패: %w", p.Name, err)
		}
		children = append(children, c)
		name := p.Name
		go func(cmd *osexec.Cmd) { done <- procResult{name: name, err: cmd.Wait()} }(c)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	var firstFailure error
	remaining := len(children)
	for remaining > 0 {
		select {
		case sig := <-sigCh:
			s, _ := sig.(syscall.Signal)
			for _, c := range children {
				if c.Process != nil {
					_ = syscall.Kill(-c.Process.Pid, s)
				}
			}
		case r := <-done:
			remaining--
			if r.err != nil && firstFailure == nil {
				firstFailure = fmt.Errorf("%s: %w", r.name, r.err)
				// Bring the rest down so the user sees the failure immediately.
				for _, c := range children {
					if c.ProcessState == nil && c.Process != nil {
						_ = syscall.Kill(-c.Process.Pid, syscall.SIGTERM)
					}
				}
			}
		}
	}
	return firstFailure
}

type procResult struct {
	name string
	err  error
}

func killAll(children []*osexec.Cmd) {
	for _, c := range children {
		if c.Process != nil {
			_ = syscall.Kill(-c.Process.Pid, syscall.SIGTERM)
		}
	}
}

// prefixWriter wraps an io.Writer and inserts a prefix at the start of every
// line. The mutex is shared between sibling writers (stdout + stderr for all
// procs) so concurrent writes can't tear lines into each other.
type prefixWriter struct {
	mu     *sync.Mutex
	w      io.Writer
	prefix string
	atBOL  bool
}

func newPrefixWriter(w io.Writer, prefix string, mu *sync.Mutex) *prefixWriter {
	return &prefixWriter{mu: mu, w: w, prefix: prefix, atBOL: true}
}

func (p *prefixWriter) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]byte, 0, len(b)+len(p.prefix))
	for _, c := range b {
		if p.atBOL {
			out = append(out, p.prefix...)
			p.atBOL = false
		}
		out = append(out, c)
		if c == '\n' {
			p.atBOL = true
		}
	}
	if _, err := p.w.Write(out); err != nil {
		return 0, err
	}
	return len(b), nil
}
