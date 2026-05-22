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

var rootCmd = &cobra.Command{
	Use:   "uq",
	Short: "un7qi3 internal CLI",
	Long: `uq is the un7qi3 internal CLI.

An LLM-callable, deterministic tool for onboarding, repo setup, secrets,
deploys, and ops. Designed to be driven primarily by Claude Code, with
human-friendly output as a fallback.`,
	SilenceUsage:  true,
	SilenceErrors: false,
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "output as JSON")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVar(&flagConfig, "config", "", "config file path")

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
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
