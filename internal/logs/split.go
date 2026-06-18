package logs

import (
	"strings"

	"github.com/un7qi3inc/un7qi3-cli/internal/run"
)

// SplitSupported 는 logs 의 --split 이 지원하는 멀티플렉서인지(=cmux/iterm2).
func SplitSupported(mux run.Multiplexer) bool {
	return mux == run.MuxCmux || mux == run.MuxITerm2
}

// BuildPanels 는 eb argv(eb 제외) 목록을 "eb <argv...>" 셸 패널로 만든다.
func BuildPanels(argvs [][]string, labels []string) []run.Panel {
	panels := make([]run.Panel, 0, len(argvs))
	for i, argv := range argvs {
		panels = append(panels, run.Panel{
			Label:   labels[i],
			Command: "eb " + strings.Join(shellQuoteAll(argv), " "),
		})
	}
	return panels
}

// shellQuoteAll 은 각 인자를 필요시 작은따옴표로 감싼다(공백·특수문자 안전).
func shellQuoteAll(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		// 알파벳, 숫자, 일부 특수문자만 포함하면 인용 불필요
		if isSafeArgument(a) {
			out[i] = a
		} else {
			out[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
	}
	return out
}

// isSafeArgument 는 인자가 셸 특수문자를 포함하지 않는지 검사.
func isSafeArgument(s string) bool {
	for _, c := range s {
		// 영문자, 숫자, 하이픈, 언더스코어, 점, 슬래시만 안전
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '/') {
			return false
		}
	}
	return true
}
