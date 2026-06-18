// Package logs implements the `uq logs <repo>` command.
package logs

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	authpkg "github.com/un7qi3inc/un7qi3-cli/internal/auth"
	eblogs "github.com/un7qi3inc/un7qi3-cli/internal/logs"
	"github.com/un7qi3inc/un7qi3-cli/internal/output"
	"github.com/un7qi3inc/un7qi3-cli/internal/repocfg"
	"github.com/un7qi3inc/un7qi3-cli/internal/run"
)

var (
	instanceNum int
	grep        string
	noFollow    bool
	split       bool
	dryRun      bool
)

// NewCmd returns the `uq logs` command.
func NewCmd() *cobra.Command {
	long := strings.Join([]string{
		output.Desc("Elastic Beanstalk 다중 인스턴스의 로그를 멀티플렉스로 스트리밍합니다."),
		"",
		output.Desc("기본은 전체 인스턴스를 한 스트림으로. ") + output.Yellow("--split") + output.Desc(" 으로 인스턴스별 패널 분리."),
		output.Desc("국가·환경은 위치인자로 지정하거나, TTY 에서 대화형 선택합니다."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq logs forceteller-api kr beta", "kr beta 환경 전체 인스턴스"),
		output.HelpExample("uq logs forceteller-api kr", "kr 환경 대화형 선택"),
		output.HelpExample("uq logs forceteller-api kr beta --split", "인스턴스별 패널 분리"),
		output.HelpExample("uq logs forceteller-api kr beta --grep ERROR", "ERROR 패턴만 필터"),
		output.HelpExample("uq logs forceteller-api kr beta --dry-run", "해석된 명령만 출력"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "logs <repo> [필터...]",
		Short: "EB 인스턴스 멀티플렉스 로그 스트리밍",
		Long:  long,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogs(cmd, args[0], args[1:])
		},
	}
	cmd.Flags().IntVar(&instanceNum, "instance", 0, "1-base 인스턴스 번호로 한정 (0=전체)")
	cmd.Flags().StringVar(&grep, "grep", "", "정규식으로 라인 필터")
	cmd.Flags().BoolVar(&noFollow, "no-follow", false, "follow 없이 최근 N줄만 출력하고 종료")
	cmd.Flags().BoolVar(&split, "split", false, "인스턴스별 패널 분리 (cmux/iterm2)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "해석된 app/region/환경/명령만 출력")
	cmd.MarkFlagsMutuallyExclusive("split", "no-follow")
	return cmd
}

func runLogs(cmd *cobra.Command, repo string, filters []string) error {
	w := cmd.OutOrStderr()

	// 1. allowlist 확인
	cfg, err := repocfg.Load()
	if err != nil {
		return err
	}
	lc, ok := cfg.LogsFor(repo)
	if !ok {
		fmt.Fprintln(w, output.Red("✗"), repo, "는 logs 미등록입니다. repos.yml 의 logs: 에 추가하세요.")
		os.Exit(1)
	}

	// 2. aws 인증 사전확인 (exit 4)
	if s := authpkg.AwsStatus(); !s.OK {
		return &authpkg.RequiredError{Msg: "aws 인증 안 됨. `uq auth login --aws-only` 실행"}
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
			os.Exit(1)
		}
	}
	tgt0, ok := lc.Countries[country]
	if !ok {
		fmt.Fprintln(w, output.Red("✗"), "알 수 없는 국가:", country)
		os.Exit(1)
	}
	tgt := eblogs.Target{Country: country, App: tgt0.App, Region: tgt0.Region}

	// 4. 환경 발견 + 필터 + 선택
	src := eblogs.NewEBSource(lc.PathOrDefault())
	envs, err := src.Environments(tgt)
	if err != nil {
		fmt.Fprintln(w, output.Red("✗"), err)
		os.Exit(1)
	}
	matched := eblogs.MatchEnvs(envs, envFilters)
	env, err := resolveEnv(matched, isTTY)
	if err != nil {
		fmt.Fprintln(w, output.Red("✗"), err)
		os.Exit(1)
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
		os.Exit(1)
	}
	if instanceNum > 0 {
		insts = filterInstance(insts, instanceNum)
	}
	if len(insts) == 0 {
		fmt.Fprintln(w, output.Red("✗"), "대상 인스턴스가 없습니다")
		os.Exit(1)
	}

	// 6. dry-run / split / merged
	lines := 100
	if dryRun {
		return printLogsPlan(w, tgt, env, insts, src, !noFollow, lines)
	}
	if split {
		mux := run.DetectMultiplexer()
		if eblogs.SplitSupported(mux) {
			if grep != "" {
				fmt.Fprintln(w, output.Yellow("⚠"), "--split 에서는 --grep 가 적용되지 않습니다")
			}
			return runLogsSplit(w, src, tgt, env, insts, mux, !noFollow, lines)
		}
		fmt.Fprintln(w, output.Yellow("⚠"), "현재 터미널은 분할 미지원 — merged 로 진행")
	}
	return eblogs.StreamMerged(cmd.Context(), w, src, tgt, env, insts, !noFollow, lines, grep)
}

// countryCodes 는 LogsConfig 의 국가 코드 목록을 반환한다.
func countryCodes(lc repocfg.LogsConfig) []string {
	codes := make([]string, 0, len(lc.Countries))
	for k := range lc.Countries {
		codes = append(codes, k)
	}
	sort.Strings(codes)
	return codes
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
	src eblogs.Source, follow bool, lines int) error {
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
		args := src.TailArgs(tgt, env, in, follow, lines)
		fmt.Fprintf(w, "    eb %s\n", strings.Join(args, " "))
	}
	return nil
}

// runLogsSplit 은 멀티플렉서 패널로 인스턴스별 로그를 분리 실행한다.
func runLogsSplit(w io.Writer, src eblogs.Source, tgt eblogs.Target, env string,
	insts []eblogs.Instance, mux run.Multiplexer, follow bool, lines int) error {
	argvs := make([][]string, 0, len(insts))
	labels := make([]string, 0, len(insts))
	for _, in := range insts {
		argvs = append(argvs, src.TailArgs(tgt, env, in, follow, lines))
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
