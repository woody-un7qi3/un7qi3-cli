package issue

import (
	"context"
	"fmt"
	"strings"

	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
)

// createIssue 는 gh 로 이슈를 생성하고 생성된 이슈 URL 을 반환한다.
func createIssue(ctx context.Context, r uqexec.Runner, repo string, f form) (string, error) {
	stdout, stderr, err := r.Run(ctx, "gh", "issue", "create",
		"--repo", repo,
		"--title", f.title,
		"--body", f.body(),
		"--label", f.label(),
	)
	if err != nil {
		msg := strings.TrimSpace(stderr)
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("이슈 생성 실패: %s", msg)
	}
	return strings.TrimSpace(stdout), nil
}
