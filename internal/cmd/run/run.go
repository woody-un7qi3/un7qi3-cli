// Package run implements `uq run <repo>[:profile]`.
//
// It launches one or more local development commands inside a repo's clone,
// with the Node runtime and environment variables that the repos.yml run
// profile demands. The user's currently-checked-out git branch is never
// changed — `uq run` is for development on whatever branch you happen to
// be on.
package run

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	osexec "os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/un7qi3inc/un7qi3-cli/internal/clierr"
	"github.com/un7qi3inc/un7qi3-cli/internal/config"
	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
	"github.com/un7qi3inc/un7qi3-cli/internal/output"
	"github.com/un7qi3inc/un7qi3-cli/internal/repocfg"
	"github.com/un7qi3inc/un7qi3-cli/internal/run"
)

// NewCmd returns `uq run`.
func NewCmd() *cobra.Command {
	var (
		dryRun       bool
		bgFlag       bool
		fgFlag       bool
		splitFlag    bool
		splitDirFlag string
		countryFlag  string
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
		output.HelpExample("uq run profiles", "등록된 프로파일 나열 (사람용)"),
		output.HelpExample("uq run profiles --json", "에이전트/자동화용 머신 출력"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "run <repo>[:profile]",
		Short: "레포의 로컬 개발 서버 실행",
		Long:  long,
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			repoName, profileName, err := splitTarget(args[0])
			if err != nil {
				return err
			}

			cfg, err := repocfg.Load()
			if err != nil {
				return err
			}
			profile, resolvedName, err := cfg.ProfileFor(repoName, profileName)
			if err != nil {
				return err
			}

			reposDir, err := config.ReposDir()
			if err != nil {
				return err
			}
			dir := filepath.Join(reposDir, repoName)
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

			isTTY := term.IsTerminal(int(os.Stdin.Fd()))
			label := repoName + ":" + resolvedName
			country, err := resolveCountry(profile, dir, countryFlag, label, isTTY, c.OutOrStderr())
			if err != nil {
				// 국가 결정/사전 검증 실패는 사용법 에러(2)가 아닌 런타임 에러(1).
				// 메시지는 형식 보존을 위해 직접 찍고, cobra 의 "Error: " 중복을
				// 막은 뒤 런타임 에러를 반환한다(exit code 는 main 의 Classify=1).
				fmt.Fprintln(c.OutOrStderr(), output.Red("✗"), err)
				c.SilenceErrors = true
				return clierr.PreconditionError{Msg: err.Error()}
			}
			if country != nil {
				profile = profile.SubstituteScript(country.Script)
			}

			// --split-dir 값은 분할을 쓸 때만 의미 있지만, 잘못된 값은 미리 막는다.
			if splitFlag {
				if _, _, err := parseSplitDir(splitDirFlag); err != nil {
					return err
				}
			}

			// 실행 전 포트 충돌 점검 — 선언된 localhost 포트가 이미 LISTEN 중이면
			// 띄우기 전에 멈춘다. dry-run 은 상태만 보여주고 멈추지 않는다.
			ports := declaredPorts(profile, repoName)
			portStatuses := run.InspectPorts(portNumbers(ports))

			if dryRun {
				printDryRun(c.OutOrStdout(), repoName, resolvedName, dir, branch, profile, pathAdded, nodeRes, country)
				printPortStatus(c.OutOrStdout(), ports, portStatuses)
				if splitFlag && len(profile.Procs) > 1 {
					printSplitPlan(c.OutOrStdout(), dir, profile, profile.Env, pathAdded, splitDirFlag)
				}
				return nil
			}

			if msg, conflict := portConflictMsg(ports, portStatuses); conflict {
				fmt.Fprint(c.OutOrStderr(), msg)
				c.SilenceErrors = true
				return clierr.PreconditionError{Msg: "포트 충돌"}
			}

			mode, err := chooseMode(fgFlag, bgFlag, splitFlag, isTTY, procNames(profile.Procs))
			if err != nil {
				return err
			}

			printHeader(c.OutOrStderr(), repoName, resolvedName, branch, nodeRes, profile, country)
			switch mode {
			case modeBackground:
				return runBackground(c.OutOrStderr(), repoName, dir, profile, env)
			case modeSplit:
				if len(profile.Procs) > 1 {
					// 방향 결정/프롬프트는 runSplit 안에서 터미널 종류에 맞춰 처리.
					return runSplit(c.OutOrStderr(), repoName, dir, profile, profile.Env, pathAdded, env, splitDirFlag, isTTY)
				}
				// 단일 프로세스는 분할 대상이 없다 — 일반 포그라운드로.
			}
			var runErr error
			if len(profile.Procs) > 0 {
				runErr = execMulti(c.Context(), dir, profile.Procs, env)
			} else {
				runErr = execSingle(c.Context(), dir, profile.Cmd, env)
			}
			// 자식 프로세스가 비정상 종료하면 그 종료 코드를 그대로 전달한다.
			// 자식이 이미 자기 출력을 냈으므로 cobra 의 "Error: ..." 는 덧붙이지 않는다.
			var coded clierr.ExitCodeError
			if errors.As(runErr, &coded) {
				c.SilenceErrors = true
			}
			return runErr
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "실제 실행 없이 풀어진 cmd/env/PATH 만 출력")
	cmd.Flags().BoolVar(&bgFlag, "bg", false, "백그라운드 실행 (로그 파일로 분리, 즉시 반환)")
	cmd.Flags().BoolVar(&fgFlag, "fg", false, "포그라운드 실행 (현재 터미널에 로그 출력)")
	cmd.Flags().BoolVar(&splitFlag, "split", false, "포그라운드 패널 분할 (proc별 별도 패널, 멀티프로세스 전용)")
	cmd.Flags().StringVar(&splitDirFlag, "split-dir", "", "분할 방향: col(좌우, 기본) | row(상하) — 생략 시 TTY 에서 물음")
	cmd.Flags().StringVar(&countryFlag, "country", "", "국가 선택 (예: kr/en/jp — countries 가 선언된 프로파일에서만)")
	cmd.MarkFlagsMutuallyExclusive("bg", "fg")
	cmd.MarkFlagsMutuallyExclusive("bg", "split")
	cmd.AddCommand(newProfilesCmd())
	return cmd
}

type runMode int

const (
	modeForeground runMode = iota
	modeBackground
	modeSplit
)

// chooseMode decides how to launch.
//
//   - --split / --bg / --fg 명시:  그대로
//   - 비TTY:                       포그라운드 merged (스크립트/CI 안전)
//   - TTY, 멀티프로세스:           merged / split / background 3지선다
//   - TTY, 단일프로세스:           foreground / background 2지선다
//
// procNames are the proc names of the profile; more than one enables the split
// option (split only makes sense when there are multiple procs to spread
// across panels) and is shown in the merged-log label so the user knows which
// processes share the screen.
func chooseMode(fg, bg, split, isTTY bool, procNames []string) (runMode, error) {
	if split {
		return modeSplit, nil
	}
	if bg {
		return modeBackground, nil
	}
	if fg {
		return modeForeground, nil
	}
	if !isTTY {
		return modeForeground, nil
	}

	// 모든 선택지를 "포그라운드/백그라운드 · 설명" 한 형식으로 통일한다.
	// jargon(proc) 없이, Ctrl+C 안내는 메뉴 설명에서 한 번만.
	var choice string
	if len(procNames) > 1 {
		mergedLabel := fmt.Sprintf("포그라운드 · 한 화면에 로그 합쳐 보기 (%s)", strings.Join(procNames, " + "))
		// 분할 옵션은 이 터미널이 실제로 할 수 있는 방식(패널/새 창)으로 문구를
		// 바꾸고, 분할 자체가 불가하면 메뉴에서 아예 뺀다.
		opts := []huh.Option[string]{huh.NewOption(mergedLabel, "fg")}
		if styleOf(run.DetectMultiplexer()) != styleNone {
			opts = append(opts, huh.NewOption(splitMenuLabel(run.DetectMultiplexer()), "split"))
		}
		opts = append(opts, huh.NewOption("백그라운드 · 로그는 파일로 남기고 바로 복귀", "bg"))
		if err := huh.NewSelect[string]().
			Title("어떻게 실행할까요?").
			Description("포그라운드는 Ctrl+C 로 종료합니다.").
			Options(opts...).
			Value(&choice).
			Run(); err != nil {
			return 0, err
		}
	} else {
		err := huh.NewSelect[string]().
			Title("어떻게 실행할까요?").
			Description("포그라운드는 Ctrl+C 로 종료합니다.").
			Options(
				huh.NewOption("포그라운드 · 이 터미널에 로그 출력", "fg"),
				huh.NewOption("백그라운드 · 로그는 파일로 남기고 바로 복귀", "bg"),
			).
			Value(&choice).
			Run()
		if err != nil {
			return 0, err
		}
	}
	switch choice {
	case "bg":
		return modeBackground, nil
	case "split":
		return modeSplit, nil
	default:
		return modeForeground, nil
	}
}

// portCheck pairs a declared port with the proc (or repo) that owns it, for
// labeling the pre-flight output.
type portCheck struct {
	port  int
	label string
}

// declaredPorts collects the localhost ports a profile declares via its URLs
// (the single-cmd URL, or each proc's URL). Non-localhost URLs are skipped —
// uq can't meaningfully pre-flight a remote port.
func declaredPorts(p repocfg.Profile, repo string) []portCheck {
	var out []portCheck
	add := func(rawURL, label string) {
		if port, ok := localhostPort(rawURL); ok {
			out = append(out, portCheck{port: port, label: label})
		}
	}
	if len(p.Procs) > 0 {
		for _, pr := range p.Procs {
			add(pr.URL, pr.Name)
		}
	} else {
		add(p.URL, repo)
	}
	return out
}

// localhostPort extracts the port from a localhost/127.0.0.1 URL.
func localhostPort(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return 0, false
	}
	if h := u.Hostname(); h != "localhost" && h != "127.0.0.1" {
		return 0, false
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return 0, false
	}
	return port, true
}

func portNumbers(checks []portCheck) []int {
	nums := make([]int, len(checks))
	for i, c := range checks {
		nums[i] = c.port
	}
	return nums
}

// printPortStatus renders the dry-run port section: each declared port with
// whether it's free or already taken (and by what).
func printPortStatus(w io.Writer, checks []portCheck, statuses []run.PortStatus) {
	if len(checks) == 0 {
		return
	}
	fmt.Fprintf(w, "  포트 점검:\n")
	for i, st := range statuses {
		state := output.Green("비어 있음")
		if st.InUse {
			state = output.Red("사용 중 — " + portOwnerDesc(st))
		}
		fmt.Fprintf(w, "    %s %d  %s\n", output.Cyan("["+checks[i].label+"]"), checks[i].port, state)
	}
}

// portOwnerDesc describes who holds a port: the full launched command line when
// ps could read it, else the short command name, with the pid.
func portOwnerDesc(st run.PortStatus) string {
	who := st.Path
	if who == "" {
		who = st.Command
	}
	if who == "" {
		return "점유 프로세스 미상"
	}
	var meta []string
	if st.PID > 0 {
		meta = append(meta, fmt.Sprintf("pid %d", st.PID))
	}
	if st.Cwd != "" {
		meta = append(meta, "cwd: "+st.Cwd)
	}
	if len(meta) > 0 {
		return who + " (" + strings.Join(meta, ", ") + ")"
	}
	return who
}

// portConflictMsg builds the abort message when any declared port is taken,
// returning (message, true). When all ports are free it returns ("", false).
func portConflictMsg(checks []portCheck, statuses []run.PortStatus) (string, bool) {
	var b strings.Builder
	var pids []string
	any := false
	for i, st := range statuses {
		if !st.InUse {
			continue
		}
		any = true
		fmt.Fprintf(&b, "  %s %d 사용 중 — %s\n", output.Cyan("["+checks[i].label+"]"), checks[i].port, portOwnerDesc(st))
		if st.PID > 0 {
			pids = append(pids, strconv.Itoa(st.PID))
		}
	}
	if !any {
		return "", false
	}
	header := output.Red("✗") + " 포트 충돌 — 실행을 중단합니다\n"
	hint := ""
	if len(pids) > 0 {
		hint = output.Dim("  비우려면: ") + output.Cyan("kill "+strings.Join(pids, " ")) + "\n"
	}
	return header + b.String() + hint, true
}

// procNames returns the proc names in declaration order.
func procNames(procs []repocfg.Proc) []string {
	names := make([]string, len(procs))
	for i, p := range procs {
		names[i] = p.Name
	}
	return names
}

// splitTarget parses "repo" or "repo:profile" into its parts.
//
// 정상 입력은 기존과 동일하게 갈린다: "repo" → ("repo", ""), "repo:profile" →
// ("repo", "profile"). 콜론을 썼는데 한쪽이 빈 모호한 입력(":", "repo:",
// ":profile")은 조용히 빈 값을 흘려보내 ProfileFor 단계에서 엉뚱한 메시지로
// 이어지므로, 여기서 clierr.InvalidArgError(usage, exit 2)로 명확히 막는다.
func splitTarget(s string) (repo, profile string, err error) {
	i := strings.IndexByte(s, ':')
	if i < 0 {
		return s, "", nil
	}
	repo, profile = s[:i], s[i+1:]
	if repo == "" {
		return "", "", clierr.InvalidArgError{Msg: fmt.Sprintf("repo 가 비어 있습니다: %q — `repo` 또는 `repo:profile` 형식으로 지정하세요", s)}
	}
	if profile == "" {
		return "", "", clierr.InvalidArgError{Msg: fmt.Sprintf("프로파일이 비어 있습니다: %q — 콜론 뒤에 프로파일명을 적거나, 기본 프로파일을 쓰려면 `%s` 로 실행하세요", s, repo)}
	}
	return repo, profile, nil
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

func printHeader(w io.Writer, repo, profile, branch string, node *run.NodeResolution, p repocfg.Profile, country *repocfg.Country) {
	nodeInfo := ""
	if node != nil {
		nodeInfo = fmt.Sprintf(", node=%s(%s)", node.Version, node.Source)
	}
	profileLabel := profile
	if country != nil {
		profileLabel = fmt.Sprintf("%s(%s)", profile, country.Code)
	}
	fmt.Fprintf(w, "%s %s %s\n",
		output.Cyan("▶"),
		output.Bold(repo),
		output.Dim(fmt.Sprintf("(%s) — profile=%s%s", branch, profileLabel, nodeInfo)),
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

func printDryRun(w io.Writer, repo, profile, dir, branch string, p repocfg.Profile, pathAdded string, node *run.NodeResolution, country *repocfg.Country) {
	fmt.Fprintf(w, "%s %s:%s\n", output.Bold("dry-run"), repo, profile)
	fmt.Fprintf(w, "  실행 디렉토리: %s\n", dir)
	fmt.Fprintf(w, "  현재 브랜치:   %s %s\n", branch, output.Dim("(건드리지 않음)"))
	if country != nil {
		verified := "검증 통과"
		if len(country.Requires) > 0 {
			verified = "검증 통과: " + joinComma(country.Requires)
		}
		fmt.Fprintf(w, "  국가:          %s  %s\n", country.Code, output.Dim("("+verified+")"))
	}
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
// current TTY. When ctx is cancelled (Ctrl+C/SIGTERM, wired into ctx by main's
// signal.NotifyContext) the interrupt is forwarded to the child's process group
// so dev servers can shut down cleanly; execSingle then keeps waiting for the
// child to exit so its real exit code is still surfaced. A non-zero child exit
// is propagated as a clierr.ExitCodeError so main can mirror the child's exact
// exit code while the RunE-level defers still run.
func execSingle(ctx context.Context, dir string, cmd []string, env []string) error {
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

	done := make(chan error, 1)
	go func() { done <- c.Wait() }()

	select {
	case <-ctx.Done():
		// 취소가 들어오면 자식 프로세스 그룹에 SIGINT 를 보내 깨끗이 종료시킨다.
		// 그 뒤에도 done 을 계속 기다려 자식의 실제 종료 코드를 회수한다.
		if c.Process != nil {
			_ = syscall.Kill(-c.Process.Pid, syscall.SIGINT)
		}
		return waitChild(<-done)
	case err := <-done:
		return waitChild(err)
	}
}

// waitChild 는 자식의 Wait 결과를 RunE 계약에 맞는 에러로 변환한다.
func waitChild(err error) error {
	if err == nil {
		return nil
	}
	var exitErr *osexec.ExitError
	if errors.As(err, &exitErr) {
		return clierr.ExitCodeError{Code: exitErr.ExitCode()}
	}
	return err
}

// execMulti runs every Proc concurrently with output prefixed as "[name] ".
//
// Each proc gets its own process group so SIGINT can be forwarded cleanly.
// When ctx is cancelled (Ctrl+C/SIGTERM via main's signal.NotifyContext) every
// child group is sent SIGINT so the dev servers tear down instead of leaking.
// If any proc exits non-zero before all are done, the rest are sent SIGTERM
// — a partial set of dev servers is rarely useful, and leaving them running
// hides the failure. Stdin is not connected (multi-proc + stdin is rarely
// what you want).
func execMulti(ctx context.Context, repoDir string, procs []repocfg.Proc, env []string) error {
	// Single mutex shared across all prefix writers prevents two procs from
	// interleaving each other's lines on the TTY.
	var ttyMu sync.Mutex
	colors := []func(string) string{output.Cyan, output.Yellow, output.Green, output.Blue}

	// pids 는 Start 직후 한 번만 기록한다. *exec.Cmd 의 ProcessState 는 Wait
	// goroutine 이 동시 기록하므로, 신호 전파 시 Cmd 를 읽지 않고 이 PID 스냅샷만
	// 써서 데이터 레이스를 피한다(자식 그룹 전체에 -pid 로 시그널).
	pids := make([]int, 0, len(procs))
	done := make(chan procResult, len(procs))

	killGroups := func(sig syscall.Signal) {
		for _, pid := range pids {
			_ = syscall.Kill(-pid, sig)
		}
	}

	for i, p := range procs {
		dir := repoDir
		if p.Cwd != "" {
			dir = filepath.Join(repoDir, p.Cwd)
		}
		if _, err := os.Stat(dir); err != nil {
			killGroups(syscall.SIGTERM)
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
			killGroups(syscall.SIGTERM)
			return fmt.Errorf("proc %s 시작 실패: %w", p.Name, err)
		}
		pids = append(pids, c.Process.Pid)
		name := p.Name
		go func(cmd *osexec.Cmd) { done <- procResult{name: name, err: cmd.Wait()} }(c)
	}

	var firstFailure error
	remaining := len(pids)
	// cancelled 가 닫히면 ctx.Done() 은 계속 ready 상태라 select 가 바쁜 루프에
	// 빠진다. 한 번 처리한 뒤 nil 로 바꿔 그 case 를 영구 비활성화한다.
	cancelled := ctx.Done()
	for remaining > 0 {
		select {
		case <-cancelled:
			// 취소 시 모든 자식 그룹에 SIGINT 를 보내 정리하고, 이후로는 자식들의
			// Wait 완료(done)만 기다려 깨끗이 빠져나간다.
			killGroups(syscall.SIGINT)
			cancelled = nil
		case r := <-done:
			remaining--
			if r.err != nil && firstFailure == nil {
				firstFailure = fmt.Errorf("%s: %w", r.name, r.err)
				// Bring the rest down so the user sees the failure immediately.
				killGroups(syscall.SIGTERM)
			}
		}
	}
	return firstFailure
}

type procResult struct {
	name string
	err  error
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
