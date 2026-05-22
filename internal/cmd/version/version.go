// Package version implements the `uq version` command.
package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/un7qi3inc/un7qi3-cli/internal/output"
	"github.com/un7qi3inc/un7qi3-cli/internal/version"
)

// NewCmd returns the `uq version` command.
func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "uq 버전 표시",
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
}
