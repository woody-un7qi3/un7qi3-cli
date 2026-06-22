<a id="readme-top"></a>

<div align="center">
  <h1>UN7QI3 CLI (<code>uq</code>)</h1>
  <p>온보딩 · 레포 · 로컬 실행 · 배포 · 운영을 하나로 묶은 un7qi3 사내 CLI</p>
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
    <li><a href="#주요-기능">주요 기능</a></li>
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
        <li><a href="#설계-원칙">설계 원칙</a></li>
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

`uq` 는 un7qi3 개발자의 일상 작업 — 온보딩, 레포 셋업, 로컬 개발 서버 실행, 시크릿, 배포, 로그 스트리밍 — 을 하나의 명령 트리로 묶은 사내 CLI입니다.

설계 목표는 **결정론(determinism)** 입니다. 모든 명령은 입력이 같으면 출력이 같고, `--json` / `--dry-run` 같은 머신 친화 플래그를 제공합니다. 그래서 주로 **Claude Code 같은 LLM 에이전트가 호출**하지만, 사람이 직접 써도 편하도록 컬러·TUI·대화형 선택을 갖췄습니다.

<p align="right"><a href="#readme-top">맨 위로</a></p>

---

## 주요 기능

- **온보딩 (`init` / `doctor`)** — 새 머신/팀원이 필요한 툴·인증을 한 번에 점검하고 워크스페이스를 잡습니다.
- **통합 인증 (`auth`)** — `gh` · `aws` · `gcloud` 세 provider의 로그인/로그아웃/상태를 한 명령으로 관리합니다.
- **레포 작업 (`repo`)** — un7qi3inc 조직 레포 목록·클론·풀. SSH 키 없이 `gh` 토큰으로 모든 git 작업을 처리합니다.
- **로컬 실행 (`run`)** — 레포별 node 버전·env·실행 명령을 통일된 프로파일로 띄웁니다. 멀티프로세스 프로파일은 로그를 합쳐 보여주고, 패널 분할(`--split`)도 지원합니다.
- **배포 (`deploy`)** — 레포 루트 `.uq.yml` 매니페스트 기반 배포 워크플로 (현재 stub).
- **로그 스트리밍 (`logs`)** — Elastic Beanstalk 다중 인스턴스 로그를 멀티플렉스로, TUI 뷰어 또는 인스턴스별 패널로 스트리밍합니다.
- **자체 관리 (`version` / `update`)** — 빌드 메타데이터 표시, GitHub Releases 에서 제자리 업그레이드.

> 에이전트·자동화용으로는 `--json`, 사람용으로는 컬러/TUI 출력을 기본으로 합니다. TTY가 아니면 평문으로 자동 강등됩니다.

<p align="right"><a href="#readme-top">맨 위로</a></p>

---

## 기술 스택

| 영역 | 기술 |
|------|------|
| **언어** | Go 1.25 |
| **CLI 프레임워크** | [cobra](https://github.com/spf13/cobra) (명령 트리 / 플래그) |
| **TUI / 출력** | [charmbracelet](https://charm.sh) — Bubble Tea, Bubbles, Huh, Lip Gloss |
| **설정** | YAML (`gopkg.in/yaml.v3`) |
| **빌드 / 배포** | Makefile · [GoReleaser](https://goreleaser.com) · [release-please](https://github.com/googleapis/release-please) · GitHub Actions |

<p align="right"><a href="#readme-top">맨 위로</a></p>

---

## 시작하기

### 사전 요구사항

- **Go** 1.25+ (소스 빌드 시)
- **gh** (GitHub CLI) — 레포/릴리스 접근에 필수
- 그 외 역할별 툴(aws, gcloud, node, docker 등)은 `uq doctor` 가 점검해 줍니다

### 설치

```bash
git clone https://github.com/un7qi3inc/un7qi3-cli.git
cd un7qi3-cli

make install          # go build → ~/.local/bin/uq (기존 설치본은 자동 정리)
uq version            # 동작 확인
```

> `make install` 은 `PATH` 에 잡힐 수 있는 흔한 위치(`~/.local/bin`, `/usr/local/bin`, `~/go/bin`)의 기존 `uq` 를 먼저 지워 **중복 설치를 방지**합니다.
> 설치 경로를 바꾸려면 `make install PREFIX=/usr/local` 처럼 `PREFIX` 를 넘깁니다. `~/.local/bin` 이 `PATH` 에 없다면 추가하세요.

### 최초 설정

```bash
uq init               # 인증(gh) 점검 + 워크스페이스 위치 결정
uq doctor             # 내 역할에 필요한 툴 상태 점검
```

`uq init` 은 기존 un7qi3 레포 위치를 자동 스캔해 후보로 제시하고, 선택 결과를 `~/.config/un7qi3/config.yml` 에 저장합니다.

<p align="right"><a href="#readme-top">맨 위로</a></p>

---

## 명령어

전체 트리는 `uq --help`, 각 명령 상세는 `uq <명령> --help` 로 확인합니다.

| 그룹 | 명령 | 설명 |
|------|------|------|
| **시작하기** | `uq init` | 최초 설정 (인증 점검 + 워크스페이스 위치) |
| | `uq doctor [--role …] [--json]` | 필수 외부 툴 설치/인증 점검 (역할 필터 가능) |
| | `uq auth login\|logout\|status` | gh · aws · gcloud 통합 인증 |
| **개발 워크플로** | `uq repo list\|clone\|pull` | un7qi3inc 조직 레포 작업 (TUI 다중 선택) |
| | `uq run <repo>[:profile]` | 레포 로컬 개발 서버 실행 (`--bg`/`--fg`/`--split`/`--dry-run`) |
| | `uq run profiles [--json]` | 등록된 실행 프로파일 나열 |
| **배포 & 운영** | `uq deploy run <repo> --env <env>` | `.uq.yml` 기반 배포 (stub) |
| | `uq logs <대상> [국가] [환경]` | EB 다중 인스턴스 로그 스트리밍 (TUI / `--split` / `--grep`) |
| **도구** | `uq version [--json]` | 버전 / 커밋 / 빌드 시각 |
| | `uq update` (= `upgrade`) | GitHub Releases 최신 버전으로 제자리 업그레이드 |
| | `uq completion <shell>` | 셸 자동완성 스크립트 생성 (숨김) |

```bash
# 자주 쓰는 예시
uq run forceteller-app                 # default 프로파일로 로컬 실행
uq run forceteller-admin --split       # back + front 를 패널 분할로
uq logs forceteller-api kr beta        # kr beta 전체 인스턴스 로그
uq doctor --role frontend --json       # 프런트 역할 툴 점검 결과를 JSON 으로
uq repo clone                          # 인자 없으면 TUI 다중 선택
```

> 전역 플래그: `--verbose, -v` (실행하는 외부 명령 출력), `--help, -h`. `--json` 은 지원하는 명령에만 로컬로 존재합니다.

<p align="right"><a href="#readme-top">맨 위로</a></p>

---

## 설정

### 사용자 설정 (config.yml)

머신마다 다른 사용자 설정은 `~/.config/un7qi3/config.yml`(`$XDG_CONFIG_HOME` 존중)에 저장됩니다.

| 키 | 의미 |
|----|------|
| `repos_dir` | 레포를 클론할 워크스페이스 경로 |

레포 디렉터리 결정 우선순위:

```
$UQ_REPOS_DIR  >  config.yml 의 repos_dir  >  기본값 ~/un7qi3
```

### 레포 메타데이터 (repos.yml)

레포별 브랜치·실행 프로파일은 바이너리에 **임베드**된 [`internal/repocfg/repos.yml`](internal/repocfg/repos.yml) 에서 관리합니다. 수정 후 `make install` 로 재빌드하면 즉시 반영됩니다.

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

> 배포 매니페스트 `.uq.yml` 은 각 레포 루트에 두며 `uq deploy` 가 읽습니다.

<p align="right"><a href="#readme-top">맨 위로</a></p>

---

## 프로젝트 구조

```
├── cmd/uq/                   # main 진입점 (context + signal 처리, Execute 호출)
├── internal/
│   ├── cmd/                    # 명령 정의 (cobra 트리)
│   │   ├── root.go               # 루트 명령 + 그룹/템플릿 + 디스패처
│   │   ├── init/ doctor/ auth/   # 시작하기 그룹
│   │   ├── repo/ run/ env/       # 개발 워크플로 그룹
│   │   ├── deploy/ log/          # 배포 & 운영 그룹
│   │   └── version/ update/ skills/  # 도구 그룹
│   ├── auth/                   # gh / aws / gcloud 인증 로직
│   ├── run/                    # 로컬 실행 (node 매니저 탐색, 포트, 패널)
│   ├── log/                    # 로그 멀티플렉서 + TUI 뷰어
│   ├── exec/                   # 외부 프로세스 실행 래퍼 (--verbose 지원)
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

> 코드를 수정했으면 동작 확인 전 **`make install`** 로 재빌드하는 것이 기본 루틴입니다 (특히 `repos.yml` 같은 임베드 자산 변경 시).

### 새 명령 추가하기

1. `internal/cmd/<name>/` 에 패키지를 만들고 `func NewCmd() *cobra.Command` 를 노출합니다.
2. `internal/cmd/root.go` 의 `init()` 에서 알맞은 그룹으로 `rootCmd.AddCommand(inGroup(<name>.NewCmd(), group…))` 등록합니다.
3. 사용자 대면 문자열(`Short`/`Long`/예시)은 한글로, `output.Heading` / `output.HelpExample` 헬퍼를 사용합니다.
4. 머신 출력이 필요하면 `--json` 플래그를 **로컬**로 추가합니다(전역 아님).
5. 안정화 전 명령은 `cmd.Hidden = true` 로 `--help` 목록에서 숨겨둡니다 (`env`, `skills` 참고).

> 인자 없이 호출된 명령은 `root.go` 의 `helpOnEmptyArgs` 덕분에 에러(exit 2) 대신 자기 도움말(exit 0)을 출력합니다.

### 설계 원칙

- **결정론** — 같은 입력 → 같은 출력. 사이드이펙트는 명시적으로.
- **에이전트 우선, 사람 친화** — `--json`/`--dry-run` 제공, TTY 면 컬러/TUI, 아니면 평문.
- **context 전파** — `Execute(ctx)` 가 signal(Ctrl+C/SIGTERM) 취소를 모든 `RunE` 로 전달합니다.
- **종료 코드 계약** — 에러는 `internal/clierr` 로 감싸 종료 코드를 일관되게 유지합니다.

<p align="right"><a href="#readme-top">맨 위로</a></p>

---

## 릴리스 / 배포

배포는 **release-please + GoReleaser + GitHub Actions** 로 완전 자동화돼 있습니다. 버전 번호를 손으로 올리지 않습니다.

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

- **PR 제목**은 Conventional Commit 형식이어야 합니다 (`pr-title` 워크플로가 검사). squash 머지 시 PR 제목이 곧 main 커밋이 되어 release-please 가 읽습니다.
- CHANGELOG 정본은 release-please 가 만든 한글 [`CHANGELOG.md`](CHANGELOG.md) 입니다.

### 업그레이드

```bash
uq update             # = uq upgrade. GitHub Releases 최신 버전으로 제자리 교체
```

> 비공개 레포 에셋을 받기 위해 `gh` 인증을 사용합니다.

<p align="right"><a href="#readme-top">맨 위로</a></p>

---

## 기여

```
feature/xxx ─┐
fix/xxx ─────┼──▶ PR (Conventional 제목) ──▶ main (squash merge)
refactor/xxx ┘
```

- `main` 직접 push 금지 — PR squash 머지만 사용합니다.
- 커밋 메시지는 **한글**, AI 표기(Co-Authored-By 등) 금지.
- 커밋 타입(`feat`/`fix`/`refactor`/`chore`…)이 릴리스 버전 bump 를 결정하므로 정확히 선택합니다.
- 코드 수정 후 `make test` · `make lint` · `make install` 로 검증한 뒤 PR 을 올립니다.

<p align="right"><a href="#readme-top">맨 위로</a></p>
