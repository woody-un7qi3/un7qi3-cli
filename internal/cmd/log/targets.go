package log

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/un7qi3inc/un7qi3-cli/internal/output"
	"github.com/un7qi3inc/un7qi3-cli/internal/repocfg"
)

// logsTargetJSON 은 AI agent·자동화가 소비하는 안정 JSON 스키마다.
// 내부 repocfg 타입은 바뀔 수 있으나 이 DTO 는 계약으로 고정한다.
//
//	repo       repos.yml 의 logs: 키
//	path       인스턴스에서 tail 할 로그 파일 경로(미설정 시 기본값)
//	countries  국가코드→(app, region). kr→en→jp 고정 순서, 나머지는 알파벳순
type logsTargetJSON struct {
	Repo      string            `json:"repo"`
	Path      string            `json:"path"`
	Countries []logsCountryJSON `json:"countries"`
}

type logsCountryJSON struct {
	Code   string `json:"code"`
	App    string `json:"app"`
	Region string `json:"region"`
}

type logsTargetsOutput struct {
	Targets []logsTargetJSON `json:"targets"`
}

// newTargetsCmd 은 `uq log targets [repo]` 서브명령을 만든다. run profiles 와
// 동일한 패턴: repos.yml 에서 동적으로 읽어 사람용 표 또는 --json 으로 낸다.
func newTargetsCmd() *cobra.Command {
	long := strings.Join([]string{
		output.Desc("등록된 ") + output.Cyan("uq log") + output.Desc(" 대상을 나열합니다."),
		"",
		output.Desc("AI agent / 자동화용 머신 출력은 ") + output.Yellow("--json") + output.Desc(" 으로."),
		output.Desc("repo 이름을 지정하면 그 repo 의 대상만 표시합니다."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq log targets", "전체 대상 (사람용 표)"),
		output.HelpExample("uq log targets --json", "전체 (JSON, 안정 스키마)"),
		output.HelpExample("uq log targets forceteller-api", "특정 repo 만"),
		output.HelpExample("uq log targets --json | jq '.targets[].countries[]'", "국가만 평탄화"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "targets [repo]",
		Short: "등록된 log 대상 나열",
		Long:  long,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := repocfg.Load()
			if err != nil {
				return err
			}
			filter := ""
			if len(args) == 1 {
				filter = args[0]
			}
			targets := collectLogsTargets(cfg, filter)
			if filter != "" && len(targets) == 0 {
				return fmt.Errorf("'%s' 에 등록된 log 대상이 없습니다", filter)
			}
			jsonOut, _ := c.Flags().GetBool("json")
			if jsonOut {
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(logsTargetsOutput{Targets: targets})
			}
			printLogsTargetsHuman(c.OutOrStdout(), targets)
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "JSON 형식으로 출력")
	return cmd
}

// collectLogsTargets 는 cfg.Logs 를 결정적이고 agent 안정적인 목록으로 평탄화한다.
//
// 정렬: 레포는 알파벳순(LogsRepos). 국가는 countryCodes 와 동일하게 kr→en→jp
// 고정 순서, 나머지는 알파벳순. filterRepo 가 비어있지 않으면 그 repo 로 한정한다.
func collectLogsTargets(cfg *repocfg.Config, filterRepo string) []logsTargetJSON {
	out := []logsTargetJSON{}
	for _, repo := range cfg.LogsRepos() {
		if filterRepo != "" && repo != filterRepo {
			continue
		}
		lc := cfg.Logs[repo]
		entry := logsTargetJSON{
			Repo:      repo,
			Path:      lc.PathOrDefault(),
			Countries: make([]logsCountryJSON, 0, len(lc.Countries)),
		}
		for _, code := range countryCodes(lc) {
			ct := lc.Countries[code]
			entry.Countries = append(entry.Countries, logsCountryJSON{
				Code:   code,
				App:    ct.App,
				Region: ct.Region,
			})
		}
		out = append(out, entry)
	}
	return out
}

// printLogsTargetsHuman 은 (repo, 국가) 한 쌍을 한 줄로 탭 정렬 표로 렌더한다.
//
// 컬럼: REPO:국가 | APP | REGION | 로그경로.
func printLogsTargetsHuman(w io.Writer, targets []logsTargetJSON) {
	if len(targets) == 0 {
		fmt.Fprintln(w, output.Dim("(등록된 log 대상 없음)"))
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "REPO:국가\tAPP\tREGION\t로그경로")
	for _, t := range targets {
		for _, c := range t.Countries {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
				output.Cyan(t.Repo+":"+c.Code), c.App, c.Region, output.Dim(t.Path))
		}
	}
	_ = tw.Flush()
}
