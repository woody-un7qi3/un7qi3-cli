# 0003 — Phase 3: uq run 국가 선택 + 로그 레이아웃

**Status:** 완료

## Context

`uq run <repo>[:profile]`(Phase 2 후속으로 이미 구현됨)은 `internal/repocfg/repos.yml`의 `runs:` 블록에 선언된 프로파일을 실행한다. 멀티프로세스 프로파일(예: `forceteller-admin`)은 `[back]`/`[front]` prefix를 붙여 **한 화면에 로그를 합쳐** 포그라운드로 띄운다.

이번 Phase는 실제 개발 워크플로에서 빠진 두 가지를 채운다.

1. **국가 선택** — `forceteller-admin`은 KR/EN/JP 로컬 환경을 따로 띄울 수 있다. 현재는 KR(`npm run local`)만 가능하다.
2. **로그 레이아웃 선택** — 지금은 항상 한 화면에 합친다. proc별로 분리해서 보고 싶은 사용자를 위해 선택지를 준다.

### 조사 결과 — forceteller-admin의 국가 메커니즘

국가는 **npm 스크립트 suffix**로 구분된다. back/front가 같은 네이밍 규칙을 공유한다.

| 국가 | back-end 스크립트 | front-end 스크립트 | 포트 |
|---|---|---|---|
| KR | `local` | `local` | back 3000 / front 4200 |
| EN | `local_en` | `local_en` | back 3000 / front 4200 |
| JP | `local_jp` | `local_jp` | back 3000 / front 4200 |

- back-end `package.json`: `"local_en": "cross-env NODE_ENV='local_en' nodemon src/app.ts"` — `cross-env`가 `NODE_ENV`만 세팅.
- `.env` 선택은 앱이 직접 한다 (`back-end/src/app.ts:27-45`): `NODE_ENV`가 `local`→`.env`, `local_en`→`.env.en`, `local_jp`→`.env.jp`를 `fs.readFileSync`로 읽는다. **파일이 없으면 readFileSync에서 크래시한다.**
- front-end는 Angular config로 구분: `local_en`→`-c=en-prod`, `local_jp`→`-c=jp-prod`.
- **포트가 국가 무관 동일**하므로 한 번에 한 국가만 띄울 수 있다. URL도 동일하다.

함의:
- uq는 `.env` 파일을 직접 건드릴 필요가 없다. **올바른 스크립트(=올바른 NODE_ENV)만 실행**하면 된다.
- 단, 앱이 readFileSync로 크래시하므로 uq가 **실행 전에 해당 국가의 `.env` 파일 존재를 검증**해야 한다. (현재 `.env.jp` 없음 → JP 실행 시 크래시)

## 설계 원칙 (이번 Phase 추가)

Phase 0/2의 원칙에 더해:

9. **선언적 변형(variant)** — 국가처럼 "프로파일과 직교하는 축"은 cmd/proc 구조를 복제하지 않고 한 곳에서 토큰 치환으로 표현한다.
10. **실행 전 사전 검증(pre-flight)** — 외부 도구가 런타임에 크래시할 조건(없는 파일 등)을 uq가 미리 감지해 친절히 막는다. gh 인증 사전 확인(`repo` 명령)과 같은 철학.
11. **환경 적응(graceful degradation)** — 터미널 분할처럼 환경에 의존하는 기능은 가능한 환경을 감지해 최선을 쓰고, 불가능하면 조용히 차선으로 강등하되 무엇을 했는지 알린다.

## 기능 1 — 국가 선택

### repos.yml 스키마 확장

`Profile`에 선택적 `countries` 블록 추가. 없으면 기존 동작과 100% 동일 (forceteller-app 등 영향 없음).

```yaml
forceteller-admin:
  default: local
  profiles:
    local:
      node: "16"
      countries:
        default: kr
        options:
          - code: kr
            script: local
            requires: [back-end/.env]
          - code: en
            script: local_en
            requires: [back-end/.env.en]
          - code: jp
            script: local_jp
            requires: [back-end/.env.jp]
      procs:
        - name: back
          cwd: back-end
          cmd: ["npm", "run", "{script}"]   # {script} ← 선택 국가로 치환
          url: "http://localhost:3000"
        - name: front
          cwd: front-end/workspace
          cmd: ["npm", "run", "{script}"]
          url: "http://localhost:4200"
```

- `countries.default`: 비TTY/플래그 없을 때 쓰는 국가 코드.
- `countries.options[]`: 선언 순서대로 TUI에 노출.
  - `code`: 국가 식별자 (kr/en/jp).
  - `script`: 그 국가의 npm 스크립트 이름.
  - `requires`: 실행 전 존재해야 하는 파일들. **레포 루트 기준 상대경로.**
- `{script}` 토큰: 선택 국가의 `script` 값으로 치환. `Profile.Cmd`와 모든 `Proc.Cmd`의 argv 토큰에 적용. back/front가 동일 스크립트 이름을 쓰는 전제(forceteller-admin 충족). proc별로 스크립트 이름이 다른 경우는 이번 범위 밖.

### 국가 결정 흐름 (`internal/cmd/run/run.go`)

1. `--country <code>` 플래그가 있으면 그 값 사용. 프로파일의 `countries.options`에 없는 코드면 에러(exit 1).
2. 플래그 없고 프로파일에 `countries`가 있으면:
   - **TTY** → 국가 선택 TUI (아래).
   - **비TTY**(CI/agent) → `countries.default`.
3. 프로파일에 `countries`가 없으면 이 단계 전부 스킵.

### .env 사전 검증

선택된 국가의 `requires` 파일들이 레포 루트(`<reposDir>/<repo>`) 기준으로 모두 존재하는지 확인.

- 전부 존재 → 진행.
- 하나라도 없음 → 에러(exit 1):
  `forceteller-admin:local (jp) 실행 불가 — 없는 파일: back-end/.env.jp`

### 국가 선택 TUI

`countries.options`를 순회하며 각 국가의 `requires` 충족 여부를 미리 검사한다.

- **충족** 국가만 `huh.NewSelect` 선택지에 올린다 (default를 pre-select).
- **미충족** 국가는 선택 전 dim 안내로 나열하고 선택 불가:
  ```
  ⊘ jp — back-end/.env.jp 없음 (선택 불가)
  ```
  (huh가 옵션 단위 비활성을 지원하면 비활성 항목으로, 아니면 위처럼 선택지에서 제외 + 사전 안내)
- 충족 국가가 0개면 에러(exit 1)로 누락 파일 안내.

### --dry-run / 헤더 반영

- `--dry-run`: 선택 국가, 검증 결과, `{script}` 치환된 최종 cmd 표시.
- 실행 헤더: `profile=local(en)` 형태로 국가 표기.

## 기능 2 — 로그 레이아웃 선택

멀티프로세스 포그라운드 실행에만 의미가 있다 (단일 proc은 분할 대상 없음, `--bg`는 이미 파일 분리).

### 레이아웃 종류

- **merged** (기본, 현재 동작): `[name]` prefix로 한 화면에 합침.
- **split**: proc마다 별도 패널/창. 터미널 환경을 감지해 네이티브 분할을 쓰고, 불가하면 자동 강등.

### 선택 UX

기존 fg/bg 모드 선택(`chooseMode`)을 확장한다. **멀티프로세스 프로파일**일 때 TTY 선택지:

```
어떻게 실행할까요?
  포그라운드 — 한 화면에 로그 합침 ([name] prefix), Ctrl+C 로 종료
  포그라운드 — 패널 분할 (감지: cmux), proc별 별도 패널
  백그라운드 — 로그 파일로 분리, 즉시 복귀
```

- 단일 proc 프로파일은 기존대로 fg/bg 2지선다.
- 플래그: `--split`(패널 분할), 기존 `--fg`/`--bg` 유지. `--split`은 `--bg`와 상호 배타.
- 비TTY는 기존대로 포그라운드 merged.

### 터미널/멀티플렉서 감지 (`internal/run/terminal.go`)

**우선순위 사다리** (cmux를 `TERM_PROGRAM`보다 먼저 — cmux는 ghostty를 임베드하므로 `TERM_PROGRAM=ghostty`로 오인됨):

| 순위 | 감지 단서 | 분할 수단 |
|---|---|---|
| 1. tmux | `$TMUX` set | `tmux split-window` |
| 2. cmux | `CMUX_PANEL_ID` && `CMUX_SOCKET_PATH` set | cmux CLI (`CMUX_BUNDLED_CLI_PATH`) `new-pane` + `respawn-pane` |
| 3. iTerm2 | `TERM_PROGRAM == "iTerm.app"` | osascript (AppleScript) |
| 4. none | 그 외 (ghostty 단독/Apple_Terminal/vscode/wezterm/미상) | (분할 불가) |

`DetectMultiplexer() Multiplexer` 반환: `tmux | cmux | iterm2 | none`.

### split 실행 (`internal/cmd/run/split.go`)

감지 결과에 따라:

- **tmux/cmux/iTerm2** → proc마다 패널을 띄우고 각 패널에서 그 proc의 cmd 실행. 각 패널은 새 셸이므로 cwd, 병합된 env(노드 PATH prepend 포함), 치환된 cmd를 함께 넘긴다. (env는 `export ...; exec <cmd>` 형태 셸 명령으로 전달)
- **none** → **자동 fallback (옵션 A)**: 기존 `runBackground`로 proc별 로그파일 분리 + `tail -f` 안내. 무엇을 했는지 알린다:
  ```
  이 터미널(ghostty 단독)은 패널 분할 미지원 → 백그라운드 로그파일로 분리합니다.
  tmux/cmux/iTerm2 안에서 실행하면 패널이 분할됩니다.
  ```

## 디렉토리 구조 (Phase 3에 추가/변경)

```
internal/
├── run/
│   └── terminal.go          # DetectMultiplexer() — tmux/cmux/iterm2/none 사다리
├── repocfg/
│   ├── repocfg.go           # (변경) Profile.Countries, Country 타입, {script} 치환 헬퍼
│   └── repos.yml            # (변경) forceteller-admin.local 에 countries 블록
└── cmd/run/
    ├── run.go               # (변경) 국가 결정/검증/치환, chooseMode 확장(split)
    ├── split.go             # (신규) 멀티플렉서별 패널 분할 실행 + none fallback
    ├── country.go           # (신규) 국가 TUI, requires 검증, {script} 치환 적용
    └── background.go        # (재사용) split none-fallback이 호출
```

기존 파일 변경:
- `internal/repocfg/repocfg.go`: `Profile`에 `Countries *Countries` 추가, `Countries`/`Country` 타입, `ResolveScript(profile, code)` 등 치환 헬퍼.
- `internal/cmd/run/run.go`: 국가 플래그/결정/검증 호출, `chooseMode`에 split 추가, 치환된 cmd로 exec.
- `internal/cmd/run/profiles.go`: `profileJSON`에 국가 목록 노출(선택). `uq run profiles --json`이 국가별 cmd/requires를 보여주면 agent에 유용.

## 에러 코드 사용

- 0: 성공
- 1: 일반 에러 (없는 `requires` 파일, 알 수 없는 `--country` 코드, 선택 가능 국가 0개)
- 2: 사용법 에러 (cobra; `--split`+`--bg` 충돌 등)

## 검증 (Verification)

```bash
cd /Users/woody/un7qi3/un7qi3-cli
make install

# --- 국가 선택 ---
# 비TTY → default(kr) 사용, {script}=local 치환 확인
uq run forceteller-admin --dry-run | grep -E "local\b"            # back/front 모두 npm run local
uq run forceteller-admin --country en --dry-run | grep "local_en" # local_en 으로 치환
uq run forceteller-admin --country jp --dry-run                   # exit 1, "back-end/.env.jp 없음"
uq run forceteller-admin --country xx --dry-run                   # exit 1, 알 수 없는 국가

# 국가 없는 프로파일은 영향 없음
uq run forceteller-app --dry-run                                  # 기존대로, 국가 단계 없음

# profiles 출력에 국가 노출(구현 시)
uq run profiles --json | jq '.profiles[] | select(.repo=="forceteller-admin")'

# --- 로그 레이아웃 ---
# 플래그 충돌
uq run forceteller-admin --split --bg                             # exit 2 (상호 배타)

# 터미널 감지 (현재 세션은 cmux)
# split 선택 시 감지 로그가 "cmux" 로 떠야 함 (TERM_PROGRAM=ghostty 오인 아님)
uq run forceteller-admin --country kr --split --dry-run           # 감지 결과 + 패널 분할 계획 표시

# 분할 불가 환경 시뮬레이션 (CMUX_*/TMUX 제거)
env -u TMUX -u CMUX_PANEL_ID -u CMUX_SOCKET_PATH TERM_PROGRAM=Apple_Terminal \
  uq run forceteller-admin --country kr --split --dry-run         # none → 백그라운드 fallback 안내

# 단위 테스트
go test ./internal/run/... ./internal/repocfg/...                 # DetectMultiplexer, {script} 치환, requires 검증
```

> 주의: 실제 `uq run`(--dry-run 없이)은 dev 서버를 띄우므로 자동 검증에서는 `--dry-run`으로만 확인한다.

## 명시적으로 제외 (Phase 3)

- **proc별 서로 다른 스크립트 이름** — 모든 proc이 같은 국가 스크립트 이름을 쓰는 전제. 달라지면 후속 Phase에서 proc-단위 `{script}` 매핑 도입.
- **국가별 포트 분리 / 동시 다국가 실행** — 실제 앱이 local에서 포트를 고정(3000/4200)하므로 불가. 동시 실행은 범위 밖.
- **`.env` 내용 생성/동기화** — uq는 존재만 검증한다. `.env` 발급은 env Phase(SSM) 책임.
- **tmux/cmux/iTerm2 외 터미널의 네이티브 분할** — wezterm/kitty 등은 none으로 처리(백그라운드 fallback). 필요해지면 후속 추가.
- **분할 패널의 생명주기 관리** — 분할 후 패널 일괄 종료/정리는 범위 밖. 사용자가 패널을 직접 닫는다.
```
