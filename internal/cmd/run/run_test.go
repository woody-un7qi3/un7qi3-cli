package run

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/un7qi3inc/un7qi3-cli/internal/clierr"
	"github.com/un7qi3inc/un7qi3-cli/internal/repocfg"
)

// splitTarget 은 정상 입력("repo", "repo:profile")을 기존과 동일하게 가르고,
// 콜론을 썼는데 한쪽이 빈 모호한 입력은 InvalidArgError(usage, exit 2)로 막는다.
func TestSplitTarget(t *testing.T) {
	tests := []struct {
		name        string
		in          string
		wantRepo    string
		wantProfile string
		wantErr     bool
	}{
		{"repo-only", "forceteller-app", "forceteller-app", "", false},
		{"repo-and-profile", "forceteller-app:app3", "forceteller-app", "app3", false},
		{"profile-with-extra-colon", "repo:a:b", "repo", "a:b", false},
		{"empty-profile", "repo:", "", "", true},
		{"empty-repo", ":profile", "", "", true},
		{"only-colon", ":", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, profile, err := splitTarget(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("splitTarget(%q) = (%q, %q, nil), want error", tt.in, repo, profile)
				}
				var invalid clierr.InvalidArgError
				if !errors.As(err, &invalid) {
					t.Fatalf("splitTarget(%q) err = %T, want clierr.InvalidArgError", tt.in, err)
				}
				if got := clierr.Classify(err); got != 2 {
					t.Errorf("Classify = %d, want 2", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("splitTarget(%q) unexpected err: %v", tt.in, err)
			}
			if repo != tt.wantRepo || profile != tt.wantProfile {
				t.Errorf("splitTarget(%q) = (%q, %q), want (%q, %q)", tt.in, repo, profile, tt.wantRepo, tt.wantProfile)
			}
		})
	}
}

// execSingle 는 자식 프로세스의 비정상 종료 코드를 os.Exit 가 아니라
// clierr.ExitCodeError 로 전파해야 한다 — 그래야 RunE 의 defer 가 실행되고
// main 이 자식과 동일한 종료 코드를 미러링할 수 있다.
func TestExecSingle_PropagatesChildExitCode(t *testing.T) {
	err := execSingle(context.Background(), t.TempDir(), []string{"sh", "-c", "exit 7"}, nil)
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
	if err := execSingle(context.Background(), t.TempDir(), []string{"sh", "-c", "exit 0"}, nil); err != nil {
		t.Fatalf("정상 종료에 nil 을 기대했지만: %v", err)
	}
}

// 실행 자체가 불가능하면(바이너리 없음) ExitCodeError 가 아니라 시작 실패
// 에러를 반환한다 — 이건 사용법/런타임 일반 에러로 classify 되어야 한다.
func TestExecSingle_StartFailureIsPlainError(t *testing.T) {
	err := execSingle(context.Background(), t.TempDir(), []string{"this-binary-does-not-exist-xyz"}, nil)
	if err == nil {
		t.Fatal("시작 실패에 에러를 기대했지만 nil")
	}
	var coded clierr.ExitCodeError
	if errors.As(err, &coded) {
		t.Errorf("시작 실패는 ExitCodeError 가 아니어야 한다: %v", err)
	}
}

// ctx 취소는 자식 프로세스 그룹에 SIGINT 를 전파해 장기 실행 자식을 정리하고,
// execSingle 이 블록되지 않고 빠져나와야 한다. 가드가 없으면 sleep 가 끝날 때까지
// (또는 영원히) 매달려 goroutine·프로세스가 누수된다. -race 로 함께 검증한다.
func TestExecSingle_CancelTerminatesChild(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	done := make(chan error, 1)
	go func() {
		// 30초 sleep — 취소가 자식을 죽이지 못하면 테스트 타임아웃까지 매달린다.
		done <- execSingle(ctx, t.TempDir(), []string{"sh", "-c", "sleep 30"}, nil)
	}()

	select {
	case <-done:
		// 취소로 자식이 SIGINT 받고 종료 → execSingle 반환. 성공.
	case <-time.After(5 * time.Second):
		t.Fatal("ctx 취소 후에도 execSingle 이 반환하지 않음 — 자식 정리 누락")
	}
}

// execMulti 도 동일하게 ctx 취소 시 모든 자식 그룹을 정리하고 반환해야 한다.
func TestExecMulti_CancelTerminatesChildren(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	procs := []repocfg.Proc{
		{Name: "a", Cmd: []string{"sh", "-c", "sleep 30"}},
		{Name: "b", Cmd: []string{"sh", "-c", "sleep 30"}},
	}
	done := make(chan error, 1)
	go func() { done <- execMulti(ctx, t.TempDir(), procs, nil) }()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("ctx 취소 후에도 execMulti 가 반환하지 않음 — 자식 정리 누락")
	}
}
