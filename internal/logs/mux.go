package logs

import (
	"fmt"
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

// PrefixLine 은 로그 라인에 색상 [#k] prefix 를 붙인다.
func PrefixLine(num int, line string) string {
	return colorNum(num, fmt.Sprintf("[#%d]", num)) + " " + line
}

// GrepMatch 은 re 가 nil 이면 항상 true, 아니면 line 매칭 여부.
func GrepMatch(re *regexp.Regexp, line string) bool {
	if re == nil {
		return true
	}
	return re.MatchString(line)
}

// colorNum 은 인스턴스 번호별로 색을 돌려 라벨에 입힌다.
func colorNum(num int, s string) string {
	palette := []func(string) string{
		output.Cyan, output.Green, output.Yellow, output.Red, output.Blue, output.Dim,
	}
	return palette[(num-1)%len(palette)](s)
}
