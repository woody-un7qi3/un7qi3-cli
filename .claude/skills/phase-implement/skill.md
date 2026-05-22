---
name: phase-implement
description: docs/NNNN-*.md Phase 문서를 받아 정의된 작업을 끝까지 수행한다. Status 라인을 시작전→진행중→완료로 단계적으로 갱신하고, 본문은 절대 수정하지 않으며, 팀(go-scaffolder + phase-verifier)을 조율한다. Phase 문서 구현, "0001 구현해줘", "Phase 0 진행" 같은 요청에서 반드시 사용한다.
---

# Phase Implement — Phase 문서 lifecycle 오케스트레이션

## 트리거 시점

- 사용자가 `docs/NNNN-*.md` 문서를 가리키며 "구현해줘", "진행해줘", "이대로 해줘" 라고 한 경우
- "Phase 0 시작", "Phase 0 구현", "이 계획 실행해줘" 같은 요청
- `@docs/0001-*.md 이걸 구현` 같은 첨부 + 지시 조합

## 절대 규칙

**문서 본문(Status 라인 제외)을 절대 수정하지 않는다.** 이 프로젝트의 docs/ 는 ADR/RFC 컨벤션이며, 문서가 일단 작성되면 본문은 history로 보존된다. 본문에 오타가 있어도, 누락이 있어도, 더 좋은 설계가 떠올라도 — 본문을 수정하지 말고 **새 번호의 후속 문서를 만들 것**을 사용자에게 제안한다.

수정 허용 범위: 최상단 `**Status:** ...` 줄 하나만. 이 줄의 값을 `시작전` ↔ `진행중` ↔ `완료` 사이에서 전환.

## 워크플로우

### 1. Phase 문서 파싱

Read 도구로 문서 전체 로드. 다음 섹션을 식별:

- **Status 라인**: 최상단 `**Status:** <값>` 위치 기억
- **디렉토리 구조**: ` ```` ``` 코드 블록 안의 트리. 각 파일/디렉토리 경로 추출
- **초기 명령 트리 표**: `구현` vs `stub` vs `Phase N stub` 분류
- **검증 (Verification)** 섹션: 실행할 명령 목록

추출 결과를 머릿속(혹은 _workspace/ 파일)에 보관. 본문 자체는 수정하지 않음.

### 2. Status를 진행중으로

Edit 도구로 `**Status:** 시작전` → `**Status:** 진행중` 단 한 줄 교체. old_string에 라인 전체를 포함하여 정확히 매칭.

이미 `진행중`이면 스킵. 이미 `완료`면 사용자에게 "이미 완료된 문서입니다. 다시 진행할까요?" 확인.

### 3. 팀 구성

`TeamCreate` 도구로 팀 생성:

- team_name: `phase-builders`
- members:
  - `phase-orchestrator` (자기 자신, 리더)
  - `go-scaffolder`
  - `phase-verifier`

모든 호출에 `model: "opus"` 명시.

### 4. 작업 분배

`TaskCreate`로 두 개의 작업 등록:

- `scaffold`: go-scaffolder 담당, 의존성 없음
- `verify`: phase-verifier 담당, scaffold 완료 후

`SendMessage`로 go-scaffolder에게 작업 시작 트리거:
```
to: go-scaffolder
message: |
  Phase 문서를 구현해주세요.
  문서: <절대 경로>
  작업 디렉토리: <절대 경로>
  
  주의:
  - 문서 본문 수정 금지
  - 문서의 디렉토리 트리/명령 목록 그대로
  - 완료 후 진행 상황 보고
```

### 5. 스캐폴딩 결과 검수

go-scaffolder가 보고하면:
- 생성된 파일 목록 확인
- `go build ./cmd/uq` 가 성공했는지 확인
- 막힘 보고가 있으면 1회 재시도 지시. 재실패 시 진행 중단.

### 6. 검증 위임

`SendMessage`로 phase-verifier에게 트리거:
```
to: phase-verifier
message: |
  검증을 시작해주세요.
  문서: <절대 경로>
  작업 디렉토리: <절대 경로>
  
  문서의 "검증 (Verification)" 섹션 명령을 모두 실행하고 통과/실패 분류 결과 보고.
```

### 7. 결과 종합

phase-verifier의 결과를 받음:
- `failed`가 비어있음 → Status를 `완료`로 Edit. 사용자에게 전체 보고.
- `failed`가 있음 → Status는 `진행중` 유지. 실패한 명령과 원인을 사용자에게 보고. 사용자 지시 대기.

### 8. 팀 정리

(선택) `TeamDelete`로 팀 해체. 또는 다음 Phase에서 재사용 가능하게 유지.

## 사용자 보고 형식

```
## Phase <번호> 구현 결과

**문서**: docs/NNNN-*.md
**최종 Status**: 진행중 | 완료

### 생성된 파일
- <경로 1>
- <경로 2>
- ...

### 검증 결과
- 통과: N개
- 실패: M개

#### 실패 상세 (있을 경우)
- `<명령>` (exit <code>): <원인 요약>

### 다음 단계
<완료 시>: "Phase 완료. 이후 작업(예: Phase 2 release infra)을 시작할 준비가 됐습니다."
<실패 시>: "<원인> 때문에 진행 멈춤. 어떻게 처리할지 알려주세요."
```

## 자주 빠지는 함정

- **본문 수정 충동**: "여기 typo가 있네, 고치면서..." 금지. 본문은 history.
- **Status를 두 단계 점프**: `시작전` → `완료` 직접 점프 금지. 반드시 `진행중`을 거침.
- **검증 실패 무시**: 일부 명령만 통과해도 "거의 완료" 라며 `완료` 마킹 금지.
- **stub과 실제 구현 혼동**: Phase 문서 명령 트리 표가 SSOT. `stub`은 stub 메시지만 보여도 OK.

## 다중 Phase 문서 발견 시

`docs/` 에 여러 문서가 있고 사용자가 어떤 것인지 명시하지 않은 경우:
- Status가 `시작전`인 가장 낮은 번호 문서를 후보로 제시
- 사용자에게 확인 요청 ("docs/0001-... 진행할까요?")
- 사용자 명시 후에만 작업 시작
