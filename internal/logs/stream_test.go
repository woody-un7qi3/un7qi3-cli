package logs

import (
	"context"
	"os/exec"
	"testing"
)

// fakeSource 는 TailArgs 가 셸로 두 줄을 출력하게 만들어 StreamLines 를 테스트한다.
type fakeSource struct{}

func (fakeSource) Environments(Target) ([]string, error)        { return nil, nil }
func (fakeSource) Instances(Target, string) ([]Instance, error) { return nil, nil }
func (fakeSource) TailArgs(t Target, env string, inst Instance, follow bool, lines int, grep string) []string {
	return []string{"printf", "line-a\\nline-b\\n"}
}

func TestStreamLinesEmitsLines(t *testing.T) {
	// execCommand 를 sh 로 바꿔 args 를 그대로 실행(첫 인자=프로그램).
	orig := execCommand
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, args[0], args[1:]...)
	}
	defer func() { execCommand = orig }()

	insts := []Instance{{ID: "i-aaa", Num: 1}}
	ch := StreamLines(context.Background(), fakeSource{}, Target{}, "env", insts, false, 10, "")

	var texts []string
	for ln := range ch {
		if ln.Kind == KindLog {
			texts = append(texts, ln.Text)
			if ln.Num != 1 || ln.ID != "i-aaa" {
				t.Errorf("LogLine 메타 틀림: %+v", ln)
			}
		}
	}
	if len(texts) != 2 || texts[0] != "line-a" || texts[1] != "line-b" {
		t.Errorf("방출 라인 틀림: %v", texts)
	}
}
