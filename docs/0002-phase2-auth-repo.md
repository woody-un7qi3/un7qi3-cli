# 0002 — Phase 2: auth + repo 명령 구현

**Status:** 완료

## Context

Phase 0(`docs/0001-phase0-scaffolding.md`)에서 명령 트리만 잡혔다. 모든 명령은 stub 상태. 이번 Phase는 **stub을 실제 동작으로 채우는 첫 단계**로, 다른 모든 명령의 전제조건이 되는 `auth`와 `repo` 그룹부터 구현한다.

원래 Phase 0 문서에서 Phase 2는 "릴리즈 인프라(GoReleaser/GitHub Actions)"로 잡혀 있었으나, **릴리즈할 만한 실제 기능이 부족한 상태에서 릴리즈 인프라부터 만드는 것은 의미가 없다**고 판단해 순서를 바꾼다. 릴리즈 인프라는 명령들이 안정화된 뒤 별도 후속 Phase로 미룬다.

## 단계 재정렬

| Phase | 범위 | 비고 |
|---|---|---|
| 0 (완료) | 로컬 스캐폴딩, 명령 트리, 한글 i18n | docs/0001 |
| **2 (이번)** | `uq auth` + `uq repo` 실제 구현 | gh + aws + gcloud, 클론 |
| 후속 | env (.uq.yml 매니페스트 + AWS SSM), deploy/logs, install TUI | 점진적 |
| 더 후속 | 릴리즈 인프라 (GoReleaser/GitHub Actions, `uq upgrade`) | 명령 안정화 후 |

## 설계 원칙 (이번 Phase 추가)

Phase 0의 6개 원칙(명사-동사 일관성, 비대화형 우선, `--json`, `--dry-run`, 자기 설명적 `--help`, exit code 규칙)에 더해:

7. **SSH 키 불필요** — un7qi3 내부 모든 git 작업은 gh 인증 토큰으로 처리한다. `gh repo clone`(HTTPS+토큰), `gh auth setup-git`(credential helper 등록)으로 신입은 SSH 키 셋업 없이 `uq auth login` 한 번이면 끝.
8. **외부 CLI shell-out 우선** — Phase 2에서는 AWS SDK 직접 호출 없이 `gh`/`aws`/`gcloud` CLI를 외부 호출한다. 도구 자체의 인증/설정 메커니즘을 재발명하지 않는다. AWS SDK는 env Phase에서 SSM 직접 호출이 필요해질 때 도입.

## 구현 명령

### `uq auth login`

gh + AWS SSO + gcloud 일괄 로그인.

- 기본: 세 provider 모두 로그인 시도. 이미 인증된 provider는 "이미 로그인됨" 출력 + 스킵.
- gh 로그인 성공 시 `gh auth setup-git`을 추가로 실행 → git credential helper 등록.
- 플래그(택일):
  - `--gh-only`, `--aws-only`, `--gcloud-only`: 해당 provider만
  - `--skip-gh`, `--skip-aws`, `--skip-gcloud`: 해당 provider 제외
- 내부적으로 `gh auth login`, `aws sso login`, `gcloud auth login` 외부 호출 (interactive).
- 한 provider라도 실패 시 다른 provider 시도는 계속, 마지막에 실패 요약 + exit 1.

### `uq auth logout`

같은 플래그 체계로 일괄 로그아웃.

- `gh auth logout`
- `aws sso logout`
- `gcloud auth revoke --all`

### `uq auth status`

세 provider 인증 상태 확인. `--json` 지원.

사람 출력 (doctor 스타일):
```
$ uq auth status
✓ gh        woody-un7qi3 으로 인증됨
✓ aws       123456789012 (woody@un7qi3.co)
✓ gcloud    woody@un7qi3.co (active)

모든 provider 인증됨. (3/3)
```

`--json`:
```json
{
  "providers": [
    {"name": "gh", "ok": true, "user": "woody-un7qi3"},
    {"name": "aws", "ok": true, "account": "123456789012", "arn": "arn:aws:sts::123456789012:assumed-role/..."},
    {"name": "gcloud", "ok": true, "account": "woody@un7qi3.co"}
  ],
  "summary": {"ok": 3, "failed": 0}
}
```

체크 방법:
- gh: `gh auth status` (현재 doctor에 이미 있음 — 재사용)
- aws: `aws sts get-caller-identity` → Account/Arn
- gcloud: `gcloud auth list --format=json` → ACTIVE 계정

하나라도 실패 시 exit 4 (인증 필요).

### `uq repo list`

`un7qi3inc` 조직의 레포 목록.

- 내부 호출: `gh repo list un7qi3inc --json name,description,visibility,updatedAt,isArchived --limit <N>`
- 사람 출력: 컬럼 정렬 (NAME / VISIBILITY / UPDATED / DESCRIPTION)
- `--json`: gh 결과 그대로 stdout
- 플래그:
  - `--limit <N>` (기본 100)
  - `--archived`: archived만
  - `--no-archived`: archived 제외 (기본)
- gh 인증 안 됐을 경우 exit 4 (`uq auth status` 안내).

### `uq repo clone <name>`

`~/un7qi3/<name>`에 클론.

- 내부 호출: `gh repo clone un7qi3inc/<name> ~/un7qi3/<name>`
- 이미 디렉토리 존재 시 stderr "이미 존재함: <경로>" + exit 1.
- 플래그:
  - `--dir <path>`: 클론 위치 오버라이드 (기본 `$HOME/un7qi3/<name>`)
- gh 인증 안 됐을 경우 exit 4.

## 디렉토리 구조 (Phase 2에 추가)

```
internal/
├── exec/
│   └── exec.go            # 외부 CLI 호출 헬퍼 (gh/aws/gcloud)
│                          #  - Run(name, args...) ([]byte, error)
│                          #  - RunInteractive(name, args...) error
│                          #  - --verbose 시 명령 echo
└── auth/
    ├── auth.go            # 공용 타입 (Provider, Status)
    ├── gh.go              # GhStatus/GhLogin/GhLogout/GhSetupGit
    ├── aws.go             # AwsStatus/AwsLogin/AwsLogout
    └── gcloud.go          # GcloudStatus/GcloudLogin/GcloudLogout
```

기존 파일 변경:
- `internal/cmd/auth/{login,logout,status}.go`: stub 제거, 위 internal/auth 호출
- `internal/cmd/repo/{list,clone}.go`: stub 제거, internal/exec로 gh 호출
- `internal/cmd/doctor/doctor.go`: gh status check을 internal/auth.GhStatus로 위임 (DRY)

## 에러 코드 사용

- 0: 성공
- 1: 일반 에러 (이미 디렉토리 존재, gh CLI 자체 실패 등)
- 2: 사용법 에러 (cobra가 자동 처리)
- 4: 인증 필요 (status 명령에서 하나라도 인증 안 됐을 때, repo 명령에서 gh 인증 안 됐을 때)

## 검증 (Verification)

```bash
cd /Users/woody/un7qi3/un7qi3-cli
make install

# auth status
uq auth status                     # 3-provider, 모두 ✓ (woody 머신 가정)
uq auth status --json              # providers 배열 + summary
uq auth status --json | jq '.summary.ok'    # 3

# auth login (이미 로그인된 상태)
uq auth login --gh-only            # "이미 로그인됨: woody-un7qi3" + exit 0
uq auth login --aws-only           # 이미 SSO 세션 유효하면 스킵
uq auth login --gcloud-only        # 이미 로그인 시 스킵

# repo list
uq repo list                       # un7qi3inc 레포 컬럼 출력
uq repo list --json | jq 'length'  # 숫자
uq repo list --json | jq '.[0] | keys'   # ["description","isArchived","name",...]
uq repo list --limit 5             # 5개만

# repo clone
uq repo clone un7qi3-cli           # 이미 ~/un7qi3/un7qi3-cli 존재 → exit 1
uq repo clone un7qi3-cli --dir /tmp/un7qi3-cli-test   # /tmp에 클론 성공, exit 0
rm -rf /tmp/un7qi3-cli-test

# 사용법 에러
uq repo clone                      # exit 2 (인자 누락)
uq auth login --gh-only --aws-only # exit 2 (충돌 플래그) — cobra MarkFlagsMutuallyExclusive
```

## 명시적으로 제외 (Phase 2)

- **AWS SDK 직접 호출** — env Phase에서 SSM 직접 호출이 필요해질 때 도입.
- **`.uq.yml` 매니페스트** — env/deploy/logs Phase의 책임.
- **인증 자동 갱신** — 만료된 토큰을 자동으로 갱신하지 않는다. status에서 안내만.
- **`uq repo create/delete/fork`** — Phase 2 범위 밖. 필요해지면 후속 Phase에서.
- **릴리즈 인프라** — 별도 후속 Phase.
