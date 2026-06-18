# 0004 — Phase 4: uq logs (EB 실시간 로그 스트리밍)

**Status:** 시작전

## Context

`uq logs <repo>` 는 현재 Phase 0 stub("TODO: 아직 구현되지 않음")이다. un7qi3 서비스 대부분이 **AWS Elastic Beanstalk(EB)** 로 구성되어 있고, 지금 개발자는 로그를 보려면 수동으로:

```
eb ssh api-beta-kr-j21       # 인스턴스 한 대에 SSH 접속
cd /var/log
tail -f web.stdout.log       # 직접 tail
```

이 흐름의 문제:

- **인스턴스마다 따로 SSH** — 오토스케일로 N대면 창을 N개 열어야 하고, 어느 인스턴스가 요청을 받았는지 몰라 다 뒤져야 함.
- **수동 단계 반복** — 접속 → 경로 이동 → tail 을 매번 손으로.
- **국가/환경 전환 번거로움** — kr/en/jp(=리전), beta/prod 전환을 매번 환경명 외워서 재접속.

이번 Phase는 이 수동 작업을 `uq logs <repo>` 한 줄로 대체한다. **실시간(follow) 멀티플렉스**가 핵심 목표다.

### 조사 결과 — 인프라 구조와 코드 전제

한 레포(서비스)가 **국가별로 별도의 EB application + 별도 리전**에 걸쳐 있고, 각 application 안에 beta/prod 환경이 있다. forceteller-api 실제 예:

| 국가 | EB application | 환경 | 리전 |
|---|---|---|---|
| kr | `kr-forceteller-api` | `api-beta-kr-j21`, `api-prod-kr-j21` | `ap-northeast-2` |
| en | `en-forceteller-api` | `api-beta-en-j21`, `api-prod-en-j21` | `ap-southeast-1` |
| jp | `jp-forceteller-api` | `api-beta-jp-j21`, `api-prod-jp-j21` | `ap-northeast-1` |

여기서 도출되는 핵심:

- **국가 = (application + region)** — application 이름과 region 이 국가마다 다르다. 발견 불가능(특히 region)하므로 **선언해야 한다.**
- **환경(beta/prod)은 발견 가능** — application 안의 환경명은 `aws elasticbeanstalk describe-environments` 로 실시간 발견한다. 이름의 `-j21` suffix 는 재생성 시 바뀌므로 손으로 적지 않는다.
- **`eb ssh <env> --region <r> -c "<cmd>"` 는 디렉토리에 무관하게 동작**(레포 클론·`.elasticbeanstalk/config.yml` 불필요)함을 실측 확인. eb 가 SSH 키(`~/.ssh/forceteller-service.pem`)를 자동 해석하고 `-c` 명령 출력을 stdout 으로 흘린다 → uq 는 키 관리·클론 불필요, `-c "sudo tail -F …"` 로 스트리밍.
  - 단, eb ssh 는 접속 시 SG 22번 포트를 임시로 열려 시도하며 소스 제한이 있으면 `WARNING: ... Use the --force flag` 를 내고 열지 않은 채 접속한다. **uq 는 `--force` 를 절대 쓰지 않는다**(인프라 변경). 개발자 IP 가 SG 에 없어 접속 실패하면 eb 의 경고/에러를 그대로 사용자에게 전달한다.
- 모니터링 대상은 **미리 등록된 레포로 제한**한다(allowlist). 계정 전체를 훑지 않아 무관 서비스 노출·TUI 노이즈를 막는다.
- `aws` CLI 는 doctor 가 추적. `eb` CLI 는 미추적(이번에 추가).
- `internal/run/split.go` 의 패널 여는 로직과 `internal/run/terminal.go` 의 `DetectMultiplexer` 가 이미 있어 `--split` 에 재사용.

## 설계 원칙 (이번 Phase 추가)

12. **발견 가능한 것은 발견하고, 불가능한 것만 선언** — 환경명(beta/prod+suffix)은 aws 로 발견하고, 발견 불가한 국가→(app, region) 매핑만 설정에 둔다.
13. **등록된 것만 모니터링(allowlist)** — `uq logs` 는 `repos.yml` 의 `logs:` 블록에 등록된 레포만 허용한다.
14. **소스는 데이터** — 로그 소스(eb/ecs/…)는 `Source` 드라이버 인터페이스로 추상화. CLI 표면(`uq logs <repo>`)은 고정하고 내부 드라이버로 확장한다.
15. **검증 가능한 계획(plan)** — 실 스트리밍은 자동 검증이 어려우므로 `--dry-run` 으로 "해석된 app/region/환경/인스턴스/실행 명령"을 부작용 없이 출력한다.

## CLI 표면

```
uq logs <repo> [필터...] [--instance N] [--grep <regex>] [--split] [--no-follow] [--dry-run]
```

- `<repo>` — `repos.yml` `logs:` 에 등록된 레포 이름. 미등록이면 exit 1 + 안내.
- `필터...` — 선택. 위치인자. 토큰이 **국가 코드**(`kr`/`en`/`jp` 등 `countries` 키)와 같으면 국가(=app+region) 확정, 그 외 토큰은 **발견된 환경명에 대한 부분문자열 필터**(여러 개면 AND). 예: `kr beta` → kr 국가 + 'beta' 포함 환경.
- `--instance N` — 1-base 인스턴스 번호로 한정(`eb ssh -n N`). 생략 시 전체 인스턴스.
- `--grep <regex>` — 클라이언트단 정규식 라인 필터.
- `--split` — 인스턴스별 패널 분리(분할 로직 재사용). 생략 시 한 화면 merged. `--no-follow` 와 상호 배타 → exit 2.
- `--no-follow` — `tail -F` 없이 최근 N줄(기본 100)만 출력하고 종료. 생략 시 실시간 follow(기본).
- `--dry-run` — 해석된 app/region/환경/인스턴스 목록과 각 eb ssh 명령을 출력하고 종료(실행 안 함).

> 기본 동작은 **실시간 follow + merged**.

### 환경 결정 흐름

```
uq logs forceteller-api
 1. logs[forceteller-api] 설정 로드 (없으면 exit 1: "logs 미등록")
 2. 국가 결정: 위치인자에 countries 키가 있으면 그것, 없으면 huh TUI (countries 키: kr/en/jp)
      → 선택 국가의 (app, region) 확정  예: kr → (kr-forceteller-api, ap-northeast-2)
 3. 환경 발견: aws elasticbeanstalk describe-environments
      --application-name kr-forceteller-api --region ap-northeast-2
      → [api-beta-kr-j21, api-prod-kr-j21]
 4. 환경 결정: 남은 위치인자(예: beta)로 부분문자열 필터 → 1개면 자동, 여러 개면 huh TUI
 5. 인스턴스 발견: describe-environment-resources --environment-name <env> --region <region>
      → 실행 순간의 라이브 인스턴스 N개(오토스케일 반영). 시작 시 1회 스냅샷 — 세션 도중
        스케일 변동분은 잡지 않으며, 새 인스턴스를 보려면 재실행한다.
 6. 스트리밍: 각 인스턴스에 eb ssh <env> --region <region> -n <i> -c "sudo tail -F <path>"
      merged: [<env>#i] 색상 prefix 한 화면 / --split: 인스턴스별 패널
      --grep: 클라이언트단 정규식 필터
```

- 비TTY 에서 국가/환경이 모호(>1)하면 exit 1 — 자동화는 단일로 좁혀지는 위치인자를 줘야 한다.
- `uq logs forceteller-api kr beta` 처럼 다 주면 TUI 없이 바로 스트리밍.

## 설정 — `repos.yml` 의 logs 블록

레포 클론·`.uq.yml` 에 의존하지 않으므로 중앙 `repos.yml` 에 둔다(`runs:` 와 동일 위치). 이 블록이 곧 allowlist.

```yaml
# repos.yml
logs:
  forceteller-api:                        # uq logs <이 키>
    path: /var/log/web.stdout.log         # 선택. 기본 /var/log/web.stdout.log
    countries:                            # 국가 → (app, region). 발견 불가 정보만 선언
      kr: { app: kr-forceteller-api, region: ap-northeast-2 }
      en: { app: en-forceteller-api, region: ap-southeast-1 }
      jp: { app: jp-forceteller-api, region: ap-northeast-1 }
```

- 환경(beta/prod)·인스턴스는 적지 않는다 — aws 로 발견.
- 국가축이 없는 단순 서비스는 단일 키로 표현(예: `countries: { default: { app: …, region: … } }`). 키가 1개면 국가 TUI 를 건너뛴다.

repocfg 확장(`internal/repocfg/repocfg.go`):

```go
type LogsConfig struct {
    Path      string                   `yaml:"path"`      // "" → "/var/log/web.stdout.log"
    Countries map[string]CountryTarget `yaml:"countries"`
}

type CountryTarget struct {
    App    string `yaml:"app"`
    Region string `yaml:"region"`
}
```

`repos.yml` 최상위에 `logs: map[string]LogsConfig` 추가. `repocfg.Load()` 가 함께 파싱하고, `LogsFor(repo)` 접근자를 제공한다.

## 내부 구조 — Source 드라이버

```go
// internal/logs/source.go
type Target struct {          // 한 국가의 EB application + region
    Country string
    App     string
    Region  string
}

type Instance struct {
    ID    string // EC2 인스턴스 id (표시용)
    Num   int    // 1-base, eb ssh -n 번호
    Label string // 표시용, 예: "api-beta-kr-j21#1"
}

type Source interface {
    // Environments 는 target(app+region)의 Ready 환경명을 발견한다.
    Environments(t Target) ([]string, error)
    // Instances 는 환경의 인스턴스 목록을 반환한다.
    Instances(t Target, env string) ([]Instance, error)
    // TailArgs 는 한 인스턴스를 스트리밍하는 argv 를 반환한다.
    TailArgs(t Target, env string, inst Instance, follow bool, lines int) []string
}
```

EB 드라이버(`internal/logs/eb.go`):

- `Environments`: `aws elasticbeanstalk describe-environments --application-name <t.App> --region <t.Region> --query "Environments[?Status=='Ready'].EnvironmentName" --output text`.
- `Instances`: `aws elasticbeanstalk describe-environment-resources --environment-name <env> --region <t.Region>` → `Instances[].Id`. `Num` 1..N, 라벨 `"<env>#<Num>"`.
- `TailArgs`: `eb ssh <env> --region <t.Region> -n <Num> -c "sudo tail <-F | -n N> <path>"`.
  - follow: `sudo tail -n 100 -F <path>` (최근 100줄 후 실시간).
  - no-follow: `sudo tail -n <lines> <path>` 후 종료.

## 출력 — merged / split

- **merged**(기본): 각 인스턴스의 tail 프로세스를 spawn 해 한 스트림으로 합친다(`internal/logs/mux.go`). 어느 인스턴스 로그인지 식별 가능해야 하므로:
  - **시작 시 범례** 출력 — `#k` → 실제 인스턴스 매핑:
    ```
    #1 → i-0abc123  (13.125.233.58)
    #2 → i-0def456  (13.125.233.59)
    ```
  - **라인별 prefix** `[#k]` 를 **인스턴스마다 다른 색**으로 붙임(run 의 `[back]`/`[front]` 방식). 색+번호로 구분, 범례로 실제 EC2 id/IP 추적.
  - `--grep` 정규식은 prefix 를 제외한 본문에 적용. Ctrl+C 로 전체 종료.
- **split**: 인스턴스별 별도 패널. `internal/run/split.go` 의 패널 여는 로직을 `internal/run` 공유 패키지로 추출해 run·logs 가 함께 쓴다. 분할 불가 환경은 run 과 동일하게 merged 로 graceful degrade + 안내.

### 연결 실패 처리

eb ssh 는 SG 소스 제한이 있으면 포트를 열지 않고 접속을 시도하며, 현재 IP 가 SG 에 없으면 SSH 타임아웃난다. uq 는:

1. **eb/ssh 원본 경고·에러를 그대로 전달** + 한 줄 힌트:
   `✗ SSH 접속 실패 — 현재 IP 가 보안그룹에 허용돼 있지 않을 수 있습니다. SG 에 IP 추가/VPN 확인. (uq 는 SG 를 자동 변경하지 않음)`
2. **인스턴스별 격리** — merged 에서 한 인스턴스가 실패해도 나머지 스트림은 계속. `[<env>#k] 접속 실패` 표시 후 그 프로세스만 종료, 전체는 유지.
3. **빠른 실패** — `eb ssh ... -o "ConnectTimeout=10"` 으로 ssh 연결 타임아웃을 단축(eb ssh 의 ssh 옵션 전달 가능 여부 구현 시 확인; 불가하면 문서화).

`--force`(SG 임시 개방)는 **노출하지 않는다** — 인프라/보안 변경이므로 uq 책임 밖.

### split 로직 추출(소폭 리팩터)

`internal/cmd/run/split.go` 의 패널 타입·열기 함수(`panel`, `openPanel`, `openCmuxPanel`, `openITerm2Panel`, `openGhosttyPanel`, `openAppleTerminalPanel`, 셸 인용 헬퍼)를 `internal/run/panel.go` 로 옮겨 export 한다.

- `run.Panel{ Label, Command, Dir }` + `run.OpenPanel(mux, panel, splitDir)` 공개.
- `internal/cmd/run/split.go` 는 기존 `buildPanels` 결과를 `run.Panel` 로 변환해 호출(동작 불변).
- `internal/logs` 는 인스턴스마다 `run.Panel{Label, Command: eb ssh ...}` 을 만들어 동일 함수 사용.

## 도구 의존성 — doctor 확장

`eb` CLI 를 doctor 의 추적 도구에 추가(`internal/cmd/doctor/doctor.go`):

```go
{ Name: "eb", Fix: "pip install awsebcli", Run: versionCheck("eb", []string{"--version"}, `EB CLI (\S+)`) }
```

## 디렉토리 구조 (Phase 4에 추가/변경)

```
internal/cmd/logs/logs.go          [변경] stub → cobra 와이어링·플래그·오케스트레이션·aws 사전확인·국가/환경 TUI
internal/logs/source.go            [신규] Source 인터페이스 + Target/Instance
internal/logs/eb.go                [신규] EB 드라이버 (describe-environments/-resources + eb ssh)
internal/logs/eb_test.go           [신규] TailArgs 명령 구성·환경/인스턴스 파싱 단위테스트
internal/logs/select.go            [신규] 국가/환경 선택(위치인자 매칭 + huh TUI)
internal/logs/select_test.go       [신규] 위치인자→국가/필터 해석 단위테스트
internal/logs/mux.go               [신규] merged 멀티플렉스 출력 + --grep 필터
internal/logs/mux_test.go          [신규] prefix·grep 필터 단위테스트
internal/repocfg/repocfg.go        [변경] LogsConfig/CountryTarget 파싱 + LogsFor 접근자
internal/repocfg/repos.yml         [변경] logs: 블록 추가 (forceteller-api)
internal/repocfg/repocfg_test.go   [변경] logs 파싱·기본값 테스트
internal/run/panel.go              [신규] split.go 에서 추출한 공유 패널 로직 (export)
internal/cmd/run/split.go          [변경] run.OpenPanel 사용 (동작 불변)
internal/cmd/doctor/doctor.go      [변경] eb 도구 추적 추가
docs/0004-phase4-logs-eb.md        [신규] 본 문서
```

## 에러 코드 사용

- **exit 4** — aws 인증 미충족(`uq auth login --aws-only` 안내). repo 명령의 gh 사전확인과 동일 패턴.
- **exit 2** — 사용법 에러(인자 개수, `--split`+`--no-follow` 상호 배타 등 cobra 처리).
- **exit 1** — 런타임 에러: logs 미등록 레포 / 알 수 없는 국가 / 비TTY 모호 / 매칭 환경 0개 / 인스턴스 0개 / aws·eb 실행 실패.

## 검증 (Verification)

```bash
# 빌드 + 단위 테스트
go build ./...
go test ./internal/logs/... ./internal/repocfg/... ./internal/run/...

# --help / 사용법
uq logs --help                         # 플래그·예시 노출 (stub 문구 없음)
uq logs                                # exit 2 (인자 필요)

# allowlist: 미등록 레포
uq logs forceteller-app                # exit 1, "logs 미등록" 안내 (logs 블록 없음)

# 사전확인: aws 미인증 시 → exit 4, "uq auth login --aws-only" 안내

# 계획만 출력 (실제 스트리밍 없이) — 핵심 자동 검증 경로
uq logs forceteller-api kr beta --dry-run
#   → 국가 kr=(kr-forceteller-api, ap-northeast-2), 발견 환경에서 beta 필터 → api-beta-kr-j21,
#     인스턴스(#1..#N), 각 eb ssh --region ap-northeast-2 명령 출력, exit 0
uq logs forceteller-api kr beta --instance 1 --dry-run   # 인스턴스 1개로 한정
uq logs forceteller-api kr zzz --dry-run                 # exit 1, 필터에 맞는 환경 없음
uq logs forceteller-api xx --dry-run                     # exit 1, 알 수 없는 국가

# 플래그 충돌
uq logs forceteller-api kr beta --split --no-follow --dry-run   # exit 2 (상호 배타)

# split 감지 (현재 세션 cmux)
uq logs forceteller-api kr beta --split --dry-run        # 감지 결과 + 패널별 명령 표시

# 분할 불가 환경 → merged fallback 안내
env -u TMUX -u CMUX_PANEL_ID -u CMUX_SOCKET_PATH TERM_PROGRAM=Apple_Terminal \
  uq logs forceteller-api kr beta --split --dry-run
```

> 주의: 실제 follow 스트리밍은 EB·SSH·aws 자격증명이 필요해 자동 검증에서 제외한다. 자동 검증은 `--dry-run` 과 단위 테스트로만 한다. `--dry-run` 도 환경/인스턴스 발견에 aws 호출이 필요하므로, 발견 단계는 테스트에서 fake Source 로 주입한다.

## 명시적으로 제외 (Phase 4)

- **`--since` 시간 기반 필터** — 파일 tail 의 시간 필터는 로그 포맷마다 달라 신뢰성이 낮다. MVP 는 실시간 follow + 최근 100줄 백로그만. (CloudWatch 도입 시 자연 해결.)
- **CloudWatch Logs / SSM transport** — 이번 MVP 는 `eb ssh` 만. 서버사이드 필터·인스턴스 id 타겟팅·인프라 무중단은 후속 Phase 에서 `Source` 드라이버 추가로 확장(CLI 불변).
- **EB 외 소스(ecs/k8s/local)** — 인터페이스만 열어두고 드라이버 미구현.
- **세션 중 인스턴스 재발견** — 인스턴스는 시작 시 1회 스냅샷. follow 중 오토스케일 증감분은 반영 안 함(재실행 필요). 주기적 재발견은 후속.
- **국가별 동시 스트리밍** — 한 번에 한 국가(=한 region/app). 여러 국가 동시 보기는 범위 밖.
- **로그 경로 per-환경/멀티 파일** — 레포당 단일 `path`. 여러 파일 동시 tail 은 범위 밖.
- **SG 자동 개방(`--force`/`--open-sg`)** — uq 는 보안그룹을 변경하지 않는다. IP 미허용 시 안내만.
- **분할 패널 생명주기 관리** — 패널 일괄 종료/정리는 run 과 동일하게 범위 밖.
- **application/환경의 region-cross 발견** — region 은 국가 설정에서 옴. 한 국가가 여러 region 에 걸치는 경우는 범위 밖.
