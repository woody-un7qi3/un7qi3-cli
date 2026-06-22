package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/un7qi3inc/un7qi3-cli/internal/auth"
	"github.com/un7qi3inc/un7qi3-cli/internal/clierr"
	"github.com/un7qi3inc/un7qi3-cli/internal/cmd"
)

func main() {
	// 최상위 진입점에서만 Background 를 만든다. Ctrl+C(SIGINT)/SIGTERM 를 context
	// 에 연결해 RunE 의 cmd.Context() 가 취소를 받게 한다 — 라이브러리·exec 함수는
	// 이 ctx 를 따라 goroutine·자식 프로세스를 정리한다. 두 번째 신호부터는
	// NotifyContext 가 자동으로 핸들러를 해제하므로 기본 동작(즉시 종료)으로 돌아가
	// 멈춘 자식에 갇히지 않는다.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// Exit codes are decided in one place by clierr.Classify; see its package
	// doc for the full convention. main is the single exit point so that defer
	// cleanup in command bodies runs before the process leaves.
	//   0 — success
	//   1 — runtime error (recognised domain errors)
	//   2 — usage error: cobra unknown commands / flag parse / Args violations,
	//       and any error not otherwise classified (historical default).
	//   4 — authentication required (auth.RequiredError returned from RunE)
	err := cmd.Execute(ctx)
	code := clierr.Classify(err)
	if code == 0 {
		return
	}

	// Auth-required errors print their bare message to stderr — the form
	// users have always seen for exit 4. (cobra also prints "Error: ..."
	// since SilenceErrors is false in root.go; this pre-existing double
	// line is preserved here, not introduced by the refactor.)
	var authErr *auth.RequiredError
	if errors.As(err, &authErr) {
		fmt.Fprintln(os.Stderr, authErr.Msg)
	}

	os.Exit(code)
}
