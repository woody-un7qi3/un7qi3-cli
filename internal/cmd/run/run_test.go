package run

import (
	"errors"
	"testing"

	"github.com/un7qi3inc/un7qi3-cli/internal/clierr"
)

// execSingle 는 자식 프로세스의 비정상 종료 코드를 os.Exit 가 아니라
// clierr.ExitCodeError 로 전파해야 한다 — 그래야 RunE 의 defer 가 실행되고
// main 이 자식과 동일한 종료 코드를 미러링할 수 있다.
func TestExecSingle_PropagatesChildExitCode(t *testing.T) {
	err := execSingle(t.TempDir(), []string{"sh", "-c", "exit 7"}, nil)
	if err == nil {
		t.Fatal("비정상 종료에 에러를 기대했지만 nil")
	}
	var coded clierr.ExitCodeError
	if !errors.As(err, &coded) {
		t.Fatalf("ExitCodeError 를 기대했지만 %T (%v)", err, err)
	}
	if coded.Code != 7 {
		t.Errorf("Code = %d, want 7", coded.Code)
	}
	if got := clierr.Classify(err); got != 7 {
		t.Errorf("Classify = %d, want 7", got)
	}
}

// 정상 종료(코드 0)는 nil 을 반환한다.
func TestExecSingle_SuccessReturnsNil(t *testing.T) {
	if err := execSingle(t.TempDir(), []string{"sh", "-c", "exit 0"}, nil); err != nil {
		t.Fatalf("정상 종료에 nil 을 기대했지만: %v", err)
	}
}

// 실행 자체가 불가능하면(바이너리 없음) ExitCodeError 가 아니라 시작 실패
// 에러를 반환한다 — 이건 사용법/런타임 일반 에러로 classify 되어야 한다.
func TestExecSingle_StartFailureIsPlainError(t *testing.T) {
	err := execSingle(t.TempDir(), []string{"this-binary-does-not-exist-xyz"}, nil)
	if err == nil {
		t.Fatal("시작 실패에 에러를 기대했지만 nil")
	}
	var coded clierr.ExitCodeError
	if errors.As(err, &coded) {
		t.Errorf("시작 실패는 ExitCodeError 가 아니어야 한다: %v", err)
	}
}
