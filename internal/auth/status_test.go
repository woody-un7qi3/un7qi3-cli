package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	uqexec "github.com/un7qi3inc/un7qi3-cli/internal/exec"
)

// fakeRunner is a test double for uqexec.Runner. It ignores the command and
// returns canned stdout/stderr/err, recording the last invocation so tests can
// assert the probe issued the expected command.
type fakeRunner struct {
	stdout string
	stderr string
	err    error

	gotName string
	gotArgs []string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, string, error) {
	f.gotName = name
	f.gotArgs = args
	return f.stdout, f.stderr, f.err
}

func TestGhStatus_AuthenticatedExtractsUser(t *testing.T) {
	// gh writes its human report to stderr on success; user line varies by
	// gh version ("Logged in to github.com as woody" / "account woody").
	r := &fakeRunner{stderr: "github.com\n  ✓ Logged in to github.com account woody (keyring)\n"}
	s := ghStatus(context.Background(), r)
	if !s.OK {
		t.Fatalf("OK = false, want true; err=%q", s.Error)
	}
	if s.User != "woody" {
		t.Errorf("User = %q, want %q", s.User, "woody")
	}
	if !strings.Contains(s.Detail, "woody") {
		t.Errorf("Detail = %q, want it to mention user", s.Detail)
	}
	if r.gotName != "gh" || strings.Join(r.gotArgs, " ") != "auth status" {
		t.Errorf("실행 명령 = %q %q, want gh auth status", r.gotName, r.gotArgs)
	}
}

func TestGhStatus_UnauthenticatedReportsError(t *testing.T) {
	// Unauthenticated → non-zero exit; message lives in the combined output.
	r := &fakeRunner{
		stderr: "You are not logged into any GitHub hosts. Run gh auth login to authenticate.\n",
		err:    errors.New("exit status 1"),
	}
	s := ghStatus(context.Background(), r)
	if s.OK {
		t.Fatalf("OK = true, want false")
	}
	if s.User != "" {
		t.Errorf("User = %q, want empty", s.User)
	}
	if !strings.Contains(s.Error, "not logged into") {
		t.Errorf("Error = %q, want unauthenticated message", s.Error)
	}
}

func TestGhStatus_AuthenticatedNoUserLineStillOK(t *testing.T) {
	r := &fakeRunner{stderr: "github.com\n  ✓ Token: gho_xxxx\n"}
	s := ghStatus(context.Background(), r)
	if !s.OK {
		t.Fatalf("OK = false, want true (인증됐지만 user 라인 없음)")
	}
	if s.User != "" {
		t.Errorf("User = %q, want empty", s.User)
	}
	if s.Detail != "인증됨" {
		t.Errorf("Detail = %q, want %q", s.Detail, "인증됨")
	}
}

func TestAwsStatus_ParsesCallerIdentity(t *testing.T) {
	r := &fakeRunner{stdout: `{"Account":"123456789012","Arn":"arn:aws:iam::123456789012:user/woody","UserId":"AIDA..."}`}
	s := awsStatus(context.Background(), r)
	if !s.OK {
		t.Fatalf("OK = false, want true; err=%q", s.Error)
	}
	if s.Account != "123456789012" {
		t.Errorf("Account = %q, want %q", s.Account, "123456789012")
	}
	if s.Arn != "arn:aws:iam::123456789012:user/woody" {
		t.Errorf("Arn = %q", s.Arn)
	}
	if s.Detail != "123456789012" {
		t.Errorf("Detail = %q, want account id", s.Detail)
	}
}

func TestAwsStatus_ExecErrorReportsTrimmed(t *testing.T) {
	// A multi-line SSO-expired error must be trimmed to its first line.
	r := &fakeRunner{err: errors.New("aws sts get-caller-identity --output json: Token has expired and refresh failed\nsecond line should drop")}
	s := awsStatus(context.Background(), r)
	if s.OK {
		t.Fatalf("OK = true, want false")
	}
	if strings.Contains(s.Error, "\n") {
		t.Errorf("Error should be single line: %q", s.Error)
	}
	if !strings.Contains(s.Error, "Token has expired") {
		t.Errorf("Error = %q, want expiry message", s.Error)
	}
}

func TestAwsStatus_InvalidJSONReportsParseError(t *testing.T) {
	r := &fakeRunner{stdout: "not json at all"}
	s := awsStatus(context.Background(), r)
	if s.OK {
		t.Fatalf("OK = true, want false")
	}
	if !strings.HasPrefix(s.Error, "응답 파싱 실패") {
		t.Errorf("Error = %q, want parse-failure prefix", s.Error)
	}
}

func TestGcloudStatus_PicksActiveAccount(t *testing.T) {
	r := &fakeRunner{stdout: `[{"account":"inactive@example.com","status":""},{"account":"woody@un7qi3.co","status":"ACTIVE"}]`}
	s := gcloudStatus(context.Background(), r)
	if !s.OK {
		t.Fatalf("OK = false, want true; err=%q", s.Error)
	}
	if s.Account != "woody@un7qi3.co" {
		t.Errorf("Account = %q, want active account", s.Account)
	}
	if !strings.Contains(s.Detail, "active") {
		t.Errorf("Detail = %q, want it to note active", s.Detail)
	}
}

func TestGcloudStatus_NoActiveAccount(t *testing.T) {
	r := &fakeRunner{stdout: `[{"account":"woody@un7qi3.co","status":""}]`}
	s := gcloudStatus(context.Background(), r)
	if s.OK {
		t.Fatalf("OK = true, want false (활성 계정 없음)")
	}
	if s.Error != "활성 계정 없음" {
		t.Errorf("Error = %q, want %q", s.Error, "활성 계정 없음")
	}
}

func TestGcloudStatus_InvalidJSONReportsParseError(t *testing.T) {
	r := &fakeRunner{stdout: "}{ broken"}
	s := gcloudStatus(context.Background(), r)
	if s.OK {
		t.Fatalf("OK = true, want false")
	}
	if !strings.HasPrefix(s.Error, "응답 파싱 실패") {
		t.Errorf("Error = %q, want parse-failure prefix", s.Error)
	}
}

// With an empty PATH (no gh/aws/gcloud), the public probes must still report
// the "미설치" guard — behavior preservation. uqexec.LookPath consults $PATH,
// so clearing it simulates an unmanaged host. The parse cores above stay
// PATH-independent; only this guard depends on the binary being present.
func TestStatus_MissingBinaryReportsNotInstalled(t *testing.T) {
	t.Setenv("PATH", "")
	for _, tc := range []struct {
		probe   func(context.Context) Status
		name    string
		wantErr string
	}{
		{GhStatus, "gh", "gh CLI 설치되지 않음"},
		{AwsStatus, "aws", "aws CLI 설치되지 않음"},
		{GcloudStatus, "gcloud", "gcloud CLI 설치되지 않음"},
	} {
		s := tc.probe(context.Background())
		if s.OK {
			t.Errorf("%s: OK = true, want false (미설치)", tc.name)
		}
		if s.Error != tc.wantErr {
			t.Errorf("%s: Error = %q, want %q", tc.name, s.Error, tc.wantErr)
		}
	}
}

// deadlineRunner simulates a probe whose external command does not return
// before the context deadline — it blocks on ctx.Done() and reports the
// context error, exactly as OSRunner does when exec.CommandContext kills a
// timed-out child. It lets us drive the timeout path without spawning a real
// slow process.
type deadlineRunner struct{}

func (deadlineRunner) Run(ctx context.Context, _ string, _ ...string) (string, string, error) {
	<-ctx.Done()
	return "", "", ctx.Err()
}

// When the probe context is already past its deadline, each *Status core must
// report the explicit "타임아웃" reason (OK=false) rather than hang or surface a
// raw "context deadline exceeded". This is the new behavior the timeout work
// introduces — fast paths are unchanged, only the hang turns into a clean fail.
func TestStatus_ContextDeadlineReportsTimeout(t *testing.T) {
	for _, tc := range []struct {
		name  string
		probe func(context.Context, uqexec.Runner) Status
	}{
		{"gh", ghStatus},
		{"aws", awsStatus},
		{"gcloud", gcloudStatus},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		s := tc.probe(ctx, deadlineRunner{})
		cancel()
		if s.OK {
			t.Errorf("%s: OK = true, want false on timeout", tc.name)
		}
		if !strings.Contains(s.Error, "타임아웃") {
			t.Errorf("%s: Error = %q, want it to mention 타임아웃", tc.name, s.Error)
		}
	}
}
