// Package deploy implements `uq deploy ...` commands.
package deploy

import "github.com/spf13/cobra"

// NewCmd returns the `uq deploy` parent command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: ".uq.yml에 정의된 배포 워크플로 실행",
	}
	cmd.AddCommand(newRunCmd())
	return cmd
}
