package auth

import (
	"context"
	"encoding/json"
	"fmt"

	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
)

type gcloudAccount struct {
	Account string `json:"account"`
	Status  string `json:"status"`
}

// GcloudStatus probes `gcloud auth list --format=json` for an ACTIVE account
// using the package default Runner. The binary-presence guard lives here (not
// in the parse core) so unit tests can drive gcloudStatus with a fake Runner
// on hosts where gcloud isn't installed.
func GcloudStatus() Status {
	if !uqexec.LookPath("gcloud") {
		return Status{Name: "gcloud", Error: "gcloud CLI 설치되지 않음"}
	}
	return gcloudStatus(context.Background(), defaultRunner)
}

// gcloudStatus runs the probe through r and parses the result. It assumes
// gcloud exists (GcloudStatus guards that) so it is exercisable with a fake
// Runner independent of the host PATH.
func gcloudStatus(ctx context.Context, r uqexec.Runner) Status {
	s := Status{Name: "gcloud"}
	out, _, err := r.Run(ctx, "gcloud", "auth", "list", "--format=json")
	if err != nil {
		s.Error = trimMsg(err.Error())
		return s
	}
	var accounts []gcloudAccount
	if jerr := json.Unmarshal([]byte(out), &accounts); jerr != nil {
		s.Error = fmt.Sprintf("응답 파싱 실패: %v", jerr)
		return s
	}
	for _, a := range accounts {
		if a.Status == "ACTIVE" {
			s.OK = true
			s.Account = a.Account
			s.Detail = fmt.Sprintf("%s (active)", a.Account)
			return s
		}
	}
	s.Error = "활성 계정 없음"
	return s
}

// GcloudLogin runs `gcloud auth login` interactively unless an account is active.
func GcloudLogin() error {
	s := GcloudStatus()
	if s.OK {
		fmt.Printf("gcloud: 이미 로그인됨: %s\n", s.Account)
		return nil
	}
	if err := uqexec.RunInteractive("gcloud", "auth", "login"); err != nil {
		return fmt.Errorf("gcloud auth login 실패: %w", err)
	}
	return nil
}

// GcloudLogout revokes all credentials.
func GcloudLogout() error {
	if err := uqexec.RunInteractive("gcloud", "auth", "revoke", "--all"); err != nil {
		return fmt.Errorf("gcloud auth revoke 실패: %w", err)
	}
	return nil
}
