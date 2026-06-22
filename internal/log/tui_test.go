package log

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func sendKey(m model, s string) model {
	nm, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	return nm.(model)
}

func TestModelLogMsgAppends(t *testing.T) {
	m := newModel([]Lane{{Num: 1, Toggle: "i-a"}}, "", "")
	nm, _ := m.Update(logMsg{Num: 1, ID: "i-a", Text: "hello", Kind: KindLog})
	if len(nm.(model).buf) != 1 {
		t.Fatalf("buf 1 기대, 실제 %d", len(nm.(model).buf))
	}
}

func TestModelSlashEntersEditing(t *testing.T) {
	m := newModel(nil, "", "")
	m = sendKey(m, "/")
	if !m.editing {
		t.Error("/ 누르면 편집 모드여야 함")
	}
}

func TestModelSpaceTogglesPause(t *testing.T) {
	m := newModel(nil, "", "")
	m = sendKey(m, " ")
	if !m.paused {
		t.Error("space 누르면 일시정지")
	}
	m = sendKey(m, " ")
	if m.paused {
		t.Error("다시 누르면 재개")
	}
}

func TestStatusStylePausedIsRed(t *testing.T) {
	red := lipgloss.Color("1")
	if got := statusStyle(true).GetForeground(); got != red {
		t.Errorf("일시정지 전경색 = %v, want 빨강(%v)", got, red)
	}
	if got := statusStyle(false).GetForeground(); got == red {
		t.Error("실시간 상태는 빨강이 아니어야 함")
	}
}

func TestModelDigitSolosInstance(t *testing.T) {
	m := newModel([]Lane{{Num: 2, Toggle: "i-b"}}, "", "")
	m = sendKey(m, "2")
	if m.solo != 2 {
		t.Errorf("숫자 키로 솔로(#2) 기대, 실제 solo=%d", m.solo)
	}
	m = sendKey(m, "2")
	if m.solo != 0 {
		t.Errorf("같은 번호 다시 → 전체 복귀(solo=0) 기대, 실제 %d", m.solo)
	}
}

func TestVisible(t *testing.T) {
	ln := LogLine{Num: 1, ID: "i-a", Text: "x ERROR y", Kind: KindLog}
	if !visible(ln, "", 0) {
		t.Error("필터·솔로 없으면 보여야 함")
	}
	if !visible(ln, "error", 0) {
		t.Error("대소문자 무시 부분문자열 매칭")
	}
	if visible(ln, "zzz", 0) {
		t.Error("미매칭이면 숨김")
	}
	if !visible(ln, "", 1) {
		t.Error("솔로가 자기 번호면 보여야 함")
	}
	if visible(ln, "", 2) {
		t.Error("솔로가 다른 번호면 안 보임")
	}
}

func TestViewContentFiltersAndJoins(t *testing.T) {
	buf := []LogLine{
		{Num: 1, ID: "i-a", Text: "alpha", Kind: KindLog},
		{Num: 2, ID: "i-b", Text: "beta", Kind: KindLog},
	}
	out := viewContent(buf, "alpha", 0)
	if !strings.Contains(out, "alpha") || strings.Contains(out, "beta") {
		t.Errorf("필터 결과 틀림:\n%s", out)
	}
	solo := viewContent(buf, "", 2)
	if strings.Contains(solo, "alpha") || !strings.Contains(solo, "beta") {
		t.Errorf("솔로 #2 결과 틀림:\n%s", solo)
	}
}

func TestAppendBufCaps(t *testing.T) {
	var buf []LogLine
	total := 0
	for i := 0; i < maxBuf+10; i++ {
		var dropped int
		buf, dropped = appendBuf(buf, LogLine{Num: 1, Text: "x", Kind: KindLog})
		total += dropped
	}
	if len(buf) != maxBuf {
		t.Errorf("상한 %d 기대, 실제 %d", maxBuf, len(buf))
	}
	// maxBuf 이내에서는 폐기 없음, 그 이후 10줄이 앞에서 밀려난다.
	if total != 10 {
		t.Errorf("폐기 누적 10 기대, 실제 %d", total)
	}
}

// TestAppendBufDiscardedCount 은 상한 도달 전에는 0, 이후 매 추가마다 1씩
// 폐기됨을 단언한다.
func TestAppendBufDiscardedCount(t *testing.T) {
	buf := make([]LogLine, maxBuf) // 이미 가득 찬 상태
	buf, dropped := appendBuf(buf, LogLine{Num: 1, Text: "x", Kind: KindLog})
	if dropped != 1 {
		t.Errorf("가득 찬 버퍼에 추가 시 폐기 1 기대, 실제 %d", dropped)
	}
	if len(buf) != maxBuf {
		t.Errorf("상한 유지 %d 기대, 실제 %d", maxBuf, len(buf))
	}

	var empty []LogLine
	_, dropped = appendBuf(empty, LogLine{Num: 1, Text: "x", Kind: KindLog})
	if dropped != 0 {
		t.Errorf("여유 있는 버퍼는 폐기 0 기대, 실제 %d", dropped)
	}
}

// TestModelTracksDiscarded 은 model 이 폐기 누적을 추적하고 View 에 노출하는지
// 확인한다.
func TestModelTracksDiscarded(t *testing.T) {
	m := newModel([]Lane{{Num: 1, Toggle: "i-a"}}, "", "uq log  myapp · prod")
	// 뷰포트 준비 (View 가 m.ready 를 요구).
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = nm.(model)
	// 버퍼를 미리 가득 채워 다음 로그가 곧바로 폐기를 유발하게 한다.
	m.buf = make([]LogLine, maxBuf)
	for i := range m.buf {
		m.buf[i] = LogLine{Num: 1, ID: "i-a", Text: "old", Kind: KindLog}
	}
	nm, _ = m.Update(logMsg{Num: 1, ID: "i-a", Text: "overflow", Kind: KindLog})
	m = nm.(model)
	if m.discarded == 0 {
		t.Fatalf("폐기 카운터가 증가해야 한다, 실제 %d", m.discarded)
	}
	if v := m.View(); !strings.Contains(v, "생략") {
		t.Errorf("View 에 생략 표시가 보여야 한다:\n%s", v)
	}
}
