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
				if !send(ctx, ch, LogLine{Num: in.Num, ID: in.ID, Text: sc.Text(), Kind: KindLog}) {
					// 소비자가 멈췄거나(ctx 취소) 채널을 더 읽지 않는다 — eb 프로세스를
					// 정리하고 goroutine 을 빠져나가 누수를 막는다. CommandContext 가
					// ctx 취소 시 자식을 죽이지만, pr 를 닫아 Scan 도 즉시 풀어준다.
					pr.Close()
					_ = cmd.Wait()
					return
				}
			}
			scanErr := sc.Err()
			pr.Close()
			if err := cmd.Wait(); err != nil {
				send(ctx, ch, LogLine{Num: in.Num, ID: in.ID, Text: err.Error(), Kind: KindEnd})
			} else if scanErr != nil {
				send(ctx, ch, LogLine{Num: in.Num, ID: in.ID, Text: scanErr.Error(), Kind: KindEnd})
			}
		}(in)
	}
	go func() { wg.Wait(); close(ch) }()
	return ch
}

// send 는 ln 을 ch 로 보내되 ctx 취소를 존중한다. 보냈으면 true, ctx 가 먼저
// 취소되면 false 를 반환해 호출자가 goroutine 을 정리하고 빠져나가게 한다.
// 이 가드가 없으면 소비자가 멈춘 뒤 버퍼가 차면 send 가 영구 블록돼 누수된다.
func send(ctx context.Context, ch chan<- LogLine, ln LogLine) bool {
	select {
	case ch <- ln:
		return true
	case <-ctx.Done():
		return false
	}
}
