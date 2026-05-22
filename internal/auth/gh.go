package auth

import (
	"fmt"
	"regexp"
	"strings"

	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
)

var ghUserRe = regexp.MustCompile(`(?:Logged in to [^\s]+ (?:as|account) |account )(\S+)`)

// GhStatus probes `gh auth status` and reports authentication state.
func GhStatus() Status {
	s := Status{Name: "gh"}
	if !uqexec.LookPath("gh") {
		s.Error = "gh CLI 설치되지 않음"
		return s
	}
	// gh auth status writes its human report to stderr regardless of exit
	// code. Use CombinedOutput so we can scan whichever stream gh chose.
	out, err := uqexec.RunCombined("gh", "auth", "status")
	text := string(out)
	if err != nil {
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
func GhLogin() error {
	s := GhStatus()
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
