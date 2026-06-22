// Package log implements the `uq log <repo>` command.
package log

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	authpkg "github.com/un7qi3inc/un7qi3-cli/internal/auth"
	"github.com/un7qi3inc/un7qi3-cli/internal/clierr"
	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
	eblogs "github.com/un7qi3inc/un7qi3-cli/internal/log"
	"github.com/un7qi3inc/un7qi3-cli/internal/output"
	"github.com/un7qi3inc/un7qi3-cli/internal/repocfg"
	"github.com/un7qi3inc/un7qi3-cli/internal/run"
)

// logsOptions 는 `uq logs` 의 플래그 값을 담는다. NewCmd 의 클로저에서 생성·바인딩해
// RunE 로 전달하므로 패키지 전역 상태가 없고, 명령을 여러 번 구성해도 격리된다.
type logsOptions struct {
	instanceNum int
	grep        string
	noFollow    bool
	split       bool
	dryRun      bool
	linesN      int
	plain       bool
}

// NewCmd returns the `uq log` command.
func NewCmd() *cobra.Command {
	opts := &logsOptions{}
	long := strings.Join([]string{
		output.Desc("Elastic Beanstalk 다중 인스턴스의 로그를 멀티플렉스로 스트리밍합니다."),
		"",
		output.Desc("기본은 전체 인스턴스를 한 스트림으로. ") + output.Yellow("--split") + output.Desc(" 으로 인스턴스별 패널 분리."),
		output.Desc("국가·환경은 위치인자로 지정하거나, 대상만 주면 대화형으로 고른 뒤 TUI 뷰어로 진입합니다."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq log list", "대상부터 대화형으로 선택"),
		output.HelpExample("uq log forceteller-api", "국가·환경 대화형 선택 → TUI 뷰어"),
		output.HelpExample("uq log targets", "등록된 log 대상 나열 (사람용)"),
		output.HelpExample("uq log targets --json", "에이전트/자동화용 머신 출력"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "log <대상> [필터...]",
		Short: "EB 인스턴스 멀티플렉스 로그 스트리밍",
		Long:  long,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogs(cmd, opts, args[0], args[1:])
		},
	}
	bindLogsFlags(cmd, opts)
	cmd.AddCommand(newTargetsCmd())
	cmd.AddCommand(newListCmd())
	return cmd
}

// bindLogsFlags 는 log 스트리밍 플래그를 cmd 에 바인딩한다. 기본 명령과 list
// 서브명령이 동일한 플래그를 갖도록 공유한다.
func bindLogsFlags(cmd *cobra.Command, opts *logsOptions) {
	cmd.Flags().IntVar(&opts.instanceNum, "instance", 0, "1-base 인스턴스 번호로 한정 (0=전체)")
	cmd.Flags().IntVar(&opts.linesN, "lines", 100, "초기 백로그 줄 수 (--grep 와 함께 늘리면 과거 로그 검색)")
	cmd.Flags().StringVar(&opts.grep, "grep", "", "정규식으로 라인 필터")
	cmd.Flags().BoolVar(&opts.noFollow, "no-follow", false, "follow 없이 최근 N줄만 출력하고 종료")
	cmd.Flags().BoolVar(&opts.split, "split", false, "인스턴스별 패널 분리 (cmux/iterm2)")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "해석된 app/region/환경/명령만 출력")
	cmd.Flags().BoolVar(&opts.plain, "plain", false, "TTY 라도 평문 스트리밍 강제 (TUI 끄기)")
	cmd.MarkFlagsMutuallyExclusive("split", "no-follow")
}

// newListCmd 은 `uq log list` 서브명령을 만든다. 대상 이름을 모를 때 첫 단계(대상
// 선택)부터 대화형으로 안내한 뒤, 기존 국가·환경 선택 흐름으로 이어 TUI 뷰어에
// 진입한다. 대화형이므로 TTY 가 아니면 거절한다.
func newListCmd() *cobra.Command {
	opts := &logsOptions{}
	cmd := &cobra.Command{
		Use:   "list",
		Short: "대상을 대화형으로 골라 로그 보기",
		Long: strings.Join([]string{
			output.Desc("등록된 log 대상을 대화형으로 고른 뒤, 국가·환경 선택을 거쳐 TUI 뷰어로 진입합니다."),
			output.Desc("대상 이름이 기억나지 않을 때 첫 단계부터 안내합니다."),
			"",
			output.Heading("예시"),
			output.HelpExample("uq log list", "대상 → 국가 → 환경 순서로 대화형 선택"),
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				return clierr.PreconditionError{Msg: "uq log list 는 대화형 터미널에서만 동작합니다 — `uq log <대상>` 을 쓰세요"}
			}
			cfg, err := repocfg.Load()
			if err != nil {
				return err
			}
			repos := cfg.LogsRepos()
			if len(repos) == 0 {
				fmt.Fprintln(cmd.OutOrStderr(), output.Dim("(등록된 log 대상 없음)"))
				return nil
			}
			repo, err := pickLogsRepo(repos)
			if err != nil {
				return err
			}
			return runLogs(cmd, opts, repo, nil)
		},
	}
	bindLogsFlags(cmd, opts)
	return cmd
}

// pickLogsRepo 는 TTY 에서 log 대상을 선택받는다.
func pickLogsRepo(repos []string) (string, error) {
	opts := make([]huh.Option[string], 0, len(repos))
	for _, r := range repos {
		opts = append(opts, huh.NewOption(r, r))
	}
	choice := repos[0]
	if err := huh.NewSelect[string]().
		Title("log 대상 선택").
		Options(opts...).
		Value(&choice).
		Run(); err != nil {
		return "", err
	}
	return choice, nil
}

// greenRepos 는 repo 목록을 초록색으로 칠해 ", " 로 잇는다(help·에러 공용 포맷).
func greenRepos(repos []string) string {
	greened := make([]string, len(repos))
	for i, r := range repos {
		greened[i] = output.Green(r)
	}
	return strings.Join(greened, ", ")
}

func runLogs(cmd *cobra.Command, opts *logsOptions, repo string, filters []string) error {
	w := cmd.OutOrStderr()

	// 1. allowlist 확인
	cfg, err := repocfg.Load()
	if err != nil {
		return err
	}
	lc, ok := cfg.LogsFor(repo)
	if !ok {
		// 사용자용 메시지(글리프·사용 가능한 대상 목록)는 여기서 직접 찍어 형식을
		// 보존한다. cobra 가 "Error: ..." 를 덧붙이지 않도록 SilenceErrors 를 켜고,
		// exit code 결정은 main 의 clierr.Classify(=1) 에 맡긴다.
		fmt.Fprintln(w, output.Red("✗"), repo, "는 logs 미등록입니다. repos.yml 의 logs: 에 추가하세요.")
		if repos := cfg.LogsRepos(); len(repos) > 0 {
			fmt.Fprintln(w, "  사용 가능한 대상:", greenRepos(repos))
		}
		cmd.SilenceErrors = true
		return clierr.RepoNotFoundError{Name: repo}
	}

	// 2. 외부 도구 사전확인 (exit 4) — aws 는 발견 단계 필수, eb 는 스트리밍에만
	//    쓰여 dry-run 이면 면제.
	if err := checkLogsPreconditions(authpkg.AwsStatus(cmd.Context()), opts.dryRun, uqexec.LookPath("eb")); err != nil {
		return err
	}

	// 3. 국가 결정
	codes := countryCodes(lc)
	country, envFilters := eblogs.SplitArgs(filters, codes)
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))
	if country == "" {
		if len(codes) == 1 {
			country = codes[0]
		} else if isTTY {
			country, err = pickCountry(codes)
			if err != nil {
				return err
			}
		} else {
			fmt.Fprintln(w, output.Red("✗"), "국가를 특정하세요(예:", strings.Join(codes, "/"), ")")
			cmd.SilenceErrors = true
			return clierr.PreconditionError{Msg: "국가 미지정"}
		}
	}
	tgt0, ok := lc.Countries[country]
	if !ok {
		fmt.Fprintln(w, output.Red("✗"), "알 수 없는 국가:", country)
		cmd.SilenceErrors = true
		return clierr.PreconditionError{Msg: "알 수 없는 국가: " + country}
	}
	tgt := eblogs.Target{Country: country, App: tgt0.App, Region: tgt0.Region}

	// 4. 환경 발견 + 필터 + 선택
	src := eblogs.NewEBSource(lc.PathOrDefault())
	envs, err := src.Environments(tgt)
	if err != nil {
		fmt.Fprintln(w, output.Red("✗"), err)
		cmd.SilenceErrors = true
		return clierr.PreconditionError{Msg: err.Error()}
	}
	matched := eblogs.MatchEnvs(envs, envFilters)
	env, err := resolveEnv(matched, isTTY)
	if err != nil {
		fmt.Fprintln(w, output.Red("✗"), err)
		cmd.SilenceErrors = true
		return clierr.PreconditionError{Msg: err.Error()}
	}

	// 4-1. SSH 키 해석 — eb ssh --custom 으로 accept-new 를 주입해 호스트 키 프롬프트를
	// 없앤다(다중 인스턴스 동시 tail 시 stdin 충돌 방지). 실패해도 치명적이지 않으므로
	// 경고만 하고 eb 기본 ssh 로 진행한다.
	if err := src.ResolveKey(tgt, env); err != nil {
		fmt.Fprintln(w, output.Yellow("⚠"), err)
	}

	// 5. 인스턴스 발견 (스냅샷)
	insts, err := src.Instances(tgt, env)
	if err != nil {
		fmt.Fprintln(w, output.Red("✗"), err)
		cmd.SilenceErrors = true
		return clierr.PreconditionError{Msg: err.Error()}
	}
	if opts.instanceNum > 0 {
		insts = filterInstance(insts, opts.instanceNum)
	}
	if len(insts) == 0 {
		fmt.Fprintln(w, output.Red("✗"), "대상 인스턴스가 없습니다")
		cmd.SilenceErrors = true
		return clierr.PreconditionError{Msg: "대상 인스턴스가 없습니다"}
	}

	// 6. dry-run / split / merged
	lines := opts.linesN
	if lines < 1 {
		lines = 1
	}
	if opts.dryRun {
		return printLogsPlan(w, tgt, env, insts, src, !opts.noFollow, lines, opts.grep)
	}
	if opts.split {
		mux := run.DetectMultiplexer()
		if eblogs.SplitSupported(mux) {
			return runLogsSplit(w, src, tgt, env, insts, mux, !opts.noFollow, lines, opts.grep)
		}
		fmt.Fprintln(w, output.Yellow("⚠"), "현재 터미널은 분할 미지원 — merged 로 진행")
	}
	if isStdoutTTY() && !opts.noFollow && !opts.plain {
		tctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()
		ch := eblogs.StreamLines(tctx, src, tgt, env, insts, true, lines, opts.grep)
		return eblogs.RunTUI(tctx, ch, insts, opts.grep, tgt.App, env)
	}
	return eblogs.StreamMerged(cmd.Context(), w, src, tgt, env, insts, !opts.noFollow, lines, opts.grep)
}

// checkLogsPreconditions 는 logs 실행 전 외부 도구 상태를 검증한다.
// aws 는 발견 단계(describe-environments 등)에 항상 필수다. eb 는 스트리밍에만
// (eb ssh) 쓰이므로 dry-run 이면 생략한다 — dry-run 은 명령만 출력하기 때문.
// eb 는 aws 자격증명으로 동작해 자체 인증이 없으니 presence(hasEB)만 본다.
func checkLogsPreconditions(aws authpkg.Status, dryRun, hasEB bool) error {
	if !aws.OK {
		return &authpkg.RequiredError{Msg: "aws 인증 안 됨. `uq auth login --aws-only` 실행"}
	}
	if !dryRun && !hasEB {
		return &authpkg.RequiredError{Msg: "eb CLI 미설치. `brew install aws-elasticbeanstalk` 실행"}
	}
	return nil
}

// isStdoutTTY 는 표준출력이 터미널인지.
func isStdoutTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// countryPreferredOrder 는 국가 선택 시 고정 노출 순서. 여기 없는 코드는
// 알파벳순으로 뒤에 붙는다.
var countryPreferredOrder = []string{"kr", "en", "jp"}

// countryCodes 는 LogsConfig 의 국가 코드를 kr→en→jp 고정 순서로 반환한다.
func countryCodes(lc repocfg.LogsConfig) []string {
	preferred := make(map[string]bool, len(countryPreferredOrder))
	codes := make([]string, 0, len(lc.Countries))
	for _, c := range countryPreferredOrder {
		if _, ok := lc.Countries[c]; ok {
			codes = append(codes, c)
			preferred[c] = true
		}
	}
	rest := make([]string, 0)
	for k := range lc.Countries {
		if !preferred[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	return append(codes, rest...)
}

// pickCountry 는 TTY 에서 국가 코드를 선택받는다.
func pickCountry(codes []string) (string, error) {
	opts := make([]huh.Option[string], 0, len(codes))
	for _, c := range codes {
		opts = append(opts, huh.NewOption(c, c))
	}
	choice := codes[0]
	if err := huh.NewSelect[string]().
		Title("국가 선택").
		Options(opts...).
		Value(&choice).
		Run(); err != nil {
		return "", err
	}
	return choice, nil
}

// resolveEnv 는 매칭된 환경 목록에서 하나를 결정한다.
// 0개 → 에러, 1개 → 자동 선택, 다수 → TTY 에서 선택 또는 에러.
func resolveEnv(matched []string, isTTY bool) (string, error) {
	switch len(matched) {
	case 0:
		return "", fmt.Errorf("조건에 맞는 환경이 없습니다")
	case 1:
		return matched[0], nil
	}
	if !isTTY {
		return "", fmt.Errorf("환경이 여러 개입니다: %s — 필터를 추가하세요", strings.Join(matched, ", "))
	}
	opts := make([]huh.Option[string], 0, len(matched))
	for _, e := range matched {
		opts = append(opts, huh.NewOption(e, e))
	}
	choice := matched[0]
	if err := huh.NewSelect[string]().
		Title("환경 선택").
		Options(opts...).
		Value(&choice).
		Run(); err != nil {
		return "", err
	}
	return choice, nil
}

// filterInstance 는 Num 이 일치하는 인스턴스만 반환한다.
func filterInstance(insts []eblogs.Instance, num int) []eblogs.Instance {
	for _, in := range insts {
		if in.Num == num {
			return []eblogs.Instance{in}
		}
	}
	return nil
}

// printLogsPlan 은 dry-run 시 해석된 대상과 eb 명령을 출력한다.
func printLogsPlan(w io.Writer, tgt eblogs.Target, env string, insts []eblogs.Instance,
	src eblogs.Source, follow bool, lines int, grep string) error {
	fmt.Fprintf(w, "%s logs dry-run\n", output.Bold("uq"))
	fmt.Fprintf(w, "  app:    %s\n", tgt.App)
	fmt.Fprintf(w, "  region: %s\n", tgt.Region)
	fmt.Fprintf(w, "  env:    %s\n", env)
	fmt.Fprintf(w, "  인스턴스 %d개:\n", len(insts))
	for _, in := range insts {
		fmt.Fprintf(w, "    %s → %s\n", output.Cyan(in.Label), output.Dim(in.ID))
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s\n", output.Bold("명령:"))
	for _, in := range insts {
		args := src.TailArgs(tgt, env, in, follow, lines, grep)
		fmt.Fprintf(w, "    eb %s\n", strings.Join(args, " "))
	}
	return nil
}

// runLogsSplit 은 멀티플렉서 패널로 인스턴스별 로그를 분리 실행한다.
func runLogsSplit(w io.Writer, src eblogs.Source, tgt eblogs.Target, env string,
	insts []eblogs.Instance, mux run.Multiplexer, follow bool, lines int, grep string) error {
	argvs := make([][]string, 0, len(insts))
	labels := make([]string, 0, len(insts))
	for _, in := range insts {
		argvs = append(argvs, src.TailArgs(tgt, env, in, follow, lines, grep))
		labels = append(labels, in.Label)
	}
	panels := eblogs.BuildPanels(argvs, labels)
	for _, p := range panels {
		fmt.Fprintf(w, "%s 패널 열기: %s\n", output.Cyan("▶"), p.Label)
		if err := run.OpenPanel(mux, p, run.SplitCol); err != nil {
			return fmt.Errorf("패널 열기 실패(%s): %w", p.Label, err)
		}
	}
	return nil
}
