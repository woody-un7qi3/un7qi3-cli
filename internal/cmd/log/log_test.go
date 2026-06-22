package log

import (
	"errors"
	"strings"
	"testing"

	authpkg "github.com/un7qi3inc/un7qi3-cli/internal/auth"
)

// TestNewCmdFlagDefaults 는 NewCmd 가 바인딩한 플래그의 이름·단축키·기본값이
// 행위 보존되는지 확인한다(전역 var → logsOptions 전환 후에도 동일).
func TestNewCmdFlagDefaults(t *testing.T) {
	cmd := NewCmd()
	wantDefaults := map[string]string{
		"instance":  "0",
		"lines":     "100",
		"grep":      "",
		"no-follow": "false",
		"split":     "false",
		"dry-run":   "false",
		"plain":     "false",
	}
	for name, def := range wantDefaults {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Fatalf("--%s 플래그가 등록돼야 함", name)
		}
		if f.DefValue != def {
			t.Errorf("--%s 기본값: got %q, want %q", name, f.DefValue, def)
		}
	}
}

// TestNewCmdFlagStateIsolation 은 NewCmd 를 두 번 호출했을 때 한쪽의 플래그
// 파싱 결과가 다른 쪽으로 누수되지 않음을 보인다. 패키지 전역 var 였다면 두
// 인스턴스가 같은 저장소를 공유해 이 테스트가 깨진다.
func TestNewCmdFlagStateIsolation(t *testing.T) {
	c1 := NewCmd()
	if err := c1.Flags().Parse([]string{"--instance", "3", "--grep", "ERROR", "--split"}); err != nil {
		t.Fatalf("c1 파싱 실패: %v", err)
	}

	c2 := NewCmd()
	// c2 는 파싱하지 않았으므로 전부 기본값이어야 한다.
	if got, _ := c2.Flags().GetInt("instance"); got != 0 {
		t.Errorf("c2 instance 가 c1 에서 누수됨: got %d, want 0", got)
	}
	if got, _ := c2.Flags().GetString("grep"); got != "" {
		t.Errorf("c2 grep 가 c1 에서 누수됨: got %q, want \"\"", got)
	}
	if got, _ := c2.Flags().GetBool("split"); got {
		t.Errorf("c2 split 이 c1 에서 누수됨: got true, want false")
	}

	// 역으로 c1 의 값은 그대로 유지돼야 한다.
	if got, _ := c1.Flags().GetInt("instance"); got != 3 {
		t.Errorf("c1 instance 값 손상: got %d, want 3", got)
	}
	if got, _ := c1.Flags().GetString("grep"); got != "ERROR" {
		t.Errorf("c1 grep 값 손상: got %q, want \"ERROR\"", got)
	}
}

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
