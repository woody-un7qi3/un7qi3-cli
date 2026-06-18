package logs

import (
	"errors"
	"strings"
	"testing"

	authpkg "github.com/un7qi3inc/un7qi3-cli/internal/auth"
)

// TestCheckLogsPreconditions 는 logs 실행 전 외부 도구 게이트의 분기를 검증한다.
// aws 는 발견 단계에 항상 필수, eb 는 스트리밍에만 쓰여 dry-run 이면 면제된다.
func TestCheckLogsPreconditions(t *testing.T) {
	okAWS := authpkg.Status{Name: "aws", OK: true}
	failAWS := authpkg.Status{Name: "aws", OK: false}

	tests := []struct {
		name      string
		aws       authpkg.Status
		dryRun    bool
		hasEB     bool
		wantErr   bool
		msgSubstr string // wantErr 일 때 메시지에 포함돼야 할 토큰
	}{
		{name: "aws 미인증이면 dry-run 이어도 에러", aws: failAWS, dryRun: true, hasEB: true, wantErr: true, msgSubstr: "aws"},
		{name: "aws OK + dry-run + eb 없음 → 통과(eb 면제)", aws: okAWS, dryRun: true, hasEB: false, wantErr: false},
		{name: "aws OK + 실행 + eb 없음 → 에러(brew 안내)", aws: okAWS, dryRun: false, hasEB: false, wantErr: true, msgSubstr: "brew install aws-elasticbeanstalk"},
		{name: "aws OK + 실행 + eb 있음 → 통과", aws: okAWS, dryRun: false, hasEB: true, wantErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := checkLogsPreconditions(tc.aws, tc.dryRun, tc.hasEB)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("에러를 기대했지만 nil")
				}
				var re *authpkg.RequiredError
				if !errors.As(err, &re) {
					t.Fatalf("RequiredError 를 기대했지만 %T", err)
				}
				if tc.msgSubstr != "" && !strings.Contains(err.Error(), tc.msgSubstr) {
					t.Fatalf("메시지에 %q 가 포함돼야 하는데: %q", tc.msgSubstr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("통과를 기대했지만 에러: %v", err)
			}
		})
	}
}
