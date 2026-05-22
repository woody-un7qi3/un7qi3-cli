// Package cmd wires the cobra command tree for the uq binary.
package cmd

import (
	"github.com/spf13/cobra"

	"github.com/un7qi3inc/un7qi3-cli/internal/cmd/auth"
	"github.com/un7qi3inc/un7qi3-cli/internal/cmd/deploy"
	"github.com/un7qi3inc/un7qi3-cli/internal/cmd/doctor"
	envcmd "github.com/un7qi3inc/un7qi3-cli/internal/cmd/env"
	"github.com/un7qi3inc/un7qi3-cli/internal/cmd/install"
	"github.com/un7qi3inc/un7qi3-cli/internal/cmd/logs"
	"github.com/un7qi3inc/un7qi3-cli/internal/cmd/repo"
	"github.com/un7qi3inc/un7qi3-cli/internal/cmd/skills"
	"github.com/un7qi3inc/un7qi3-cli/internal/cmd/upgrade"
	versioncmd "github.com/un7qi3inc/un7qi3-cli/internal/cmd/version"
)

var (
	flagJSON    bool
	flagVerbose bool
	flagConfig  string
)

const usageTemplate = `사용법:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [명령]{{end}}{{if gt (len .Aliases) 0}}

별칭:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

예시:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

사용 가능한 명령:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

추가 명령:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

플래그:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

전역 플래그:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

추가 도움말:{{range .Commands}}{{if .IsAdditionalHelpTopicRunnable}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

명령별 자세한 도움말은 "{{.CommandPath}} [명령] --help" 를 실행하세요.{{end}}
`

const helpTemplate = `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`

var rootCmd = &cobra.Command{
	Use:   "uq",
	Short: "un7qi3 사내 CLI",
	Long: `uq는 un7qi3 사내 CLI입니다.

온보딩, 레포 셋업, 시크릿, 배포, 운영 작업을 위한 LLM 호출 가능한 결정론적 도구입니다.
주로 Claude Code가 호출하며, 사람도 사용할 수 있도록 친화적인 출력을 제공합니다.`,
	SilenceUsage:  true,
	SilenceErrors: false,
}

func init() {
	rootCmd.SetUsageTemplate(usageTemplate)
	rootCmd.SetHelpTemplate(helpTemplate)

	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "JSON 형식으로 출력")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "상세 로그 출력")
	rootCmd.PersistentFlags().StringVar(&flagConfig, "config", "", "설정 파일 경로")

	rootCmd.SetHelpCommand(newHelpCmd())
	rootCmd.AddCommand(newCompletionCmd())
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.AddCommand(versioncmd.NewCmd())
	rootCmd.AddCommand(doctor.NewCmd())
	rootCmd.AddCommand(install.NewCmd())
	rootCmd.AddCommand(repo.NewCmd())
	rootCmd.AddCommand(auth.NewCmd())
	rootCmd.AddCommand(envcmd.NewCmd())
	rootCmd.AddCommand(deploy.NewCmd())
	rootCmd.AddCommand(logs.NewCmd())
	rootCmd.AddCommand(upgrade.NewCmd())
	rootCmd.AddCommand(skills.NewCmd())

	// -h/--help 플래그 설명을 한글로
	rootCmd.PersistentFlags().BoolP("help", "h", false, "도움말 표시")
}

func newHelpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "help [명령]",
		Short: "명령에 대한 도움말",
		Long:  `지정한 명령에 대한 도움말을 표시합니다. 인자를 생략하면 uq의 도움말을 표시합니다.`,
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
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "지정한 셸의 자동완성 스크립트 생성",
		Long: `지정한 셸의 자동완성 스크립트를 표준 출력으로 생성합니다.

zsh 예시:
  uq completion zsh > "${fpath[1]}/_uq"

bash 예시:
  uq completion bash > /usr/local/etc/bash_completion.d/uq`,
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

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
