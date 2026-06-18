package logs

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"sync"

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
func PrefixLine(num int, id, line string) string {
	return colorNum(num, fmt.Sprintf("[#%d %s]", num, shortID(id))) + " " + line
}

// shortID 는 인스턴스 id 의 앞 5글자만 반환한다(더 짧으면 그대로).
func shortID(id string) string {
	if len(id) > 5 {
		return id[:5]
	}
	return id
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

// StreamMerged 는 각 인스턴스의 eb 프로세스를 spawn 해 라인을 prefix·grep 후 합치고,
// 인스턴스별 실패는 격리한다.
func StreamMerged(ctx context.Context, w io.Writer, src Source, t Target, env string,
	insts []Instance, follow bool, lines int, grep string) error {

	var re *regexp.Regexp
	if grep != "" {
		var err error
		if re, err = regexp.Compile(grep); err != nil {
			return fmt.Errorf("--grep 정규식 오류: %w", err)
		}
	}
	fmt.Fprint(w, RenderLegend(insts)) // 범례

	var wg sync.WaitGroup
	var mu sync.Mutex // w 직렬화
	for _, in := range insts {
		wg.Add(1)
		go func(in Instance) {
			defer wg.Done()
			args := src.TailArgs(t, env, in, follow, lines)
			cmd := exec.CommandContext(ctx, "eb", args...)
			stdout, err := cmd.StdoutPipe()
			cmd.Stderr = cmd.Stdout // eb 경고도 같이
			if err == nil {
				err = cmd.Start()
			}
			if err != nil {
				mu.Lock()
				fmt.Fprintln(w, PrefixLine(in.Num, in.ID, output.Red("접속 실패: ")+err.Error()))
				mu.Unlock()
				return
			}
			sc := bufio.NewScanner(stdout)
			sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
			for sc.Scan() {
				line := sc.Text()
				if !GrepMatch(re, line) {
					continue
				}
				mu.Lock()
				fmt.Fprintln(w, PrefixLine(in.Num, in.ID, highlightLevel(line)))
				mu.Unlock()
			}
			if err := cmd.Wait(); err != nil {
				mu.Lock()
				fmt.Fprintln(w, PrefixLine(in.Num, in.ID, output.Dim("스트림 종료: "+err.Error())))
				mu.Unlock()
			}
		}(in)
	}
	wg.Wait()
	return nil
}
