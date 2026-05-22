---
name: uq-scaffold
description: un7qi3-cli(`uq` 바이너리) 프로젝트의 cobra 기반 Go 스캐폴딩을 생성한다. Phase 문서의 디렉토리 트리와 명령 목록을 그대로 따라가며, version/doctor는 실제 구현, 나머지는 일관된 stub 패턴으로 채운다. uq 신규 스캐폴딩, 새 명령 추가, cobra 명령 골격 작성 시 사용한다.
---

# uq Scaffold — Go + cobra 프로젝트 스캐폴딩

## 트리거 시점

- "uq 스캐폴딩 만들어줘"
- "uq에 새 명령 추가해줘" (단일 명령 추가에도 사용 가능)
- Phase 문서의 "디렉토리 구조" / "초기 명령 트리"를 코드로 옮길 때

## 절대 규칙

1. **Phase 문서가 SSOT** — 명령 목록, 디렉토리 트리, 의존성은 문서가 정한 것만. 임의 추가 금지.
2. **컨벤션 일관성** — 모든 cobra 명령 파일은 동일 골격. 실제 구현/stub 차이는 RunE 내부에만.
3. **Stub은 정직하게** — "TODO: not yet implemented (will be added in Phase X)" 같은 메시지 + exit 0. 가짜 동작 금지.

## 작업 순서

### 1. 프로젝트 초기화

작업 디렉토리에서:

```bash
cd <작업 디렉토리>
git init       # 이미 .git 있으면 스킵
go mod init github.com/un7qi3inc/un7qi3-cli
```

의존성 추가:
```bash
go get github.com/spf13/cobra@latest
go get github.com/spf13/viper@latest
go get github.com/charmbracelet/huh@latest
go get github.com/aws/aws-sdk-go-v2/config@latest
go get github.com/aws/aws-sdk-go-v2/service/ssm@latest
```

### 2. 디렉토리 구조 생성

Phase 문서의 트리 그대로 `mkdir -p` 로 생성. 빈 디렉토리도 만들고 그 안에 `.gitkeep` 또는 실제 파일을 넣음.

### 3. 진입점 (`cmd/uq/main.go`)

```go
package main

import (
	"os"

	"github.com/un7qi3inc/un7qi3-cli/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
```

### 4. 루트 명령 (`internal/cmd/root.go`)

- `uq` 자체. 글로벌 플래그(`--json`, `--verbose`, `--config`) 정의.
- 각 서브 명령(version/doctor/install/repo/auth/env/deploy/logs/upgrade/skills)을 import해서 `rootCmd.AddCommand(...)` 등록.
- `Execute()` 함수 export.

골격:
```go
package cmd

import (
	"github.com/spf13/cobra"
	// 각 서브 명령 패키지 import
)

var (
	flagJSON    bool
	flagVerbose bool
	flagConfig  string
)

var rootCmd = &cobra.Command{
	Use:   "uq",
	Short: "un7qi3 internal CLI",
	Long:  "LLM-callable CLI for un7qi3 onboarding, repo setup, deploys, and ops.",
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "output as JSON")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringVar(&flagConfig, "config", "", "config file path")

	rootCmd.AddCommand(version.NewCmd())
	rootCmd.AddCommand(doctor.NewCmd())
	// ... 나머지 명령
}

func Execute() error {
	return rootCmd.Execute()
}
```

### 5. 명령 패턴 — 실제 구현

`uq version` (`internal/cmd/version/version.go`):

```go
package version

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show uq version",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOut, _ := cmd.Flags().GetBool("json")
			if jsonOut {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{
					"version": Version,
					"commit":  Commit,
					"date":    Date,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "uq %s (%s, %s)\n", Version, Commit, Date)
			return nil
		},
	}
}
```

`uq doctor` (`internal/cmd/doctor/doctor.go`) — Phase 문서의 점검 항목 표를 코드로:
- 각 항목을 `Check` 구조체로 표현: `name`, `cmd` (실행할 명령), `parseVersion` (선택), `fix` (누락 시 가이드 메시지), `optional`.
- 순차 실행 후 사람 친화 출력 또는 `--json` 구조화 출력.

### 6. 명령 패턴 — Stub

모든 stub 명령은 동일 골격:

```go
package install

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <team>",
		Short: "Clone team's repos (TUI multi-select)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStderr(), "TODO: not yet implemented (Phase 0 stub)")
			return nil
		},
	}
	return cmd
}
```

서브커맨드를 가진 명령(`repo`, `auth`, `env`, `deploy`)은:
- `<parent>/<parent>.go` 에 부모 명령 정의 (자체 RunE 없이 서브 추가만)
- 서브를 별도 파일에 정의

### 7. Stub에서 Phase 표시

Phase 문서가 `Phase 2 stub`, `Phase 3 stub`으로 표시한 명령은 메시지에 명시:
```go
fmt.Fprintln(cmd.OutOrStderr(), "Phase 2: not yet released")
```

### 8. 보조 코드

`internal/output/json.go`:
- `WriteJSON(w io.Writer, v any) error` — 표준 JSON 출력 헬퍼

`internal/output/tty.go`:
- 색깔/체크마크 같은 사람 친화 출력 헬퍼 (`✓`, `✗`, `-`)

`internal/config/config.go`:
- `~/.config/un7qi3/config.yml` viper 로드. Phase 0에선 빈 구조체 + 로드 함수만.

`internal/manifest/manifest.go`:
- `.uq.yml` 파서. Phase 0에선 구조체 정의 + Load 함수만, 실제 사용은 후속 Phase.

`internal/version/version.go`:
- ldflags로 주입될 변수: `Version`, `Commit`, `Date`. version 명령이 이걸 import해서 사용.

### 9. Makefile

```makefile
BINARY := uq
PREFIX := /usr/local
LDFLAGS := -X github.com/un7qi3inc/un7qi3-cli/internal/version.Version=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev) \
           -X github.com/un7qi3inc/un7qi3-cli/internal/version.Commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown) \
           -X github.com/un7qi3inc/un7qi3-cli/internal/version.Date=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)

.PHONY: build install test lint clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/uq

install: build
	install -m 0755 bin/$(BINARY) $(PREFIX)/bin/$(BINARY)

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -rf bin/
```

### 10. .gitignore

```
bin/
dist/
*.test
*.out
.DS_Store
```

### 11. README.md

최소한:
- 프로젝트 한 줄 설명
- 로컬 설치 방법 (`make install`)
- 기본 명령 예시 (`uq --help`)
- 문서 위치 (`docs/`)

### 12. 컴파일 검증

마지막에 반드시:
```bash
go build ./cmd/uq
```
성공해야 작업 완료. 실패하면 정확히 어느 파일 어느 줄에서 막혔는지 보고.

## 주의

- Go 모듈 경로는 `github.com/un7qi3inc/un7qi3-cli` 고정 (Phase 문서에 명시).
- 사용자가 한국어로 보고를 받음. 변수명/함수명/주석은 영어로 작성 (Go 컨벤션). 사용자 메시지만 한국어.
- 외부 의존성을 Phase 문서에 없는 것을 추가하지 말 것. 더 좋은 라이브러리가 있어도.
