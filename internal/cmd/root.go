// Package cmd wires the cobra command tree for the uq binary.
package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/un7qi3inc/un7qi3-cli/internal/cmd/auth"
	"github.com/un7qi3inc/un7qi3-cli/internal/cmd/deploy"
	"github.com/un7qi3inc/un7qi3-cli/internal/cmd/doctor"
	envcmd "github.com/un7qi3inc/un7qi3-cli/internal/cmd/env"
	initcmd "github.com/un7qi3inc/un7qi3-cli/internal/cmd/initcmd"
	"github.com/un7qi3inc/un7qi3-cli/internal/cmd/logs"
	"github.com/un7qi3inc/un7qi3-cli/internal/cmd/repo"
	runcmd "github.com/un7qi3inc/un7qi3-cli/internal/cmd/run"
	"github.com/un7qi3inc/un7qi3-cli/internal/cmd/skills"
	"github.com/un7qi3inc/un7qi3-cli/internal/cmd/upgrade"
	versioncmd "github.com/un7qi3inc/un7qi3-cli/internal/cmd/version"
	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

var (
	flagJSON    bool
	flagVerbose bool
	flagConfig  string
)

// Top-level command groups. The IDs are arbitrary stable strings; the Titles
// are what users see in `uq --help`.
const (
	groupSetup = "setup"
	groupDev   = "dev"
	groupOps   = "ops"
	groupTools = "tools"
)

const usageTemplate = `{{heading "사용법:"}}{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [명령]{{end}}{{if gt (len .Aliases) 0}}

{{heading "별칭:"}}
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

{{heading "예시:"}}
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

{{heading "사용 가능한 명령:"}}
{{cmdList $cmds}}{{else}}{{range $group := .Groups}}

{{heading $group.Title}}
{{cmdGroup $cmds $group.ID}}{{end}}{{if not .AllChildCommandsHaveGroup}}

{{heading "추가 명령:"}}
{{cmdGroup $cmds ""}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

{{heading "플래그:"}}
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces | colorFlags}}{{end}}{{if .HasAvailableInheritedFlags}}

{{heading "전역 플래그:"}}
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces | colorFlags}}{{end}}{{if .HasHelpSubCommands}}

{{heading "추가 도움말:"}}{{range .Commands}}{{if .IsAdditionalHelpTopicRunnable}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

명령별 자세한 도움말은 {{cyan "uq <명령> --help"}} 를 실행하세요.{{end}}
`

const helpTemplate = `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`

var rootCmd = &cobra.Command{
	Use:   "uq",
	Short: "un7qi3 사내 CLI",
	Long: strings.Join([]string{
		output.Desc("uq는 un7qi3 사내 CLI입니다."),
		"",
		output.Desc("온보딩, 레포 셋업, 시크릿, 배포, 운영 작업을 위한"),
		output.Desc("LLM 호출 가능한 결정론적 도구입니다. 주로 Claude Code가 호출하며,"),
		output.Desc("사람도 사용할 수 있도록 친화적인 출력을 제공합니다."),
		"",
		output.Heading("자주 쓰는 명령"),
		output.HelpExample("uq init", "최초 설정 (인증 + 워크스페이스)"),
		output.HelpExample("uq doctor", "필수 도구 설치 점검"),
		output.HelpExample("uq auth status", "gh / aws / gcloud 인증 상태"),
		output.HelpExample("uq repo list", "un7qi3inc 레포 목록"),
		output.HelpExample("uq repo clone <name>", "~/un7qi3/<name>에 클론"),
	}, "\n"),
	SilenceUsage:  true,
	SilenceErrors: false,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		uqexec.SetVerbose(flagVerbose)
	},
}

func init() {
	cobra.AddTemplateFunc("heading", output.Heading)
	cobra.AddTemplateFunc("cyan", output.Cyan)
	cobra.AddTemplateFunc("colorFlags", output.ColorizeFlagUsages)
	cobra.AddTemplateFunc("cmdList", renderCommandList)
	cobra.AddTemplateFunc("cmdGroup", renderCommandGroup)

	rootCmd.AddGroup(
		&cobra.Group{ID: groupSetup, Title: "시작하기"},
		&cobra.Group{ID: groupDev, Title: "개발 워크플로"},
		&cobra.Group{ID: groupOps, Title: "배포 & 운영"},
		&cobra.Group{ID: groupTools, Title: "도구"},
	)

	rootCmd.SetUsageTemplate(usageTemplate)
	rootCmd.SetHelpTemplate(helpTemplate)

	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "JSON 형식으로 출력")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "상세 로그 출력")
	rootCmd.PersistentFlags().StringVar(&flagConfig, "config", "", "설정 파일 경로")

	helpCmd := newHelpCmd()
	helpCmd.GroupID = groupTools
	rootCmd.SetHelpCommand(helpCmd)

	// completion 은 cobra가 공짜로 주는 셸 자동완성 스크립트 생성기.
	// 신규 사용자에게는 첫인상에 노이즈가 되므로 --help 에서 숨기고,
	// 아는 사람만 `uq completion zsh > ...` 로 직접 호출하도록 둔다.
	completionCmd := newCompletionCmd()
	completionCmd.GroupID = groupTools
	completionCmd.Hidden = true
	rootCmd.AddCommand(completionCmd)
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// 시작하기 — 새 머신/팀원 온보딩, uq 자체 유지보수
	rootCmd.AddCommand(inGroup(initcmd.NewCmd(), groupSetup))
	rootCmd.AddCommand(inGroup(doctor.NewCmd(), groupSetup))
	rootCmd.AddCommand(inGroup(upgrade.NewCmd(), groupSetup))
	rootCmd.AddCommand(inGroup(versioncmd.NewCmd(), groupSetup))

	// 개발 워크플로 — 인증, 레포, 로컬 실행, 시크릿
	rootCmd.AddCommand(inGroup(auth.NewCmd(), groupDev))
	rootCmd.AddCommand(inGroup(repo.NewCmd(), groupDev))
	rootCmd.AddCommand(inGroup(runcmd.NewCmd(), groupDev))
	rootCmd.AddCommand(inGroup(envcmd.NewCmd(), groupDev))

	// 배포 & 운영
	rootCmd.AddCommand(inGroup(deploy.NewCmd(), groupOps))
	rootCmd.AddCommand(inGroup(logs.NewCmd(), groupOps))

	// 도구
	rootCmd.AddCommand(inGroup(skills.NewCmd(), groupTools))

	// -h/--help 플래그 설명을 한글로
	rootCmd.PersistentFlags().BoolP("help", "h", false, "도움말 표시")
}

func newHelpCmd() *cobra.Command {
	long := strings.Join([]string{
		output.Desc("지정한 명령의 도움말을 표시합니다."),
		output.Desc("인자를 생략하면 uq 전체 도움말을 표시합니다."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq help", "uq 도움말"),
		output.HelpExample("uq help auth login", "특정 명령 도움말"),
	}, "\n")
	return &cobra.Command{
		Use:   "help [명령]",
		Short: "명령에 대한 도움말",
		Long:  long,
		Run: func(c *cobra.Command, args []string) {
			cmd, _, err := c.Root().Find(args)
			if cmd == nil || err != nil {
				c.Printf("알 수 없는 명령: %v\n", args)
				_ = c.Root().Usage()
				return
			}
			cmd.InitDefaultHelpFlag()
			_ = cmd.Help()
		},
	}
}

func newCompletionCmd() *cobra.Command {
	long := strings.Join([]string{
		output.Desc("지정한 셸의 자동완성 스크립트를 표준 출력으로 생성합니다."),
		"",
		output.Heading("예시"),
		output.HelpExample(`uq completion zsh > "${fpath[1]}/_uq"`, "zsh"),
		output.HelpExample("uq completion bash > /usr/local/etc/bash_completion.d/uq", "bash"),
		output.HelpExample("uq completion fish > ~/.config/fish/completions/uq.fish", "fish"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "지정한 셸의 자동완성 스크립트 생성",
		Long:  long,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			}
			return nil
		},
	}
	return cmd
}

// inGroup tags c with groupID and returns it, for use inline with AddCommand.
func inGroup(c *cobra.Command, groupID string) *cobra.Command {
	c.GroupID = groupID
	return c
}

// renderCommandGroup is the templated `cmdGroup $cmds $groupID` — same
// formatting as renderCommandList but only over commands matching groupID.
// Pass "" for ungrouped commands (the "추가 명령" fallback).
func renderCommandGroup(cmds []*cobra.Command, groupID string) string {
	filtered := make([]*cobra.Command, 0, len(cmds))
	for _, c := range cmds {
		if c.GroupID != groupID {
			continue
		}
		filtered = append(filtered, c)
	}
	return renderCommandList(filtered)
}

// renderCommandList formats the subcommand listing block exactly like
// cobra's default template, then colorizes command names.
func renderCommandList(cmds []*cobra.Command) string {
	padding := 0
	visible := make([]*cobra.Command, 0, len(cmds))
	for _, c := range cmds {
		if !c.IsAvailableCommand() && c.Name() != "help" {
			continue
		}
		visible = append(visible, c)
		if l := len(c.Name()); l > padding {
			padding = l
		}
	}
	var b strings.Builder
	for i, c := range visible {
		name := c.Name()
		pad := strings.Repeat(" ", padding-len(name))
		b.WriteString("  ")
		b.WriteString(output.Cyan(name))
		b.WriteString(pad)
		b.WriteString("  ")
		b.WriteString(c.Short)
		if i < len(visible)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
