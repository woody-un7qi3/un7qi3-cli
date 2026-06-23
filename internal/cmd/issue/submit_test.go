package issue

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeRunner 는 마지막 호출 argv 를 기록하고 고정 응답을 돌려준다.
type fakeRunner struct {
	gotName string
	gotArgs []string
	stdout  string
	stderr  string
	err     error
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, string, error) {
	f.gotName = name
	f.gotArgs = args
	return f.stdout, f.stderr, f.err
}

func TestCreateIssueArgs(t *testing.T) {
	fr := &fakeRunner{stdout: "https://github.com/owner/repo/issues/1\n"}
	f := form{kind: kindFeature, title: "새 명령 제안", problem: "p", proposal: "q"}

	url, err := createIssue(context.Background(), fr, "owner/repo", f)
	if err != nil {
		t.Fatalf("createIssue err: %v", err)
	}
	if url != "https://github.com/owner/repo/issues/1" {
		t.Errorf("url = %q (trim 안 됨?)", url)
	}
	if fr.gotName != "gh" {
		t.Errorf("name = %q, want gh", fr.gotName)
	}
	joined := strings.Join(fr.gotArgs, " ")
	for _, want := range []string{"issue create", "--repo owner/repo", "--title 새 명령 제안", "--body", "--label enhancement"} {
		if !strings.Contains(joined, want) {
			t.Errorf("argv missing %q\nargv: %v", want, fr.gotArgs)
		}
	}
}

func TestCreateIssueError(t *testing.T) {
	fr := &fakeRunner{stderr: "gh: not authenticated", err: errors.New("exit 1")}
	f := form{kind: kindFeature, title: "새 명령 제안", problem: "p", proposal: "q"}

	url, err := createIssue(context.Background(), fr, "owner/repo", f)
	if err == nil {
		t.Fatal("createIssue err = nil, want error")
	}
	if url != "" {
		t.Errorf("url = %q, want empty", url)
	}
	if !strings.Contains(err.Error(), "이슈 생성 실패:") {
		t.Errorf("err = %q, want prefix \"이슈 생성 실패:\"", err)
	}
	if !strings.Contains(err.Error(), "gh: not authenticated") {
		t.Errorf("err = %q, want stderr content", err)
	}
}
