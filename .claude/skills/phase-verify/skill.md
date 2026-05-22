---
name: phase-verify
description: Phase 문서의 "검증 (Verification)" 섹션 명령들을 한 줄씩 실행하고 통과/실패를 분류한 구조화 결과를 만든다. stub 명령의 stub 메시지는 통과로, 의도된 exit 2(사용법 에러)도 통과로 처리한다. Phase 문서 검증, 빌드 결과 확인, uq 동작 점검 시 사용한다.
---

# Phase Verify — Verification 섹션 자동 실행

## 트리거 시점

- Phase 문서의 "검증" 섹션을 일괄 실행해야 할 때
- `uq` 빌드 직후 동작 확인이 필요할 때
- "검증해줘", "verify 돌려줘" 같은 지시

## 절대 규칙

1. **문서가 SSOT** — 검증 명령은 Phase 문서의 "검증 (Verification)" 섹션에서만 추출. 임의 추가 금지.
2. **부분 통과는 통과가 아니다** — 단 하나라도 실패하면 전체는 실패. 통과/실패 명령을 명시적으로 분리.
3. **의도된 실패는 통과** — Phase 문서가 `# exit 2 (사용법 에러)` 같은 주석으로 "이건 실패해야 한다"를 표시하면 그 의도와 일치할 때 통과.
4. **stub 메시지는 통과** — Phase 문서가 명령 트리 표에서 `stub` 으로 표시한 명령은, "TODO" 또는 "not yet implemented" 메시지가 보이면 통과.

## 워크플로우

### 1. 검증 명령 추출

Phase 문서를 Read하고 `## 검증 (Verification)` 섹션 안의 ` ```bash ... ``` ` 코드 블록(들)에서 명령을 한 줄씩 추출.

각 줄에 대해:
- `# ...` 라인은 주석. 다음 라인의 의도(예상 동작, 예상 exit code)에 대한 힌트.
- 빈 줄, 그룹 헤더 주석(`# 기본 명령`)은 스킵.
- 한 줄에 명령 1개. 파이프(`|`)나 `&&`는 셸이 처리하므로 그대로 실행.

### 2. 명령 분류

Phase 문서의 "초기 명령 트리" 표를 함께 읽어, 각 명령이 다음 중 어디 속하는지 분류:
- **구현됨**: 정상 출력 + exit 0 기대
- **stub**: stub 메시지 + exit 0 기대
- **Phase N stub**: 명시적 "not yet released" 같은 메시지 + exit 0 기대
- **의도된 실패**: 주석으로 명시된 exit code (예: `# exit 2`)

### 3. 순차 실행

각 명령:

```
cd <작업 디렉토리> && <command>
```

캡처:
- exit code
- stdout (앞 10줄 + 뒤 10줄, 그 사이 길면 `...truncated...`)
- stderr (동일)
- 실행 시간

타임아웃: 30초. 초과 시 `failed` + note: "timeout".

### 4. 판정

각 명령에 대해:

| 분류 | 통과 조건 |
|---|---|
| 구현됨 | exit code 0 AND 출력에 의미 있는 결과 (빈 출력 아님) |
| stub | exit code 0 AND ("TODO" 또는 "not yet" 키워드 포함) |
| Phase N stub | exit code 0 AND ("Phase N" 또는 "not yet released" 키워드) |
| 의도된 실패 | exit code가 주석의 의도와 일치 |

### 5. 보고

구조화 출력:

```
passed: <N개>
  - command: "<명령>"
    classification: <구현됨|stub|...>
    exit_code: <code>
    note: <한 줄 요약>

failed: <M개>
  - command: "<명령>"
    classification: <구현됨|stub|...>
    expected_exit: <code>
    actual_exit: <code>
    stdout: |
      <앞 10줄>
    stderr: |
      <앞 10줄>
    note: <원인 추정>

summary:
  total: <N+M>
  passed: <N>
  failed: <M>
  duration_ms: <합산>
```

## 자주 빠지는 함정

- **`uq` PATH 못 찾음**: `make install`이 안 됐거나 `/usr/local/bin` 이 PATH에 없음. 첫 명령 실패면 거기서 멈추고 보고.
- **`go install` 경로 충돌**: `$GOPATH/bin/uq` 와 `/usr/local/bin/uq` 가 다른 버전일 수 있음. `which uq` 결과 명시.
- **doctor의 시스템 의존**: java가 정말 안 깔린 환경이면 doctor가 일부 실패 보고하는 게 정상. 그건 doctor의 "실패"가 아니라 "정확한 보고".
- **JSON 파싱 검증 누락**: `--json` 결과는 실제 JSON으로 파싱 가능한지 확인 (`echo "..." | jq .` 같은 추가 체크).

## 분류 우선순위

명령이 Phase 문서 표에 없으면(예: `which uq`, `make install`) "도구/메타" 분류로 처리. 이 그룹은:
- exit code 0이면 통과
- 단순 정보 출력 명령

## 후속 처리

이 스킬은 결과만 만들고 끝. Status 변경, 사용자 보고는 호출자(phase-orchestrator)가 처리.
