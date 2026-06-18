package logs

import "strings"

const maxBuf = 5000

// visible 은 라인이 현재 필터·인스턴스 토글 기준으로 보여야 하는지.
func visible(ln LogLine, filter string, hidden map[int]bool) bool {
	if hidden[ln.Num] {
		return false
	}
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(ln.Text), strings.ToLower(filter))
}

// viewContent 는 버퍼를 필터링해 렌더된 줄들을 줄바꿈으로 결합한다.
func viewContent(buf []LogLine, filter string, hidden map[int]bool) string {
	var b strings.Builder
	first := true
	for _, ln := range buf {
		if !visible(ln, filter, hidden) {
			continue
		}
		if !first {
			b.WriteByte('\n')
		}
		b.WriteString(renderLine(ln))
		first = false
	}
	return b.String()
}

// appendBuf 는 링버퍼에 추가하되 maxBuf 를 넘으면 앞에서 버린다.
func appendBuf(buf []LogLine, ln LogLine) []LogLine {
	buf = append(buf, ln)
	if len(buf) > maxBuf {
		buf = buf[len(buf)-maxBuf:]
	}
	return buf
}
