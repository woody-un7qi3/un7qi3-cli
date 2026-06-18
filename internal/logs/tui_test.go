package logs

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func sendKey(m model, s string) model {
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	return nm.(model)
}

func TestModelLogMsgAppends(t *testing.T) {
	m := newModel([]Instance{{Num: 1, ID: "i-a"}}, "")
	nm, _ := m.Update(logMsg{Num: 1, ID: "i-a", Text: "hello", Kind: KindLog})
	if len(nm.(model).buf) != 1 {
		t.Fatalf("buf 1 기대, 실제 %d", len(nm.(model).buf))
	}
}

func TestModelSlashEntersEditing(t *testing.T) {
	m := newModel(nil, "")
	m = sendKey(m, "/")
	if !m.editing {
		t.Error("/ 누르면 편집 모드여야 함")
	}
}

func TestModelSpaceTogglesPause(t *testing.T) {
	m := newModel(nil, "")
	m = sendKey(m, " ")
	if !m.paused {
		t.Error("space 누르면 일시정지")
	}
	m = sendKey(m, " ")
	if m.paused {
		t.Error("다시 누르면 재개")
	}
}

func TestModelDigitTogglesInstance(t *testing.T) {
	m := newModel([]Instance{{Num: 2, ID: "i-b"}}, "")
	m = sendKey(m, "2")
	if !m.hidden[2] {
		t.Error("숫자 키로 인스턴스 숨김")
	}
	m = sendKey(m, "2")
	if m.hidden[2] {
		t.Error("다시 누르면 표시")
	}
}

func TestVisible(t *testing.T) {
	ln := LogLine{Num: 1, ID: "i-a", Text: "x ERROR y", Kind: KindLog}
	if !visible(ln, "", nil) {
		t.Error("필터 없으면 보여야 함")
	}
	if !visible(ln, "error", nil) {
		t.Error("대소문자 무시 부분문자열 매칭")
	}
	if visible(ln, "zzz", nil) {
		t.Error("미매칭이면 숨김")
	}
	if visible(ln, "", map[int]bool{1: true}) {
		t.Error("숨긴 인스턴스는 안 보임")
	}
}

func TestViewContentFiltersAndJoins(t *testing.T) {
	buf := []LogLine{
		{Num: 1, ID: "i-a", Text: "alpha", Kind: KindLog},
		{Num: 2, ID: "i-b", Text: "beta", Kind: KindLog},
	}
	out := viewContent(buf, "alpha", nil)
	if !strings.Contains(out, "alpha") || strings.Contains(out, "beta") {
		t.Errorf("필터 결과 틀림:\n%s", out)
	}
}

func TestAppendBufCaps(t *testing.T) {
	var buf []LogLine
	for i := 0; i < maxBuf+10; i++ {
		buf = appendBuf(buf, LogLine{Num: 1, Text: "x", Kind: KindLog})
	}
	if len(buf) != maxBuf {
		t.Errorf("상한 %d 기대, 실제 %d", maxBuf, len(buf))
	}
}
