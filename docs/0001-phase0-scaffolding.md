# 0001 — Phase 0: 로컬 스캐폴딩

**Status:** 완료

## Context

un7qi3 사내에서 신규 입사자 온보딩 / 레포 셋업 / 배포 같은 반복 작업을 **LLM(Claude Code)이 호출할 수 있는 결정론적 CLI**로 통일하려는 목표.

- 프로젝트명: `un7qi3-cli`
- 바이너리명: `uq` (gh가 처음부터 `gh`인 것과 동일하게, alias가 아닌 본 이름)
- 명령 패턴: `uq <noun> <verb>` (gh 스타일)
- 호출자: 주로 Claude Code, 부차적으로 사람
- 저장소: `github.com/un7qi3inc/un7qi3-cli` (Private)
- 작업 디렉토리: `/Users/woody/un7qi3/un7qi3-cli`

## 단계 구분

이번 작업은 **Phase 0: 로컬 개발 + 로컬 설치만**으로 한정한다. 배포 인프라(GoReleaser, GitHub Actions, GitHub Releases)는 명령들이 안정화된 뒤 별도 Phase로 미룬다. 우선 본인(woody) 머신에서 충분히 써보고 명령 설계를 다듬는 게 목표.

| Phase | 범위 | 시점 |
|---|---|---|
| **0 (이번)** | 로컬 빌드 + 로컬 설치(`go install` 또는 `make install`), 핵심 명령 스캐폴딩 | 지금 |
| 1 | 다른 사내 인원 1-2명에게 공유 (수동 바이너리 전달 또는 `git clone && make install`) | 명령 설계 안정화 후 |
| 2 | GoReleaser + GitHub Releases + `uq upgrade` 자동화 | 본격 사내 배포 시점 |
| 3 | Claude Code 스킬 임베드 (`uq skills install`) — 하네스 통합 | uq 명령 굳어진 뒤 |

**저장소는 Private 유지, Homebrew는 사용하지 않음** (Phase 2에서도). Phase 2 시점의 배포 방식:
- 첫 설치: `gh release download` 한 줄
- 업데이트: `uq upgrade`
- 사내 도구 바이너리는 `uq` 하나뿐 (`forceteller-*`는 소스 프로젝트, `uq repo clone`으로 다룸)

## Phase 0 로컬 설치 방법

```bash
cd ~/un7qi3/un7qi3-cli
make install          # go build → /usr/local/bin/uq 에 복사
uq version            # 동작 확인
```

또는:
```bash
go install ./cmd/uq   # $GOPATH/bin/uq 에 설치 (PATH에 들어가있어야 함)
```

코드 수정하면 `make install` 한 번 더 → 바로 새 동작 확인. 빠른 반복.

## 설계 원칙 (LLM-callable CLI)

1. **명사-동사 일관성** — `uq repo clone`, `uq env pull`, `uq auth login`. 한 번 패턴을 보면 나머지를 추론 가능.
2. **비대화형 우선** — 모든 입력을 플래그/env로 받을 수 있어야 함. 필수 입력 누락 시 명확한 에러 코드.
3. **`--json` 출력 모드** — list/view 계열 명령은 모두 `--json` 지원. Claude가 파싱하기 위함.
4. **`--dry-run`** — 파괴적 명령(deploy, env push 등)은 dry-run 지원.
5. **자기 설명적 `--help`** — `uq --help`만 보고 Claude가 다음 행동을 결정 가능해야 함.
6. **Exit code 규칙** — 0=성공, 1=일반 에러, 2=사용법 에러, 4=인증 필요 (gh 컨벤션).

## 기술 스택

Phase 0:
- **Go 1.24** (이미 시스템에 설치됨, `go1.24.4 darwin/arm64`)
- **[cobra](https://github.com/spf13/cobra)** — gh와 동일한 명령 프레임워크
- **[viper](https://github.com/spf13/viper)** — 설정 파일(`~/.config/un7qi3/config.yml`) 로딩
- **[AWS SDK for Go v2](https://github.com/aws/aws-sdk-go-v2)** — Parameter Store/SSM 호출 (env 명령용)
- **[huh](https://github.com/charmbracelet/huh)** — TUI 다중 선택 prompt (install 명령용)

Phase 2에서 추가:
- **[GoReleaser](https://goreleaser.com/)** — 멀티플랫폼 릴리즈

## 디렉토리 구조

`gh`의 [cli/cli](https://github.com/cli/cli) 레이아웃을 단순화해서 차용.

Phase 0 디렉토리 (배포 관련 파일은 Phase 2에 추가):

```
un7qi3-cli/
├── cmd/
│   └── uq/
│       └── main.go              # 진입점. cmd.Execute() 호출만.
├── internal/
│   ├── cmd/                     # cobra 명령 트리
│   │   ├── root.go              # `uq` 루트 + 글로벌 플래그(--json, --verbose)
│   │   ├── version/version.go   # `uq version`
│   │   ├── doctor/doctor.go     # `uq doctor`
│   │   ├── install/install.go   # `uq install <team>` (TUI 다중 선택)
│   │   ├── repo/
│   │   │   ├── repo.go
│   │   │   ├── list.go
│   │   │   └── clone.go
│   │   ├── auth/
│   │   │   ├── auth.go
│   │   │   ├── login.go
│   │   │   ├── logout.go
│   │   │   └── status.go
│   │   ├── env/
│   │   │   ├── env.go
│   │   │   ├── pull.go
│   │   │   ├── push.go
│   │   │   └── diff.go
│   │   ├── deploy/
│   │   │   ├── deploy.go
│   │   │   └── run.go
│   │   ├── logs/
│   │   │   └── logs.go          # `uq logs <repo>` (EB 멀티 인스턴스 스트리밍)
│   │   ├── upgrade/upgrade.go   # Phase 2까지는 stub
│   │   └── skills/skills.go     # Phase 3까지는 stub
│   ├── config/
│   │   └── config.go            # ~/.config/un7qi3/config.yml 로딩
│   ├── manifest/
│   │   └── manifest.go          # .uq.yml (레포별 secrets/deploy/logs 선언) 파서
│   ├── output/
│   │   ├── json.go              # --json 처리 헬퍼
│   │   └── tty.go               # 사람용 출력 헬퍼
│   └── version/
│       └── version.go           # ldflags로 주입되는 version/commit/date
├── docs/
│   └── 0001-phase0-scaffolding.md  # 이 문서
├── .gitignore
├── go.mod                       # module github.com/un7qi3inc/un7qi3-cli
├── go.sum
├── Makefile                     # build/install/test/lint
└── README.md
```

Phase 2에 추가될 파일: `.goreleaser.yaml`, `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `LICENSE`.

## 초기 명령 트리 (Phase 0 스캐폴딩)

모든 verb를 cobra 명령으로 등록만 하고, 실제 로직은 Phase 0에서 최소만 구현. 트리 전체가 잡혀 있어야 Claude가 `uq --help`만 보고 가능한 작업을 파악할 수 있다.

| 명령 | Phase 0 상태 | 최종 비고 |
|---|---|---|
| `uq` | 구현 | 도움말 출력 |
| `uq version` | 구현 | ldflags 주입된 버전 출력, `--json` 지원 |
| `uq doctor` | 구현 | 필수 툴 점검 (자세히는 아래 표). `--json` 지원. |
| `uq install <team>` | stub | 팀별(backend/frontend/mobile) 레포 다중 선택 클론. TUI(huh) + `--all`/`--select`/`--list --json`. |
| `uq repo list` | stub | `un7qi3inc` org 레포 목록. `--json`. |
| `uq repo clone <name>` | stub | `~/un7qi3/<name>`에 클론. |
| `uq auth login` | stub | gh + aws sso 일괄 로그인. |
| `uq auth status` | stub | 모든 인증 상태. `--json`. |
| `uq auth logout` | stub | |
| `uq env pull <repo>` | stub | `.uq.yml` 매니페스트 참조하여 AWS SSM에서 .env/.pem/.json 받아 표준 위치에 떨굼. AWS SDK 직접 호출. |
| `uq env push <repo>` | stub | 로컬 → SSM 업로드. dry-run 기본. |
| `uq env diff <repo>` | stub | 로컬 vs SSM 비교. |
| `uq deploy run <repo>` | stub | `.uq.yml` 매니페스트 기반. 1차: `./build-<env>.sh` 컨벤션. 2차: 매니페스트 cmd/requires/pre/confirm 확장. `--env`, `--dry-run`, `--yes`. |
| `uq logs <repo>` | stub | EB 인스턴스 멀티플렉스 로그 스트리밍. `--env`, `--instance`, `--since`, `--grep`, `--no-follow`, `--split`. |
| `uq upgrade` | **Phase 2 stub** | Phase 2에서 구현. Phase 0에선 "not yet released" 안내. |
| `uq skills install` | **Phase 3 stub** | 하네스 통합. Claude Code 스킬을 `~/.claude/skills/`에 복사. Phase 0에선 stub. |

## `uq doctor` 점검 항목

| 항목 | 체크 방법 | 누락 시 가이드 |
|---|---|---|
| `git` | `git --version` | macOS는 Xcode CLT (`xcode-select --install`) |
| `gh` | `gh --version` + `gh auth status` | `brew install gh && gh auth login` |
| `node` | `node --version` | `brew install node` 또는 nvm/fnm 안내 |
| `sdkman` | `~/.sdkman/bin/sdkman-init.sh` 존재 | `curl -s "https://get.sdkman.io" \| bash` |
| `java` | `java -version` | `sdk install java` (SDKMAN! 통한 설치 권장) |
| `aws` | `aws --version` | `brew install awscli` |
| `gcloud` | `gcloud --version` | `brew install --cask google-cloud-sdk` |
| `docker` | `docker --version` + 데몬 응답 | Docker Desktop 안내 (optional) |

출력 예시:
```
$ uq doctor
✓ git       2.43.0
✓ gh        2.86.0 (authenticated as woody-un7qi3)
✓ node      20.10.0
✓ sdkman    5.18.0
✗ java      not installed
            → sdk install java
✓ gcloud    458.0.0
- docker    not checked (optional)

1 issue. Run the suggested command above, then `uq doctor` again.
```

`--json` 출력:
```json
{
  "checks": [
    {"name": "git", "ok": true, "version": "2.43.0"},
    {"name": "java", "ok": false, "fix": "sdk install java"}
  ],
  "summary": {"ok": 5, "failed": 1, "optional": 1}
}
```

## 글로벌 플래그 (root.go)

- `--json` — 가능한 모든 명령에서 JSON 출력
- `--verbose, -v` — 디버그 로그
- `--config <path>` — 설정 파일 경로 오버라이드

## 문서 관리 규칙

- 모든 phase/계획 문서는 `docs/`에 **번호 매긴 markdown**으로 저장 (`docs/0001-phase0-scaffolding.md`, `docs/0002-...` 등).
- 각 문서 최상단에 **Status 필드**: `시작전` / `진행중` / `완료`.
- **문서 본문은 작성된 후 수정하지 않는다.** 상태 변경(`Status:` 라인) 외에는 불변. 변경이 필요하면 새 번호의 후속 문서 작성.
- 이는 ADR(Architecture Decision Record) / RFC 패턴과 동일.

## 실행 순서 (Phase 0)

1. **로컬 초기화**
   - `git init` in `/Users/woody/un7qi3/un7qi3-cli`
   - `go mod init github.com/un7qi3inc/un7qi3-cli`
   - cobra/viper/huh/aws-sdk 의존성 추가 (실제 import는 점진적으로)
   - 시작 시점에 이 문서의 Status를 `진행중`으로 변경
2. **스캐폴딩 생성**
   - 위 디렉토리 구조 전체 생성
   - `cmd/uq/main.go` 진입점
   - `internal/cmd/root.go` + 글로벌 플래그(`--json`, `--verbose`, `--config`)
   - 각 명령 파일들 (실제 구현 3개: version/doctor/help, 나머지는 stub)
   - `Makefile` (build/install/test/lint), `.gitignore`, `README.md`
3. **빌드 + 로컬 설치 + 검증**
   - `make install` → `/usr/local/bin/uq`
   - 검증 섹션의 모든 명령이 동작하는지 확인
   - 통과하면 이 문서의 Status를 `완료`로 변경
4. **원격 저장소 (선택)**
   - 본인 백업/원격 동기화가 필요하면: `gh repo create un7qi3inc/un7qi3-cli --private --source=. --remote=origin --push`
   - 안 해도 Phase 0 진행에는 지장 없음.

Phase 2 작업(나중에): `docs/0002-phase2-release-infra.md` 작성 → `.goreleaser.yaml`, `.github/workflows/ci.yml`, `release.yml` 추가, `uq upgrade` 구현.

## 검증 (Verification)

스캐폴딩 완료 후 다음으로 동작 확인:

```bash
cd /Users/woody/un7qi3/un7qi3-cli

# 빌드 + 로컬 설치
make install
which uq                           # /usr/local/bin/uq

# 기본 명령 (실제 동작)
uq --help                          # 명령 트리가 다 보여야 함 (install/repo/auth/env/deploy/logs/upgrade/skills)
uq version                         # 사람 친화 출력
uq version --json                  # {"version":"dev","commit":"...","date":"..."}
uq doctor                          # git/gh/node/sdkman/java/aws 등 점검
uq doctor --json                   # 구조화 결과

# Stub 명령 (메시지만)
uq install backend                 # "TODO: not yet implemented" + exit 0
uq repo list                       # stub
uq auth status                     # stub
uq env pull forceteller-api        # stub
uq deploy run forceteller-api --env beta --dry-run   # stub
uq logs forceteller-api --env prod # stub
uq upgrade                         # "Phase 2: not yet released"

# 잘못된 사용
uq repo clone                      # exit 2 (사용법 에러, name 인자 누락)
uq nonexistent                     # exit 2 (알 수 없는 명령)

# 빠른 반복: 코드 수정 → make install → 바로 확인
```

## 명시적으로 제외 (Phase 0)

- **배포 인프라** — GoReleaser, GitHub Actions, GitHub Releases, `uq upgrade` 실제 동작. 모두 Phase 2.
- **하네스/스킬** — `uq skills install`은 stub만. Phase 3에서 채움.
- **실제 비즈니스 로직** — install/repo/auth/env/deploy/logs는 모두 stub. 명령 트리만 잡고 본인이 써보면서 한 개씩 채워나감.
- **자동완성**(`uq completion`) — cobra가 무료로 제공. Phase 0에선 활성화만, 안내는 나중에.
- **테스트** — 스캐폴딩 단계라 단위 테스트는 최소(version의 ldflags 동작 정도)만. 실제 로직 들어갈 때 TDD로.
