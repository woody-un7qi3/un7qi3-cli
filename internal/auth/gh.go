package auth

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
)

var ghUserRe = regexp.MustCompile(`(?:Logged in to [^\s]+ (?:as|account) |account )(\S+)`)

// GhStatus probes `gh auth status` and reports authentication state using the
// package default Runner. The binary-presence guard lives here (not in the
// parse core) so unit tests can drive ghStatus with a fake Runner on hosts
// where gh isn't installed.
func GhStatus(ctx context.Context) Status {
	if !uqexec.LookPath("gh") {
		return Status{Name: "gh", Error: "gh CLI 설치되지 않음"}
	}
	ctx, cancel := context.WithTimeout(ctx, statusProbeTimeout)
	defer cancel()
	return ghStatus(ctx, defaultRunner)
}

// ghStatus runs the probe through r and parses the result. It assumes gh
// exists (GhStatus guards that) so it is exercisable with a fake Runner
// independent of the host PATH.
func ghStatus(ctx context.Context, r uqexec.Runner) Status {
	s := Status{Name: "gh"}
	// gh auth status writes its human report to stderr regardless of exit
	// code. Scan whichever stream gh chose by merging stdout and stderr.
	stdout, stderr, err := r.Run(ctx, "gh", "auth", "status")
	text := stdout + stderr
	if err != nil {
		if msg, ok := probeTimeoutMsg(ctx, "gh", err); ok {
			s.Error = msg
			return s
		}
		// Unauthenticated → non-zero exit; message is in combined output.
		s.Error = trimMsg(text)
		return s
	}
	if m := ghUserRe.FindStringSubmatch(text); len(m) >= 2 {
		s.OK = true
		s.User = m[1]
		s.Detail = fmt.Sprintf("%s 으로 인증됨", m[1])
		return s
	}
	// Authenticated but no recognizable user line — still treat as OK.
	s.OK = true
	s.Detail = "인증됨"
	return s
}

// GhLogin runs `gh auth login` interactively unless already authenticated.
// On a fresh login it also runs `gh auth setup-git`.
func GhLogin(ctx context.Context) error {
	s := GhStatus(ctx)
	if s.OK {
		who := s.User
		if who == "" {
			who = "(unknown)"
		}
		fmt.Printf("gh: 이미 로그인됨: %s\n", who)
		return nil
	}
	if err := uqexec.RunInteractive("gh", "auth", "login"); err != nil {
		return fmt.Errorf("gh auth login 실패: %w", err)
	}
	if err := GhSetupGit(); err != nil {
		return err
	}
	return nil
}

// GhSetupGit configures git to use gh credentials for HTTPS git operations.
func GhSetupGit() error {
	if err := uqexec.RunInteractive("gh", "auth", "setup-git"); err != nil {
		return fmt.Errorf("gh auth setup-git 실패: %w", err)
	}
	return nil
}

// GhLogout runs `gh auth logout` interactively.
func GhLogout() error {
	if err := uqexec.RunInteractive("gh", "auth", "logout"); err != nil {
		return fmt.Errorf("gh auth logout 실패: %w", err)
	}
	return nil
}

// trimMsg shortens long stderr-derived error strings for status display.
func trimMsg(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "\n"); i >= 0 {
		s = s[:i]
	}
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}
