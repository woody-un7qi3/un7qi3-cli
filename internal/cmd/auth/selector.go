package auth

import (
	"fmt"

	"github.com/spf13/cobra"

	authpkg "github.com/un7qi3inc/un7qi3-cli/internal/auth"
)

// addProviderFlags registers the --gh-only/--aws-only/--gcloud-only
// (mutually exclusive) and --skip-gh/--skip-aws/--skip-gcloud flag set on cmd.
// Login/logout/status share the same selection semantics.
func addProviderFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("gh-only", false, "gh만 처리")
	cmd.Flags().Bool("aws-only", false, "aws만 처리")
	cmd.Flags().Bool("gcloud-only", false, "gcloud만 처리")
	cmd.MarkFlagsMutuallyExclusive("gh-only", "aws-only", "gcloud-only")

	cmd.Flags().Bool("skip-gh", false, "gh 제외")
	cmd.Flags().Bool("skip-aws", false, "aws 제외")
	cmd.Flags().Bool("skip-gcloud", false, "gcloud 제외")

	cmd.MarkFlagsMutuallyExclusive("gh-only", "skip-gh")
	cmd.MarkFlagsMutuallyExclusive("gh-only", "skip-aws")
	cmd.MarkFlagsMutuallyExclusive("gh-only", "skip-gcloud")
	cmd.MarkFlagsMutuallyExclusive("aws-only", "skip-gh")
	cmd.MarkFlagsMutuallyExclusive("aws-only", "skip-aws")
	cmd.MarkFlagsMutuallyExclusive("aws-only", "skip-gcloud")
	cmd.MarkFlagsMutuallyExclusive("gcloud-only", "skip-gh")
	cmd.MarkFlagsMutuallyExclusive("gcloud-only", "skip-aws")
	cmd.MarkFlagsMutuallyExclusive("gcloud-only", "skip-gcloud")
}

// selectProviders resolves the flag set into the canonical-ordered list of
// providers to act on.
func selectProviders(cmd *cobra.Command) ([]string, error) {
	ghOnly, _ := cmd.Flags().GetBool("gh-only")
	awsOnly, _ := cmd.Flags().GetBool("aws-only")
	gcloudOnly, _ := cmd.Flags().GetBool("gcloud-only")
	switch {
	case ghOnly:
		return []string{"gh"}, nil
	case awsOnly:
		return []string{"aws"}, nil
	case gcloudOnly:
		return []string{"gcloud"}, nil
	}

	skipGh, _ := cmd.Flags().GetBool("skip-gh")
	skipAws, _ := cmd.Flags().GetBool("skip-aws")
	skipGcloud, _ := cmd.Flags().GetBool("skip-gcloud")

	out := make([]string, 0, 3)
	for _, name := range authpkg.ProviderNames {
		switch name {
		case "gh":
			if skipGh {
				continue
			}
		case "aws":
			if skipAws {
				continue
			}
		case "gcloud":
			if skipGcloud {
				continue
			}
		}
		out = append(out, name)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("모든 provider가 제외되어 처리할 대상이 없습니다")
	}
	return out, nil
}
