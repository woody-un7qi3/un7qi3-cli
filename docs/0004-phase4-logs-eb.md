# 0004 — Phase 4: uq logs (EB 실시간 로그 스트리밍)

**Status:** 시작전

## Context

`uq logs <repo>` 는 현재 Phase 0 stub("TODO: 아직 구현되지 않음")이다. un7qi3 서비스 대부분이 **AWS Elastic Beanstalk(EB)** 로 구성되어 있고, 지금 개발자는 로그를 보려면 수동으로:

```
eb ssh api-beta-kr-j21       # 인스턴스 한 대에 SSH 접속 (환경명에 -j21 같은 불규칙 suffix)
cd /var/log
tail -f web.stdout.log       # 직접 tail
```

이 흐름의 문제:

- **인스턴스마다 따로 SSH** — 오토스케일로 N대면 창을 N개 열어야 하고, 어느 인스턴스가 요청을 받았는지 몰라 다 뒤져야 함.
- **수동 단계 반복** — 접속 → 경로 이동 → tail 을 매번 손으로.
- **필터/환경 전환 번거로움** — `grep ERROR`, kr/beta/prod 전환을 매번 손으로.

이번 Phase는 이 수동 작업을 `uq logs <repo>` 한 줄로 대체한다. **실시간(follow) 멀티플렉스**가 핵심 목표다.

### 조사 결과 — 현재 코드/인프라 전제

- `aws` CLI 는 doctor 가 추적(설치/버전 점검). `eb` CLI 와 `session-manager-plugin` 은 미추적.
- **환경명 규칙이 프로젝트마다 다르고 불규칙하다** — 예: forceteller-api 는 `api-beta-kr-j21`(suffix `-j21` 은 재생성 시 바뀜), 다른 프로젝트는 `production`·`myapp-live` 등 전혀 다른 규칙. 손으로 적거나 템플릿/축(stage·country)으로 생성하는 방식은 일반화되지 않는다 → **실시간 발견(discovery) + 부분문자열 필터가 유일하게 견고**하다.
- 환경 목록은 `aws elasticbeanstalk describe-environments --application-name <app> --query "Environments[?Status=='Ready'].EnvironmentName"` 로 발견. application 이름은 레포의 `.elasticbeanstalk/config.yml` 의 `global.application_name` 에서 읽는다. (`eb list` 로도 가능하나 `*` 마커 파싱이 필요해 aws 쪽을 택함.)
- `internal/run/split.go` 에 cmux/iTerm2/ghostty/AppleTerminal 패널 여는 로직과 `internal/run/terminal.go` 의 멀티플렉서 감지(`DetectMultiplexer`)가 이미 있음 → `--split` 재사용 대상.
- `eb ssh <env>` 는 cwd 의 `.elasticbeanstalk/config.yml`(application/region)을 읽는다. 따라서 uq 는 **레포의 로컬 클론 디렉토리에서** eb ssh 를 실행해야 한다 (`uq run` 이 reposDir 에서 도는 것과 동일).

## 설계 원칙 (이번 Phase 추가)

기존 Phase 원칙(결정론·사전검증·환경 적응)에 더해:

12. **소스는 서브커맨드가 아니라 데이터** — 로그 소스(eb/ecs/…)는 레포의 속성이므로 `uq logs eb` 같은 서브커맨드가 아니라 `.uq.yml` 의 `logs.type` 으로 표현한다. CLI 표면(`uq logs <repo>`)은 고정하고, 내부는 `Source` 드라이버 인터페이스로 확장한다.
13. **검증 가능한 계획(plan)** — 실 스트리밍은 자동 검증이 어려우므로 `--dry-run` 으로 "해석된 환경·인스턴스·실행 명령"을 부작용 없이 출력한다 (run/deploy 의 `--dry-run` 철학).

## CLI 표면

```
uq logs <repo> [필터...] [--instance N] [--grep <regex>] [--split] [--no-follow] [--dry-run]
```

환경 이름 규칙은 **프로젝트마다 다르다**(`api-beta-kr-j21`, `production`, `myapp-live` …). 따라서 stage/country 같은 축을 가정하지 않고, **실시간 발견된 환경 목록에서 TUI 로 선택**하는 것을 기본으로 한다. 위치인자 `필터` 는 그 목록을 미리 좁히는 부분문자열(대소문자 무시, 여러 개면 AND)일 뿐, 이름 규칙을 해석하지 않는다.

- `<repo>` — 워크스페이스에 클론된 레포 이름 (uq run/repo 와 동일 해석).
- `필터...` — 선택. 발견된 환경 이름에 **모두 포함**되는 것만 남긴다. 예: `beta kr` → `api-beta-kr-j21`.
- `--instance N` — 1-base 인스턴스 번호로 한정(`eb ssh -n N` 의 번호). 생략 시 전체 인스턴스.
- `--grep <regex>` — 클라이언트단 정규식 라인 필터.
- `--split` — 인스턴스별 패널 분리(멀티프로세스 분할 로직 재사용). 생략 시 한 화면 merged. `--no-follow` 와 상호 배타(스냅샷을 패널로 나눌 이유가 없음 → exit 2).
- `--no-follow` — `tail -F` 없이 최근 N줄(기본 100)만 출력하고 종료. 생략 시 실시간 follow(기본).
- `--dry-run` — 해석된 EB 환경명·인스턴스 목록·각 인스턴스의 eb ssh 명령을 출력하고 종료(실행 안 함).

> 기본 동작은 **실시간 follow + merged**. 즉 `uq logs forceteller-api` → (발견된 환경) TUI 선택 → 해당 환경 전 인스턴스의 `web.stdout.log` 를 `[env#i]` prefix 로 한 화면에 실시간 합쳐 출력.

### 환경 결정 흐름

1. `.elasticbeanstalk/config.yml` 에서 application 이름을 읽어 `aws elasticbeanstalk describe-environments` 로 Ready 환경 목록을 발견.
2. 위치인자 `필터` 가 있으면 이름에 모든 필터를 포함하는 것만 남김(대소문자 무시).
3. 남은 환경이 1개면 자동 확정, 여러 개면 **huh TUI** 로 선택.
4. 비TTY 에서 모호(>1)하면 exit 1 — 자동화는 단일 환경으로 좁혀지는 필터를 줘야 한다.

## 설정 — `.uq.yml` 의 logs 블록

환경 목록은 실시간 발견하므로 **`.uq.yml` 에 환경을 적지 않는다.** 발견·이름규칙으로 알 수 없는 것(로그 파일 경로 등)만 둔다. 중앙 `repos.yml` 이 아니라 레포 안 `.uq.yml` 에 둔다.

```yaml
# forceteller-api/.uq.yml
logs:
  type: eb                       # 기본 eb. 미래: ecs / k8s ...
  path: /var/log/web.stdout.log  # 기본값. 레포가 다르면 오버라이드
  # app: <application-name>      # 선택. 보통 .elasticbeanstalk/config.yml 에서 읽으므로 생략
```

- `type` 생략 시 `eb`. `path` 생략 시 `/var/log/web.stdout.log`.
- `logs` 블록 전체가 없어도 기본값으로 동작(eb + 기본 경로). 환경 발견에 필요한 application 이름은 `.elasticbeanstalk/config.yml` 에서 읽으며, 그것도 없으면 exit 1 + `eb init` 안내.

매니페스트 확장(`internal/manifest/manifest.go`):

```go
type Manifest struct {
    Logs *LogsConfig `yaml:"logs"`
}

type LogsConfig struct {
    Type string `yaml:"type"` // "" → "eb"
    Path string `yaml:"path"` // "" → "/var/log/web.stdout.log"
    App  string `yaml:"app"`  // "" → .elasticbeanstalk/config.yml 의 global.application_name
}
```

## 내부 구조 — Source 드라이버

```go
// internal/logs/source.go
type Instance struct {
    ID    string // EC2 인스턴스 id (라벨/표시용)
    Num   int    // 1-base, eb ssh -n 번호
    Label string // 표시용, 예: "api-beta-kr-j21#1"
}

type Source interface {
    // Environments 는 발견된 환경 이름 목록을 반환한다(Ready 상태).
    Environments() ([]string, error)
    // Instances 는 환경의 인스턴스 목록을 반환한다.
    Instances(env string) ([]Instance, error)
    // TailArgs 는 한 인스턴스의 로그를 스트리밍하는 argv 를 반환한다.
    TailArgs(env string, inst Instance, follow bool, lines int) []string
}
```

EB 드라이버(`internal/logs/eb.go`):

- `Environments`: application 이름(`.uq.yml` 의 `app` 또는 `.elasticbeanstalk/config.yml` 의 `global.application_name`)으로 `aws elasticbeanstalk describe-environments --application-name <app> --query "Environments[?Status=='Ready'].EnvironmentName" --output text` → 이름 목록.
- `Instances`: `aws elasticbeanstalk describe-environment-resources --environment-name <env>` → `Instances[].Id`. 개수로 `Num` 1..N 부여, 라벨 `"<env>#<Num>"`.
  - region 은 레포의 `.elasticbeanstalk/config.yml` 또는 aws 기본 설정을 사용.
- `TailArgs`: `eb ssh <env> -n <Num> -c "sudo tail <follow?-F : -n N> <path>"`.
  - follow: `sudo tail -n 100 -F <path>` (최근 100줄 후 실시간).
  - no-follow: `sudo tail -n <lines> <path>` 후 종료.
- eb ssh 는 레포 클론 디렉토리를 cwd 로 실행(.elasticbeanstalk 필요).

## 출력 — merged / split

- **merged**(기본): 각 인스턴스의 tail 프로세스를 spawn 하고 stdout 라인마다 `[<label>]` 색상 prefix 를 붙여 한 스트림으로 합친다(`internal/logs/mux.go`). `--grep` 정규식은 여기서 필터. Ctrl+C 로 전체 종료.
- **split**: 인스턴스별 별도 패널. `internal/run/split.go` 의 패널 여는 로직을 `internal/run` 공유 패키지로 추출해 run·logs 가 함께 쓴다. 분할 불가 환경은 run 과 동일하게 merged 로 graceful degrade + 안내.

### split 로직 추출(소폭 리팩터)

`internal/cmd/run/split.go` 의 패널 타입과 패널 열기 함수(`panel`, `openPanel`, `openCmuxPanel`, `openITerm2Panel`, `openGhosttyPanel`, `openAppleTerminalPanel`, 셸 인용 헬퍼)를 `internal/run/panel.go` 로 옮겨 export 한다.

- `run.Panel{ Label, Command, Dir }` + `run.OpenPanel(mux, panel, splitDir)` 공개.
- `internal/cmd/run/split.go` 는 기존 `buildPanels` 결과를 `run.Panel` 로 변환해 `run.OpenPanel` 호출(동작 불변).
- `internal/logs` 는 인스턴스마다 `run.Panel{Label, Command: eb ssh ...}` 을 만들어 동일 함수 사용.

## 도구 의존성 — doctor 확장

`eb` CLI 를 doctor 의 추적 도구에 추가(`internal/cmd/doctor/doctor.go`):

```go
{ Name: "eb", Fix: "pip install awsebcli", Run: versionCheck("eb", []string{"--version"}, `EB CLI (\S+)`) }
```

(역할 그룹은 기존 분류 규칙에 맞춰 배치. `session-manager-plugin` 은 SSM transport 도입 시 후속 추가.)

## 디렉토리 구조 (Phase 4에 추가/변경)

```
internal/cmd/logs/logs.go          [변경] stub → cobra 와이어링·플래그·오케스트레이션·aws 사전확인·env TUI
internal/logs/source.go            [신규] Source 인터페이스 + Instance
internal/logs/eb.go                [신규] EB 드라이버 (eb ssh + describe-environment-resources)
internal/logs/mux.go               [신규] merged 멀티플렉스 출력 + --grep 필터
internal/logs/eb_test.go           [신규] TailArgs 명령 구성·env 매칭·인스턴스 파싱 단위테스트
internal/logs/mux_test.go          [신규] prefix·grep 필터 단위테스트
internal/manifest/manifest.go      [변경] Logs 블록 파싱
internal/manifest/manifest_test.go [신규] .uq.yml logs 파싱·기본값 테스트
internal/run/panel.go              [신규] split.go 에서 추출한 공유 패널 로직 (export)
internal/cmd/run/split.go          [변경] 추출된 run.OpenPanel 사용 (동작 불변)
internal/cmd/doctor/doctor.go      [변경] eb 도구 추적 추가
docs/0004-phase4-logs-eb.md        [신규] 본 문서
```

## 에러 코드 사용

- **exit 4** — aws 인증 미충족(`uq auth login --aws-only` 안내). repo 명령의 gh 사전확인과 동일 패턴.
- **exit 2** — 사용법 에러(인자 개수, 상호 배타 플래그 등 cobra 처리).
- **exit 1** — 런타임 에러: 레포 미클론 / `.uq.yml`·logs 블록 없음 / 알 수 없는 env / 인스턴스 0개 / eb·aws 실행 실패.

## 검증 (Verification)

```bash
# 빌드 + 단위 테스트
go build ./...
go test ./internal/logs/... ./internal/manifest/... ./internal/run/...

# --help / 사용법
uq logs --help                         # 플래그·예시 노출 (stub 문구 없음)
uq logs                                # exit 2 (인자 필요)

# 사전확인: aws 미인증 시 (세션에서 시뮬레이션)
#   → exit 4, "uq auth login --aws-only" 안내

# .elasticbeanstalk 없는(=eb init 안 된) 클론 레포
uq logs <eb_init_안된_레포>             # exit 1, eb init 안내

# 계획만 출력 (실제 스트리밍 없이) — 핵심 자동 검증 경로
uq logs forceteller-api beta kr --dry-run
#   → 발견된 환경에서 beta+kr 필터 → 단일(api-beta-kr-j21), 인스턴스(#1..#N), eb ssh 명령 출력, exit 0
uq logs forceteller-api beta kr --instance 1 --dry-run   # 인스턴스 1개로 한정
uq logs forceteller-api zzz --dry-run                    # exit 1, 필터에 맞는 환경 없음

# 플래그 충돌
uq logs forceteller-api beta kr --split --no-follow --dry-run   # exit 2 (상호 배타)

# split 감지 (현재 세션 cmux)
uq logs forceteller-api beta kr --split --dry-run        # 감지 결과 + 패널별 명령 표시

# 분할 불가 환경 → merged fallback 안내
env -u TMUX -u CMUX_PANEL_ID -u CMUX_SOCKET_PATH TERM_PROGRAM=Apple_Terminal \
  uq logs forceteller-api beta kr --split --dry-run
```

> 주의: 실제 follow 스트리밍은 EB·SSH·클론된 레포가 필요해 자동 검증에서 제외한다. 자동 검증은 `--dry-run` 과 단위 테스트로만 한다.

## 명시적으로 제외 (Phase 4)

- **`--since` 시간 기반 필터** — 파일 tail 의 시간 필터는 로그 포맷마다 달라 신뢰성이 낮다. MVP 는 실시간 follow + 최근 100줄 백로그만. 시간 필터는 CloudWatch 도입(아래) 시 자연스럽게 해결.
- **CloudWatch Logs / SSM transport** — 이번 MVP 는 `eb ssh` 만. 인스턴스 id 직접 타겟팅·서버사이드 필터·인프라 무중단은 후속 Phase 에서 `Source` 드라이버 추가로 확장(데이터 기반 `logs.type` 덕분에 CLI 불변).
- **EB 외 소스(ecs/k8s/local)** — 인터페이스만 열어두고 드라이버는 미구현.
- **로그 경로 per-proc/멀티 파일** — 한 환경당 단일 `path`. 여러 파일 동시 tail 은 범위 밖.
- **분할 패널 생명주기 관리** — 패널 일괄 종료/정리는 run 과 동일하게 범위 밖(사용자가 직접 닫음).
- **`.elasticbeanstalk/config.yml` 자동 생성** — eb init 은 사용자/레포 책임. uq 는 존재를 전제하고 없으면 안내만.
