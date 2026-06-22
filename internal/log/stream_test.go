package log

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

// fakeSource 는 TailArgs 가 셸로 두 줄을 출력하게 만들어 StreamLines 를 테스트한다.
type fakeSource struct{}

func (fakeSource) Environments(Target) ([]string, error)        { return nil, nil }
func (fakeSource) Instances(Target, string) ([]Instance, error) { return nil, nil }
func (fakeSource) TailArgs(t Target, env string, inst Instance, follow bool, lines int, grep string) []string {
	return []string{"printf", "line-a\\nline-b\\n"}
}

// followSource 는 무한히 라인을 쏟아내는 follow 스트림을 흉내낸다(yes). ctx 취소
// 시 goroutine 이 깔끔히 종료하는지(누수 없음) 검증하는 데 쓴다.
type followSource struct{}

func (followSource) Environments(Target) ([]string, error)        { return nil, nil }
func (followSource) Instances(Target, string) ([]Instance, error) { return nil, nil }
func (followSource) TailArgs(t Target, env string, inst Instance, follow bool, lines int, grep string) []string {
	return []string{"yes", "x"}
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

// 소비자가 더 읽지 않고 ctx 를 취소하면, 무한 follow 스트림의 producer
// goroutine 들이 블록된 send 에 갇히지 않고 깨끗이 종료해 채널이 닫혀야 한다.
// 이 가드(select{ case ch<-x: case <-ctx.Done(): })가 없으면 채널 버퍼가 찬 뒤
// send 가 영구 블록돼 goroutine 이 누수된다. -race 로 함께 검증한다.
func TestStreamLinesCancelStopsGoroutines(t *testing.T) {
	orig := execCommand
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.CommandContext(ctx, args[0], args[1:]...)
	}
	defer func() { execCommand = orig }()

	ctx, cancel := context.WithCancel(context.Background())
	insts := []Instance{{ID: "i-a", Num: 1}, {ID: "i-b", Num: 2}}
	ch := StreamLines(ctx, followSource{}, Target{}, "env", insts, true, 10, "")

	// 몇 줄만 받아 producer 가 실제로 흐르게 한 뒤 소비를 멈추고 취소한다.
	<-ch
	cancel()

	// 취소 후 채널이 (버퍼에 남은 것을 모두 drain 한 뒤) 닫혀야 한다 = 모든
	// producer goroutine 종료 + wg.Wait 완료. 닫히지 않으면 누수.
	done := make(chan struct{})
	go func() {
		for range ch { // drain
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("ctx 취소 후에도 채널이 닫히지 않음 — producer goroutine 누수")
	}
}
