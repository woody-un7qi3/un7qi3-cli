package auth

import (
	"context"
	"encoding/json"
	"fmt"

	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
)

type awsCallerIdentity struct {
	Account string `json:"Account"`
	Arn     string `json:"Arn"`
	UserID  string `json:"UserId"`
}

// AwsStatus probes `aws sts get-caller-identity` and reports SSO status using
// the package default Runner. The binary-presence guard lives here (not in the
// parse core) so unit tests can drive awsStatus with a fake Runner on hosts
// where aws isn't installed.
func AwsStatus(ctx context.Context) Status {
	if !uqexec.LookPath("aws") {
		return Status{Name: "aws", Error: "aws CLI 설치되지 않음"}
	}
	ctx, cancel := context.WithTimeout(ctx, statusProbeTimeout)
	defer cancel()
	return awsStatus(ctx, defaultRunner)
}

// awsStatus runs the probe through r and parses the result. It assumes aws
// exists (AwsStatus guards that) so it is exercisable with a fake Runner
// independent of the host PATH.
func awsStatus(ctx context.Context, r uqexec.Runner) Status {
	s := Status{Name: "aws"}
	out, _, err := r.Run(ctx, "aws", "sts", "get-caller-identity", "--output", "json")
	if err != nil {
		if msg, ok := probeTimeoutMsg(ctx, "aws", err); ok {
			s.Error = msg
			return s
		}
		s.Error = trimMsg(err.Error())
		return s
	}
	var ci awsCallerIdentity
	if jerr := json.Unmarshal([]byte(out), &ci); jerr != nil {
		s.Error = fmt.Sprintf("응답 파싱 실패: %v", jerr)
		return s
	}
	s.OK = true
	s.Account = ci.Account
	s.Arn = ci.Arn
	s.Detail = ci.Account
	return s
}

// AwsLogin runs `aws sso login` interactively unless the SSO session is valid.
func AwsLogin(ctx context.Context) error {
	s := AwsStatus(ctx)
	if s.OK {
		fmt.Printf("aws: 이미 로그인됨: %s\n", s.Account)
		return nil
	}
	if err := uqexec.RunInteractive("aws", "sso", "login"); err != nil {
		return fmt.Errorf("aws sso login 실패: %w", err)
	}
	return nil
}

// AwsLogout runs `aws sso logout` interactively.
func AwsLogout() error {
	if err := uqexec.RunInteractive("aws", "sso", "logout"); err != nil {
		return fmt.Errorf("aws sso logout 실패: %w", err)
	}
	return nil
}
