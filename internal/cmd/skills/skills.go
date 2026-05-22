// Package skills implements the `uq skills ...` commands (Phase 3 stub).
package skills

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewCmd returns the `uq skills` parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skills",
		Short: "uq에 포함된 Claude Code 스킬 설치",
	}
	cmd.AddCommand(newInstallCmd())
	return cmd
}

func newInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "포함된 Claude Code 스킬을 ~/.claude/skills/에 설치",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "Phase 3 stub: 아직 구현되지 않음")
			return nil
		},
	}
}
