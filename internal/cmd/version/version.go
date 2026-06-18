// Package version implements the `uq version` command.
package version

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/un7qi3inc/un7qi3-cli/internal/output"
	"github.com/un7qi3inc/un7qi3-cli/internal/version"
)

// NewCmd returns the `uq version` command.
func NewCmd() *cobra.Command {
	long := strings.Join([]string{
		output.Desc("uq 바이너리 버전 / 커밋 / 빌드 시각을 표시합니다."),
		"",
		output.Desc("값은 빌드 시점에 ldflags 로 주입됩니다."),
		"",
		output.Heading("예시"),
		output.HelpExample("uq version", "사람 친화 출력"),
		output.HelpExample("uq version --json", "JSON"),
	}, "\n")
	cmd := &cobra.Command{
		Use:   "version",
		Short: "uq 버전 표시",
		Long:  long,
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				return output.WriteJSON(cmd.OutOrStdout(), map[string]string{
					"version": version.Version,
					"commit":  version.Commit,
					"date":    version.Date,
				})
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "uq %s (%s, %s)\n",
				version.Version, version.Commit, version.Date)
			return err
		},
	}
	cmd.Flags().Bool("json", false, "JSON 형식으로 출력")
	return cmd
}
