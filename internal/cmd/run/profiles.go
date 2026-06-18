package run

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/un7qi3inc/un7qi3-cli/internal/config"
	"github.com/un7qi3inc/un7qi3-cli/internal/output"
	"github.com/un7qi3inc/un7qi3-cli/internal/repocfg"
)

// profileJSON is the stable JSON shape consumed by AI agents and automation.
// Internal repocfg types may evolve; this DTO is the contract.
//
// Conventions:
//   - `cwd` is an absolute path the agent can `cd` into directly. For procs,
//     each proc's `cwd` is also absolute (already joined with the repo root).
//   - `default` flags the profile that bare `uq run <repo>` would launch.
//   - `cmd` is set for single-process profiles; `procs` for multi-process.
//     The two are mutually exclusive but the JSON shape allows both for
//     forward compatibility (agents should prefer procs if present).
type profileJSON struct {
	Repo    string            `json:"repo"`
	Name    string            `json:"name"`
	Default bool              `json:"default"`
	Cwd     string            `json:"cwd"`
	Node    string            `json:"node,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	URL     string            `json:"url,omitempty"`
	Cmd     []string          `json:"cmd,omitempty"`
	Procs   []procJSON        `json:"procs,omitempty"`
	// Countries lists the locale variants when the profile declares a country
	// axis (forceteller-admin.local). Absent for profiles without one.
	Countries []countryJSON `json:"countries,omitempty"`
}

type procJSON struct {
	Name string   `json:"name"`
	Cwd  string   `json:"cwd"`
	Cmd  []string `json:"cmd"`
	URL  string   `json:"url,omitempty"`
}

type countryJSON struct {
	Code     string   `json:"code"`
	Script   string   `json:"script"`
	Default  bool     `json:"default,omitempty"`
	Requires []string `json:"requires,omitempty"`
}

type profilesOutput struct {
	Profiles []profileJSON `json:"profiles"`
}

func newProfilesCmd() *cobra.Command {
	long := strings.Join([]string{
		output.Desc("등록된 ") + output.Cyan("uq run") + output.Desc(" 프로파일을 나열합니다."),
		"",
		output.Desc("AI agent / 자동화용 머신 출력은 ") + output.Yellow("--json") + output.Desc(" 으로."),
		output.Desc("repo 이름을 지정하면 그 repo 의 프로파일만 표시합니다."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq run profiles", "전체 프로파일 (사람용 표)"),
		output.HelpExample("uq run profiles --json", "전체 (JSON, 안정 스키마)"),
		output.HelpExample("uq run profiles forceteller-admin", "특정 repo 만"),
		output.HelpExample("uq run profiles --json | jq '.profiles[]|select(.default)'", "default 만"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "profiles [repo]",
		Short: "등록된 run 프로파일 나열",
		Long:  long,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			cfg, err := repocfg.Load()
			if err != nil {
				return err
			}
			reposDir, err := config.ReposDir()
			if err != nil {
				return err
			}
			filter := ""
			if len(args) == 1 {
				filter = args[0]
			}
			profiles := collectProfiles(cfg, reposDir, filter)
			if filter != "" && len(profiles) == 0 {
				return fmt.Errorf("'%s' 에 등록된 run 프로파일이 없습니다", filter)
			}
			jsonOut, _ := c.Flags().GetBool("json")
			if jsonOut {
				enc := json.NewEncoder(c.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(profilesOutput{Profiles: profiles})
			}
			printProfilesHuman(c.OutOrStdout(), profiles)
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "JSON 형식으로 출력")
	return cmd
}

// collectProfiles flattens cfg.Runs into a deterministic, agent-stable list.
//
// Ordering: repos are sorted alphabetically. Within a repo, the default
// profile appears first, then the rest alphabetically.
//
// reposDir is the workspace root — joined with "<repo>" to produce the
// absolute cwd. filterRepo, when non-empty, limits output to that repo.
func collectProfiles(cfg *repocfg.Config, reposDir, filterRepo string) []profileJSON {
	out := []profileJSON{}
	repos := make([]string, 0, len(cfg.Runs))
	for k := range cfg.Runs {
		if filterRepo != "" && k != filterRepo {
			continue
		}
		repos = append(repos, k)
	}
	sort.Strings(repos)

	for _, repo := range repos {
		runs := cfg.Runs[repo]
		names := make([]string, 0, len(runs.Profiles))
		for n := range runs.Profiles {
			names = append(names, n)
		}
		sort.Slice(names, func(i, j int) bool {
			if names[i] == runs.Default {
				return true
			}
			if names[j] == runs.Default {
				return false
			}
			return names[i] < names[j]
		})

		repoCwd := filepath.Join(reposDir, repo)
		for _, name := range names {
			p := runs.Profiles[name]
			isDefault := name == runs.Default ||
				(runs.Default == "" && len(runs.Profiles) == 1)

			entry := profileJSON{
				Repo:    repo,
				Name:    name,
				Default: isDefault,
				Cwd:     repoCwd,
				Node:    p.Node,
				Env:     p.Env,
				URL:     p.URL,
				Cmd:     p.Cmd,
			}
			if len(p.Procs) > 0 {
				entry.Procs = make([]procJSON, 0, len(p.Procs))
				for _, pr := range p.Procs {
					procCwd := repoCwd
					if pr.Cwd != "" {
						procCwd = filepath.Join(repoCwd, pr.Cwd)
					}
					entry.Procs = append(entry.Procs, procJSON{
						Name: pr.Name,
						Cwd:  procCwd,
						Cmd:  pr.Cmd,
						URL:  pr.URL,
					})
				}
			}
			if p.Countries != nil {
				entry.Countries = make([]countryJSON, 0, len(p.Countries.Options))
				for _, ct := range p.Countries.Options {
					entry.Countries = append(entry.Countries, countryJSON{
						Code:     ct.Code,
						Script:   ct.Script,
						Default:  ct.Code == p.Countries.Default,
						Requires: ct.Requires,
					})
				}
			}
			out = append(out, entry)
		}
	}
	return out
}

// printProfilesHuman renders one row per profile as a tab-aligned table.
//
// Columns: REPO:PROFILE | DEFAULT | NODE | 종류 | URL or proc summary.
func printProfilesHuman(w io.Writer, profiles []profileJSON) {
	if len(profiles) == 0 {
		fmt.Fprintln(w, output.Dim("(등록된 프로파일 없음)"))
		return
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "REPO:PROFILE\tDEFAULT\tNODE\t종류\tURL/프로세스")
	for _, p := range profiles {
		def := " "
		if p.Default {
			def = output.Green("✓")
		}
		node := p.Node
		if node == "" {
			node = "-"
		}
		var kind, summary string
		if len(p.Procs) > 0 {
			kind = fmt.Sprintf("프로세스 %d개", len(p.Procs))
			parts := make([]string, 0, len(p.Procs))
			for _, pr := range p.Procs {
				if pr.URL != "" {
					parts = append(parts, fmt.Sprintf("%s=%s", pr.Name, pr.URL))
				} else {
					parts = append(parts, pr.Name)
				}
			}
			summary = strings.Join(parts, ", ")
		} else {
			kind = "단일 실행"
			summary = p.URL
			if summary == "" {
				summary = strings.Join(p.Cmd, " ")
			}
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			output.Cyan(p.Repo+":"+p.Name), def, node, kind, summary)
	}
	_ = tw.Flush()
}
