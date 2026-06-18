package logs

import (
	"regexp"
	"strings"
	"testing"
)

func TestRenderLegend(t *testing.T) {
	insts := []Instance{{ID: "i-aaa", Num: 1}, {ID: "i-bbb", Num: 2}}
	out := RenderLegend(insts)
	if !strings.Contains(out, "#1") || !strings.Contains(out, "i-aaa") ||
		!strings.Contains(out, "#2") || !strings.Contains(out, "i-bbb") {
		t.Errorf("범례에 #k/인스턴스 id 가 보여야 함:\n%s", out)
	}
}

func TestPrefixLine(t *testing.T) {
	got := PrefixLine(3, "hello")
	if !strings.Contains(got, "#3") || !strings.Contains(got, "hello") {
		t.Errorf("prefix 결과: %q", got)
	}
}

func TestGrepMatch(t *testing.T) {
	re := regexp.MustCompile("ERROR")
	if !GrepMatch(re, "x ERROR y") || GrepMatch(re, "ok") {
		t.Error("정규식 매칭 동작 틀림")
	}
	if !GrepMatch(nil, "anything") {
		t.Error("nil 정규식은 항상 통과")
	}
}

func TestRenderLine(t *testing.T) {
	got := renderLine(LogLine{Num: 1, ID: "i-0abc123", Text: "hello", Kind: KindLog})
	if !strings.Contains(got, "#1") || !strings.Contains(got, "hello") {
		t.Errorf("KindLog 렌더 틀림: %q", got)
	}
	ce := renderLine(LogLine{Num: 2, ID: "i-x", Text: "boom", Kind: KindConnErr})
	if !strings.Contains(ce, "접속 실패") || !strings.Contains(ce, "boom") {
		t.Errorf("KindConnErr 렌더 틀림: %q", ce)
	}
}
