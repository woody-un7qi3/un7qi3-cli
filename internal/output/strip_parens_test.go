package output

import (
	"reflect"
	"testing"
)

// go test 는 stdout 이 TTY 가 아니라 색상이 꺼진다 → Dim 은 no-op 이므로
// DimParenHints 는 괄호만 제거하고 내부 텍스트를 평문으로 돌려준다.
func TestDimParenHintsStripsParensWhenNoColor(t *testing.T) {
	cases := map[string]string{
		"인증 관리 (gh, aws, gcloud)":        "인증 관리 gh, aws, gcloud",
		"uq 최초 설정 (인증 점검 + 워크스페이스 위치)": "uq 최초 설정 인증 점검 + 워크스페이스 위치",
		"이슈 작성 (기능 요청 / 버그 리포트)":       "이슈 작성 기능 요청 / 버그 리포트",
	}
	for in, want := range cases {
		if got := DimParenHints(in); got != want {
			t.Errorf("DimParenHints(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDimParenHintsMultipleHints(t *testing.T) {
	got := DimParenHints("앞 (하나) 뒤 (둘)")
	if want := "앞 하나 뒤 둘"; got != want {
		t.Errorf("DimParenHints = %q, want %q", got, want)
	}
}

func TestDimParenHintsNoParensUnchanged(t *testing.T) {
	in := "괄호 없는 설명"
	if got := DimParenHints(in); got != in {
		t.Errorf("DimParenHints(%q) = %q, want unchanged", in, got)
	}
}

func TestParenHintReMatchesEachHint(t *testing.T) {
	got := parenHintRe.FindAllString("인증 관리 (gh, aws, gcloud) 또는 (추가)", -1)
	want := []string{"(gh, aws, gcloud)", "(추가)"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("matches = %v, want %v", got, want)
	}
}
