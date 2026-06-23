<a id="readme-top"></a>

<div align="center">
  <h1>UN7QI3 CLI (<code>uq</code>)</h1>
  <p>레포 · 로컬 실행 · 배포 · 운영을 묶은 un7qi3 사내 CLI</p>
  <a href="https://github.com/un7qi3inc/un7qi3-cli/releases">릴리스</a>
  &middot;
  <a href="https://github.com/un7qi3inc/un7qi3-cli/issues/new?template=bug_report.yml">버그 리포트</a>
  &middot;
  <a href="https://github.com/un7qi3inc/un7qi3-cli/issues/new?template=feature_request.yml">기능 요청</a>
</div>

<br />

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white" alt="Go 1.25" />
  <img src="https://img.shields.io/badge/cobra-CLI-1A1A1A?logo=go&logoColor=white" alt="cobra" />
  <img src="https://img.shields.io/badge/charm-Bubble%20Tea-FF62B6" alt="charmbracelet" />
  <img src="https://img.shields.io/badge/release-GoReleaser-317CBD?logo=goreleaser&logoColor=white" alt="GoReleaser" />
  <img src="https://img.shields.io/badge/commits-Conventional-FE5196?logo=conventionalcommits&logoColor=white" alt="Conventional Commits" />
</p>

---

<details>
  <summary>목차</summary>
  <ol>
    <li><a href="#소개">소개</a></li>
    <li><a href="#기술-스택">기술 스택</a></li>
    <li>
      <a href="#시작하기">시작하기</a>
      <ul>
        <li><a href="#사전-요구사항">사전 요구사항</a></li>
        <li><a href="#설치">설치</a></li>
        <li><a href="#최초-설정">최초 설정</a></li>
      </ul>
    </li>
    <li><a href="#명령어">명령어</a></li>
    <li>
      <a href="#설정">설정</a>
      <ul>
        <li><a href="#사용자-설정-configyml">사용자 설정 (config.yml)</a></li>
        <li><a href="#레포-메타데이터-reposyml">레포 메타데이터 (repos.yml)</a></li>
      </ul>
    </li>
    <li><a href="#프로젝트-구조">프로젝트 구조</a></li>
    <li>
      <a href="#개발">개발</a>
      <ul>
        <li><a href="#빌드--테스트">빌드 / 테스트</a></li>
        <li><a href="#새-명령-추가하기">새 명령 추가하기</a></li>
      </ul>
    </li>
    <li>
      <a href="#릴리스--배포">릴리스 / 배포</a>
      <ul>
        <li><a href="#릴리스-흐름">릴리스 흐름</a></li>
        <li><a href="#업그레이드">업그레이드</a></li>
      </ul>
    </li>
    <li><a href="#기여">기여</a></li>
  </ol>
</details>

---

## 소개

`uq` 는 레포 셋업, 로컬 개발 서버 실행, 시크릿, 배포, 로그 스트리밍을 하나의 명령 트리로 묶은 un7qi3 사내 CLI다.

명령에 따라 `--json`(머신 친화 출력) / `--dry-run`(실행 없이 계획만) 플래그를 제공해 Claude Code 같은 에이전트가 호출할 수 있고, TTY 에서는 컬러·TUI·대화형 선택을 제공해 사람이 직접 써도 된다.

기능:

- **환경 점검** (`init` / `doctor`) — 필요한 툴·인증 점검, 워크스페이스 지정
- **인증** (`auth`) — gh · aws · gcloud 로그인/로그아웃/상태를 한 명령으로
- **레포** (`repo`) — un7qi3inc 조직 레포 목록·클론·풀 (gh 토큰으로 git 처리, SSH 키 불필요)
- **로컬 실행** (`run`) — 레포별 node 버전·env·실행 명령 프로파일, 멀티프로세스 로그 합치기·패널 분할
- **배포** (`deploy`) — `.uq.yml` 매니페스트 기반 배포 (stub)
- **로그** (`logs`) — Elastic Beanstalk 다중 인스턴스 로그 멀티플렉스 스트리밍 (TUI / 패널 분할 / grep)
- **자체 관리** (`version` / `update`) — 버전 표시, GitHub Releases 에서 제자리 업그레이드

<p align="right"><a href="#readme-top">맨 위로</a></p>

---

## 기술 스택

| 영역 | 기술 |
|------|------|
| **언어** | Go 1.25 |
| **CLI 프레임워크** | [cobra](https://github.com/spf13/cobra) |
| **TUI / 출력** | [charmbracelet](https://charm.sh) — Bubble Tea, Bubbles, Huh, Lip Gloss |
| **설정** | YAML (`gopkg.in/yaml.v3`) |
| **빌드 / 배포** | Makefile · [GoReleaser](https://goreleaser.com) · [release-please](https://github.com/googleapis/release-please) · GitHub Actions |

<p align="right"><a href="#readme-top">맨 위로</a></p>

---

## 시작하기

### 사전 요구사항

- **Go** 1.25+ (소스 빌드 시)
- **gh** (GitHub CLI) — 레포/릴리스 접근에 필수
- 그 외 역할별 툴(aws, gcloud, node, docker 등)은 `uq doctor` 로 점검

### 설치

```bash
git clone https://github.com/un7qi3inc/un7qi3-cli.git
cd un7qi3-cli

make install          # go build → ~/.local/bin/uq
uq version            # 동작 확인
```

`make install` 은 `PATH` 에 잡힐 수 있는 위치(`~/.local/bin`, `/usr/local/bin`, `~/go/bin`)의 기존 `uq` 를 먼저 제거해 중복 설치를 막는다. 설치 경로는 `make install PREFIX=/usr/local` 로 바꾼다. `~/.local/bin` 이 `PATH` 에 없으면 추가한다.

### 최초 설정

```bash
uq init               # 인증(gh) 점검 + 워크스페이스 위치 결정
uq doctor             # 역할별 툴 상태 점검
```

`uq init` 은 기존 un7qi3 레포 위치를 스캔해 후보로 제시하고, 선택 결과를 `~/.config/un7qi3/config.yml` 의 `repos_dir` 에 저장한다.

<p align="right"><a href="#readme-top">맨 위로</a></p>

---

## 명령어

전체 트리는 `uq --help`, 각 명령 상세는 `uq <명령> --help`.

| 그룹 | 명령 | 설명 |
|------|------|------|
| **시작하기** | `uq init` | 최초 설정 (인증 점검 + 워크스페이스 위치) |
| | `uq doctor [--role …] [--json]` | 외부 툴 설치/인증 점검 (역할 필터) |
| | `uq auth login\|logout\|status` | gh · aws · gcloud 통합 인증 |
| **개발 워크플로** | `uq repo list\|clone\|pull` | 조직 레포 작업 (TUI 다중 선택) |
| | `uq run <repo>[:profile]` | 로컬 개발 서버 실행 (`--bg`/`--fg`/`--split`/`--dry-run`) |
| | `uq run profiles [--json]` | 등록된 실행 프로파일 나열 |
| **배포 & 운영** | `uq deploy run <repo> --env <env>` | `.uq.yml` 기반 배포 (stub) |
| | `uq logs <대상> [국가] [환경]` | EB 다중 인스턴스 로그 스트리밍 (TUI / `--split` / `--grep`) |
| **도구** | `uq version [--json]` | 버전 / 커밋 / 빌드 시각 |
| | `uq update` (= `upgrade`) | GitHub Releases 최신 버전으로 제자리 업그레이드 |
| | `uq completion <shell>` | 셸 자동완성 스크립트 생성 (숨김) |

```bash
uq run forceteller-app                 # default 프로파일로 로컬 실행
uq run forceteller-admin --split       # back + front 패널 분할
uq logs forceteller-api kr beta        # kr beta 전체 인스턴스 로그
uq doctor --role frontend --json       # 프런트 역할 툴 점검을 JSON 으로
uq repo clone                          # 인자 없으면 TUI 다중 선택
```

전역 플래그: `--verbose, -v` (실행하는 외부 명령 출력), `--help, -h`. `--json` 은 지원 명령에만 로컬로 존재한다.

<p align="right"><a href="#readme-top">맨 위로</a></p>

---

## 설정

### 사용자 설정 (config.yml)

사용자 설정은 `~/.config/un7qi3/config.yml`(`$XDG_CONFIG_HOME` 존중)에 저장된다.

| 키 | 의미 |
|----|------|
| `repos_dir` | 레포를 클론할 워크스페이스 경로 |

레포 디렉터리 결정 우선순위:

```
$UQ_REPOS_DIR  >  config.yml 의 repos_dir  >  기본값 ~/un7qi3
```

### 레포 메타데이터 (repos.yml)

레포별 브랜치·실행 프로파일은 바이너리에 임베드된 [`internal/repocfg/repos.yml`](internal/repocfg/repos.yml) 에서 관리한다. 수정 후 `make install` 로 재빌드하면 반영된다.

```yaml
repos:                         # uq repo pull 이 순회할 브랜치 (첫 번째 = primary)
  forceteller-api: [develop, master]
defaults: [main]               # 위에 없는 레포의 fallback

runs:                          # uq run <repo>[:profile] 실행 프로파일
  forceteller-app:
    default: app3              # 프로파일 생략 시 기본값
    profiles:
      app:
        cmd: ["yarn", "start"] # 셸 해석 없이 argv 그대로
        node: ">=18"           # fnm → nvm → mise → asdf → PATH 순으로 자동 탐색
        url: "http://localhost:3000"
```

배포 매니페스트 `.uq.yml` 은 각 레포 루트에 두며 `uq deploy` 가 읽는다.

<p align="right"><a href="#readme-top">맨 위로</a></p>

---

## 프로젝트 구조

```
├── cmd/uq/                   # main 진입점 (context + signal 처리)
├── internal/
│   ├── cmd/                    # 명령 정의 (cobra 트리)
│   │   ├── root.go               # 루트 명령 + 그룹/템플릿 + 디스패처
│   │   ├── init/ doctor/ auth/   # 시작하기 그룹
│   │   ├── repo/ run/ env/       # 개발 워크플로 그룹
│   │   ├── deploy/ log/          # 배포 & 운영 그룹
│   │   └── version/ update/ skills/  # 도구 그룹
│   ├── auth/                   # gh / aws / gcloud 인증
│   ├── run/                    # 로컬 실행 (node 매니저 탐색, 포트, 패널)
│   ├── log/                    # 로그 멀티플렉서 + TUI 뷰어
│   ├── exec/                   # 외부 프로세스 실행 래퍼 (--verbose)
│   ├── clierr/                 # 종료 코드를 담는 CLI 에러 타입
│   ├── output/                 # 컬러/헤딩/TTY 감지 + JSON 출력
│   ├── config/                 # 사용자 설정 (config.yml)
│   ├── repocfg/                # 임베드된 repos.yml 로더
│   ├── manifest/               # .uq.yml 배포 매니페스트
│   └── version/                # ldflags 주입 버전 메타데이터
├── docs/                     # Phase 설계/계획 문서 (0001~0005)
├── .github/workflows/        # release-please + GoReleaser + PR 검증
├── .goreleaser.yaml          # 릴리스 빌드 설정
└── Makefile                  # build / install / test / lint
```

<p align="right"><a href="#readme-top">맨 위로</a></p>

---

## 개발

### 빌드 / 테스트

```bash
make build            # bin/uq 로 빌드 (설치 없이)
make install          # 빌드 + ~/.local/bin 에 설치
make test             # go test ./...
make lint             # go vet ./...
make clean            # bin/ 정리
```

`repos.yml` 같은 임베드 자산을 바꾸면 `make install` 로 재빌드해야 반영된다.

### 새 명령 추가하기

1. `internal/cmd/<name>/` 에 패키지를 만들고 `func NewCmd() *cobra.Command` 를 노출한다.
2. `internal/cmd/root.go` 의 `init()` 에서 `rootCmd.AddCommand(inGroup(<name>.NewCmd(), group…))` 로 그룹에 등록한다.
3. 사용자 대면 문자열은 한글로, `output.Heading` / `output.HelpExample` 헬퍼를 쓴다.
4. 머신 출력이 필요하면 `--json` 플래그를 로컬로 추가한다 (전역 아님).
5. 안정화 전 명령은 `cmd.Hidden = true` 로 `--help` 목록에서 숨긴다 (`env`, `skills` 참고).

인자 없이 호출된 명령은 `root.go` 의 `helpOnEmptyArgs` 가 에러(exit 2) 대신 자기 도움말(exit 0)을 출력하게 한다.

<p align="right"><a href="#readme-top">맨 위로</a></p>

---

## 릴리스 / 배포

릴리스는 release-please + GoReleaser + GitHub Actions 로 자동화돼 있다. 버전 번호는 손으로 올리지 않는다.

### 릴리스 흐름

```
Conventional Commit (feat/fix…) ─▶ main 머지
        │
        ▼
release-please 가 "Release PR" 생성/갱신 (CHANGELOG + 버전 bump)
        │
        ▼  Release PR 머지
태그 + GitHub Release 생성 ─▶ GoReleaser 가 darwin amd64/arm64 바이너리 첨부
```

- PR 제목은 Conventional Commit 형식이어야 한다 (`pr-title` 워크플로가 검사). squash 머지 시 PR 제목이 main 커밋이 되어 release-please 가 읽는다.
- CHANGELOG 정본은 release-please 가 만드는 [`CHANGELOG.md`](CHANGELOG.md) 다.

### 업그레이드

```bash
uq update             # = uq upgrade. GitHub Releases 최신 버전으로 제자리 교체
```

비공개 레포 에셋을 받기 위해 `gh` 인증을 사용한다.

<p align="right"><a href="#readme-top">맨 위로</a></p>

---

## 기여

```
feature/xxx ─┐
fix/xxx ─────┼──▶ PR (Conventional 제목) ──▶ main (squash merge)
refactor/xxx ┘
```

- `main` 직접 push 금지 — PR squash 머지만 사용한다.
- 커밋 메시지는 한글, AI 표기(Co-Authored-By 등) 금지.
- 커밋 타입(`feat`/`fix`/`refactor`/`chore`…)이 릴리스 버전 bump 를 결정한다.
- 수정 후 `make test` · `make lint` · `make install` 로 검증하고 PR 을 올린다.

<p align="right"><a href="#readme-top">맨 위로</a></p>
