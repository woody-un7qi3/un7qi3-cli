// Package run implements `uq run <repo>[:profile]`.
//
// It launches one or more local development commands inside a repo's clone,
// with the Node runtime and environment variables that the repos.yml run
// profile demands. The user's currently-checked-out git branch is never
// changed — `uq run` is for development on whatever branch you happen to
// be on.
package run

import (
	"bufio"
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
	eblogs "github.com/un7qi3inc/un7qi3-cli/internal/log"
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
		output.Desc("멀티프로세스 프로파일(예: forceteller-admin)은 ") + output.Cyan("uq log") + output.Desc(" 와 동일한 통합 TUI 로 보여줍니다(1-9=프로세스 솔로, /=필터). 파이프 출력 시 [name] 평문."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq run list", "프로파일 골라 실행 (대화형)"),
		output.HelpExample("uq run forceteller-app", "default 프로파일"),
		output.HelpExample("uq run forceteller-app:app3", "프로파일 명시"),
		output.HelpExample("uq run forceteller-admin", "back + front 동시 실행"),
		output.HelpExample("uq run forceteller-app --dry-run", "실제 실행 없이 풀어진 cmd/env 확인"),
		output.HelpExample("uq run targets", "등록된 프로파일 나열 (사람용)"),
		output.HelpExample("uq run targets --json", "에이전트/자동화용 머신 출력"),
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
			// 멀티프로세스 + 인터랙티브 터미널이면 uq log 와 동일한 공유 TUI 로 통합해
			// 보여준다. 출력이 파이프/리다이렉트면(둘 중 하나라도 비TTY) prefix 평문으로 폴백.
			// 단일 프로세스는 stdin 핫키 보존을 위해 TUI 를 쓰지 않고 그대로 터미널에 붙인다.
			useTUI := isTTY && term.IsTerminal(int(os.Stdout.Fd()))
			var runErr error
			switch {
			case len(profile.Procs) > 0 && useTUI:
				runErr = execTUI(c.Context(), dir, profile.Procs, env,
					fmt.Sprintf("uq run  %s:%s", repoName, resolvedName))
			case len(profile.Procs) > 0:
				runErr = execMulti(c.Context(), dir, profile.Procs, env)
			default:
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
	cmd.AddCommand(newTargetsCmd())
	cmd.AddCommand(newRunListCmd(cmd))
	return cmd
}

// newRunListCmd 은 `uq run list` 서브명령을 만든다. 등록된 프로파일을 대화형으로
// 고른 뒤 그대로 실행한다(uq log list 와 동일한 패턴, 비TTY 면 거절). 선택 후에는
// 부모 run 명령의 RunE 를 재사용해 동일한 실행 흐름(국가·포트·모드·exec)을 탄다.
func newRunListCmd(parent *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "프로파일을 대화형으로 골라 실행",
		Long: strings.Join([]string{
			output.Desc("등록된 run 프로파일을 대화형으로 고른 뒤 그대로 실행합니다."),
			output.Desc("프로파일 이름이 기억나지 않을 때 첫 단계부터 안내합니다."),
			"",
			output.Heading("예시"),
			output.HelpExample("uq run list", "프로파일 골라 실행"),
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(c *cobra.Command, args []string) error {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				return clierr.PreconditionError{Msg: "uq run list 는 대화형 터미널에서만 동작합니다 — `uq run <repo>[:profile]` 를 쓰세요"}
			}
			cfg, err := repocfg.Load()
			if err != nil {
				return err
			}
			reposDir, err := config.ReposDir()
			if err != nil {
				return err
			}
			profiles := collectProfiles(cfg, reposDir, "")
			if len(profiles) == 0 {
				fmt.Fprintln(c.OutOrStderr(), output.Dim("(등록된 프로파일 없음)"))
				return nil
			}
			target, err := pickProfile(profiles)
			if err != nil {
				return err
			}
			// 선택한 프로파일에 switch 가 선언돼 있으면 실행 전 대화형으로 적용한다.
			repoName, profileName, err := splitTarget(target)
			if err != nil {
				return err
			}
			if sw := cfg.Runs[repoName].Profiles[profileName].Switches; len(sw) > 0 {
				if err := applySwitches(c.OutOrStderr(), filepath.Join(reposDir, repoName), sw); err != nil {
					return err
				}
			}
			return parent.RunE(c, []string{target})
		},
	}
}

// pickProfile 은 TTY 에서 repo:profile 을 2단계로 선택받는다(uq log list 패턴):
// 먼저 레포(큰 종류)를 고르고, 그 레포에 프로파일이 여럿이면 프로파일을 고른다.
// 레포가 하나면 레포 단계를, 프로파일이 하나면 프로파일 단계를 생략한다.
// 반환값은 uq run 인자 형식(repo:profile). profiles 는 collectProfiles 의 순서
// (레포 알파벳순, 레포 내 default 우선)를 유지한다고 가정한다.
func pickProfile(profiles []profileJSON) (string, error) {
	var repos []string
	byRepo := map[string][]profileJSON{}
	for _, p := range profiles {
		if _, ok := byRepo[p.Repo]; !ok {
			repos = append(repos, p.Repo)
		}
		byRepo[p.Repo] = append(byRepo[p.Repo], p)
	}

	repo := repos[0]
	if len(repos) > 1 {
		r, err := pickOne("레포 선택", repos, repos, "")
		if err != nil {
			return "", err
		}
		repo = r
	}

	ps := byRepo[repo]
	if len(ps) == 1 {
		return repo + ":" + ps[0].Name, nil
	}
	labels := make([]string, len(ps))
	values := make([]string, len(ps))
	for i, p := range ps {
		labels[i] = p.Name
		if p.Default {
			labels[i] += "  (default)"
		}
		if p.Desc != "" {
			labels[i] += "  · " + p.Desc
		}
		values[i] = p.Name
	}
	name, err := pickOne(repo+" — 프로파일 선택", labels, values, "")
	if err != nil {
		return "", err
	}
	return repo + ":" + name, nil
}

// pickOne 은 라벨/값 병렬 슬라이스로 huh 단일 선택을 띄운다. def 가 values 에
// 있으면 그것을 기본 선택으로, 아니면 첫 항목을 기본으로 한다.
func pickOne(title string, labels, values []string, def string) (string, error) {
	opts := make([]huh.Option[string], len(values))
	for i := range values {
		opts[i] = huh.NewOption(labels[i], values[i])
	}
	choice := values[0]
	for _, v := range values {
		if v == def {
			choice = def
			break
		}
	}
	if err := huh.NewSelect[string]().
		Title(title).
		Options(opts...).
		Value(&choice).
		Run(); err != nil {
		return "", err
	}
	return choice, nil
}

// applySwitches 는 프로파일의 switch 들을 대화형으로 적용한다. scope 가 여럿이면
// 먼저 scope(예: 로케일)를 고르고, 그 scope 의 anchor 이후 영역에서 현재 옵션을
// 감지해 기본 선택으로 보여준 뒤, 다른 옵션을 고르면 그 한 군데만 정확히 치환한다.
func applySwitches(w io.Writer, repoDir string, switches []repocfg.Switch) error {
	for _, sw := range switches {
		path := filepath.Join(repoDir, sw.File)
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("switch %q 파일 읽기 실패: %w", sw.Name, err)
		}
		content := string(data)

		sc := sw.Scopes[0]
		if len(sw.Scopes) > 1 {
			labels := make([]string, len(sw.Scopes))
			for i, s := range sw.Scopes {
				labels[i] = s.Label
			}
			idx, err := pickIndex(sw.Name+" — 대상 선택", labels, 0)
			if err != nil {
				return err
			}
			sc = sw.Scopes[idx]
		}

		// anchor 이후 영역에서 현재 옵션을 감지한다(가장 앞선 match = 그 블록의 줄).
		start := 0
		if sc.Anchor != "" {
			a := strings.Index(content, sc.Anchor)
			if a < 0 {
				fmt.Fprintln(w, output.Yellow("⚠"), sw.Name+"/"+sc.Label+": anchor 를 못 찾아 건너뜁니다")
				continue
			}
			start = a
		}
		region := content[start:]
		cur, curPos := -1, -1
		for i, o := range sc.Options {
			if p := strings.Index(region, o.Match); p >= 0 && (curPos < 0 || p < curPos) {
				cur, curPos = i, p
			}
		}
		if cur < 0 {
			fmt.Fprintln(w, output.Yellow("⚠"), sw.Name+"/"+sc.Label+": 알려진 옵션을 못 찾아 건너뜁니다")
			continue
		}

		labels := make([]string, len(sc.Options))
		for i, o := range sc.Options {
			labels[i] = o.Label
			if i == cur {
				labels[i] += "  (현재)"
			}
		}
		title := sw.Name
		if len(sw.Scopes) > 1 {
			title += " (" + sc.Label + ")"
		}
		chosenIdx, err := pickIndex(title, labels, cur)
		if err != nil {
			return err
		}
		if chosenIdx == cur {
			continue
		}

		curMatch := sc.Options[cur].Match
		abs := start + curPos
		newContent := content[:abs] + sc.Options[chosenIdx].Match + content[abs+len(curMatch):]
		info, err := os.Stat(path)
		if err != nil {
			return fmt.Errorf("switch %q stat 실패: %w", sw.Name, err)
		}
		if err := os.WriteFile(path, []byte(newContent), info.Mode().Perm()); err != nil {
			return fmt.Errorf("switch %q 쓰기 실패: %w", sw.Name, err)
		}
		fmt.Fprintln(w, output.Green("✓"), title+" →", sc.Options[chosenIdx].Label)
	}
	return nil
}

// pickIndex 는 라벨 목록에서 하나를 골라 그 인덱스를 반환한다(def 인덱스를 기본 선택).
func pickIndex(title string, labels []string, def int) (int, error) {
	values := make([]string, len(labels))
	for i := range labels {
		values[i] = strconv.Itoa(i)
	}
	defVal := ""
	if def >= 0 && def < len(values) {
		defVal = values[def]
	}
	got, err := pickOne(title, labels, values, defVal)
	if err != nil {
		return 0, err
	}
	idx, _ := strconv.Atoi(got)
	return idx, nil
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
		mergedLabel := fmt.Sprintf("포그라운드 · 통합 TUI 로 합쳐 보기 (%s)", strings.Join(procNames, " + "))
		// 분할 옵션은 이 터미널이 실제로 할 수 있는 방식(패널/새 창)으로 문구를
		// 바꾸고, 분할 자체가 불가하면 메뉴에서 아예 뺀다.
		opts := []huh.Option[string]{huh.NewOption(mergedLabel, "fg")}
		if styleOf(run.DetectMultiplexer()) != styleNone {
			opts = append(opts, huh.NewOption(splitMenuLabel(run.DetectMultiplexer()), "split"))
		}
		opts = append(opts, huh.NewOption("백그라운드 · 로그는 파일로 남기고 바로 복귀", "bg"))
		if err := huh.NewSelect[string]().
			Title("어떻게 실행할까요?").
			Description("통합 TUI 는 q, 그 외 포그라운드는 Ctrl+C 로 종료합니다.").
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

// execTUI 는 1개 이상의 프로세스 로그를 uq log 와 동일한 공유 TUI 로 띄운다(TTY 전용).
// 각 proc 의 stdout/stderr 를 라인 단위로 읽어 LogLine 채널로 흘리고, 그 채널을
// eblogs.RunTUI 에 주입한다. proc N → 소스 #N(토글 라벨=proc 이름).
//
// 사용자가 q/Ctrl+C 로 TUI 를 닫거나 ctx 가 취소되면 RunTUI 가 반환하며, 그때
// 모든 자식 그룹에 SIGINT 를 보내 정리한다. 채널은 스캐너가 모두 끝나면 닫힌다.
func execTUI(ctx context.Context, repoDir string, procs []repocfg.Proc, env []string, title string) error {
	lanes := make([]eblogs.Lane, len(procs))
	for i, p := range procs {
		lanes[i] = eblogs.Lane{Num: i + 1, Toggle: p.Name}
	}

	ch := make(chan eblogs.LogLine, 256)
	pids := make([]int, 0, len(procs))
	done := make(chan procResult, len(procs))
	var scanWg sync.WaitGroup

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

		c := osexec.Command(p.Cmd[0], p.Cmd[1:]...)
		c.Dir = dir
		c.Env = env
		c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		stdout, err := c.StdoutPipe()
		if err != nil {
			killGroups(syscall.SIGTERM)
			return fmt.Errorf("proc %s stdout: %w", p.Name, err)
		}
		stderr, err := c.StderrPipe()
		if err != nil {
			killGroups(syscall.SIGTERM)
			return fmt.Errorf("proc %s stderr: %w", p.Name, err)
		}
		if err := c.Start(); err != nil {
			killGroups(syscall.SIGTERM)
			return fmt.Errorf("proc %s 시작 실패: %w", p.Name, err)
		}
		pids = append(pids, c.Process.Pid)

		num := i + 1
		name := p.Name
		// proc 별 스캐너 완료를 기다린 뒤 Wait 한다 — StdoutPipe 문서상 모든 읽기가
		// 끝나기 전에 Wait 하면 파이프가 일찍 닫혀 레이스가 난다.
		var pwg sync.WaitGroup
		pwg.Add(2)
		scanWg.Add(2)
		scan := func(r io.Reader) {
			defer scanWg.Done()
			defer pwg.Done()
			scanInto(ch, num, r)
		}
		go scan(stdout)
		go scan(stderr)
		go func(cmd *osexec.Cmd) {
			pwg.Wait()
			done <- procResult{name: name, err: cmd.Wait()}
		}(c)
	}

	// 모든 스캐너가 끝나면(= 모든 proc 종료) 채널을 닫는다.
	go func() {
		scanWg.Wait()
		close(ch)
	}()

	tuiErr := eblogs.RunTUI(ctx, ch, lanes, "", title)

	// TUI 가 닫혔다(q/Ctrl+C/ctx 취소) → 남은 출력을 비워 스캐너가 채널에 막히지
	// 않게 하고, 자식 그룹을 SIGINT 로 정리한 뒤 reap 한다. 사용자가 의도적으로
	// 닫은 것이므로 이때의 자식 종료(신호로 인한)는 실패로 취급하지 않는다.
	go func() {
		for range ch {
		}
	}()
	killGroups(syscall.SIGINT)
	for range pids {
		<-done
	}
	// 렌더링 자체가 실패한 경우만 에러로 surface 한다(q/시그널 종료는 정상 종료).
	if tuiErr != nil && ctx.Err() == nil {
		return tuiErr
	}
	return nil
}

// scanInto 는 r 을 라인 단위로 읽어 소스 번호 num 의 LogLine 으로 채널에 보낸다.
func scanInto(ch chan<- eblogs.LogLine, num int, r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		ch <- eblogs.LogLine{Num: num, Text: sc.Text(), Kind: eblogs.KindLog}
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
