package auth

import (
	"encoding/json"
	"fmt"

	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
)

type awsCallerIdentity struct {
	Account string `json:"Account"`
	Arn     string `json:"Arn"`
	UserID  string `json:"UserId"`
}

// AwsStatus probes `aws sts get-caller-identity` and reports SSO status.
func AwsStatus() Status {
	s := Status{Name: "aws"}
	if !uqexec.LookPath("aws") {
		s.Error = "aws CLI 설치되지 않음"
		return s
	}
	out, err := uqexec.Run("aws", "sts", "get-caller-identity", "--output", "json")
	if err != nil {
		s.Error = trimMsg(err.Error())
		return s
	}
	var ci awsCallerIdentity
	if jerr := json.Unmarshal(out, &ci); jerr != nil {
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
func AwsLogin() error {
	s := AwsStatus()
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
