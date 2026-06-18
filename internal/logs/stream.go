package logs

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"sync"
)

// LineKind 는 LogLine 의 종류.
type LineKind int

const (
	KindLog     LineKind = iota // 일반 로그 라인
	KindConnErr                 // 접속 실패
	KindEnd                     // 스트림 종료(eb wait 에러)
)

// LogLine 은 한 인스턴스에서 온 로그 라인 한 줄(또는 상태 메시지).
type LogLine struct {
	Num  int
	ID   string
	Text string
	Kind LineKind
}

// execCommand 는 테스트에서 주입 가능한 명령 생성자.
var execCommand = exec.CommandContext

// StreamLines 는 각 인스턴스의 eb 프로세스를 spawn 해 LogLine 을 채널로 방출한다.
// 모든 인스턴스 스트림이 끝나면 채널을 닫는다. 인스턴스별 실패는 KindConnErr/KindEnd
// 로 격리되어 다른 인스턴스에 영향을 주지 않는다.
func StreamLines(ctx context.Context, src Source, t Target, env string,
	insts []Instance, follow bool, lines int, grep string) <-chan LogLine {

	ch := make(chan LogLine, 256)
	var wg sync.WaitGroup
	for _, in := range insts {
		wg.Add(1)
		go func(in Instance) {
			defer wg.Done()
			args := src.TailArgs(t, env, in, follow, lines, grep)
			cmd := execCommand(ctx, "eb", args...)
			pr, pw, err := os.Pipe()
			if err != nil {
				ch <- LogLine{Num: in.Num, ID: in.ID, Text: err.Error(), Kind: KindConnErr}
				return
			}
			cmd.Stdout = pw
			cmd.Stderr = pw
			if err := cmd.Start(); err != nil {
				pw.Close()
				pr.Close()
				ch <- LogLine{Num: in.Num, ID: in.ID, Text: err.Error(), Kind: KindConnErr}
				return
			}
			pw.Close() // 부모 쪽 write end 닫기 — 자식 종료 시 pr 가 EOF
			sc := bufio.NewScanner(pr)
			sc.Buffer(make([]byte, 4096), 1024*1024)
			for sc.Scan() {
				ch <- LogLine{Num: in.Num, ID: in.ID, Text: sc.Text(), Kind: KindLog}
			}
			scanErr := sc.Err()
			pr.Close()
			if err := cmd.Wait(); err != nil {
				ch <- LogLine{Num: in.Num, ID: in.ID, Text: err.Error(), Kind: KindEnd}
			} else if scanErr != nil {
				ch <- LogLine{Num: in.Num, ID: in.ID, Text: scanErr.Error(), Kind: KindEnd}
			}
		}(in)
	}
	go func() { wg.Wait(); close(ch) }()
	return ch
}
