package logs

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/un7qi3inc/un7qi3-cli/internal/output"
)

// RenderLegend 은 시작 시 #k → 인스턴스 id 매핑을 표로 출력할 문자열을 만든다.
func RenderLegend(insts []Instance) string {
	var b strings.Builder
	for _, in := range insts {
		fmt.Fprintf(&b, "%s → %s\n", colorNum(in.Num, fmt.Sprintf("#%d", in.Num)), in.ID)
	}
	return b.String()
}

// PrefixLine 은 로그 라인에 색상 [#k <단축 id>] prefix 를 붙인다(id 는 앞 5글자).
// 전체 id 는 시작 시 범례(RenderLegend)에서 확인한다.
func PrefixLine(num int, line string) string {
	return colorNum(num, fmt.Sprintf("[#%d]", num)) + " " + line
}

// shortID 는 "i-" 접두사를 유지하고 그 뒤 5글자만 남긴다(예: i-09e13).
func shortID(id string) string {
	prefix := ""
	rest := id
	if strings.HasPrefix(id, "i-") {
		prefix = "i-"
		rest = id[len("i-"):]
	}
	if len(rest) > 5 {
		rest = rest[:5]
	}
	return prefix + rest
}

// highlightLevel 은 ERROR 로그 라인 본문을 빨강으로 강조한다. 그 외는 원본 유지.
func highlightLevel(line string) string {
	if strings.Contains(line, "ERROR") {
		return output.Red(line)
	}
	return line
}

// GrepMatch 은 re 가 nil 이면 항상 true, 아니면 line 매칭 여부.
func GrepMatch(re *regexp.Regexp, line string) bool {
	if re == nil {
		return true
	}
	return re.MatchString(line)
}

// colorNum 은 인스턴스 번호별로 색을 돌려 라벨에 입힌다.
// 빨강은 ERROR 강조 전용이라 인스턴스 구분 팔레트에서 제외한다.
func colorNum(num int, s string) string {
	palette := []func(string) string{
		output.Cyan, output.Green, output.Yellow, output.Blue,
	}
	return palette[(num-1)%len(palette)](s)
}

// renderLine 은 LogLine 을 색상 prefix 가 붙은 한 줄로 렌더한다(평문/TUI 공용).
func renderLine(ln LogLine) string {
	switch ln.Kind {
	case KindConnErr:
		return PrefixLine(ln.Num, output.Red("접속 실패: ")+ln.Text)
	case KindEnd:
		return PrefixLine(ln.Num, output.Dim("스트림 종료: "+ln.Text))
	default:
		return PrefixLine(ln.Num, highlightLevel(ln.Text))
	}
}

// StreamMerged 는 StreamLines 를 소비해 한 화면에 합쳐 출력한다(평문 모드).
// --grep 가 있으면 KindLog 라인에 클라이언트 재필터를 적용해 eb 로컬 경고를 거른다.
func StreamMerged(ctx context.Context, w io.Writer, src Source, t Target, env string,
	insts []Instance, follow bool, lines int, grep string) error {

	var re *regexp.Regexp
	if grep != "" {
		var err error
		if re, err = regexp.Compile(grep); err != nil {
			return fmt.Errorf("--grep 정규식 오류: %w", err)
		}
	}
	fmt.Fprint(w, RenderLegend(insts))
	for ln := range StreamLines(ctx, src, t, env, insts, follow, lines, grep) {
		if ln.Kind == KindLog && !GrepMatch(re, ln.Text) {
			continue
		}
		fmt.Fprintln(w, renderLine(ln))
	}
	return nil
}
