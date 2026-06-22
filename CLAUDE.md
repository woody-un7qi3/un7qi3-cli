# un7qi3-cli (`uq`)

cobra + charmbracelet 기반 Go CLI. 빌드/설치: `make install`. 테스트: `make test`. 커밋 메시지는 한글, AI 표기(Co-Authored-By 등) 금지.

## 하네스: 진단 & 시니어 리팩토링

**목표:** Go 코드베이스를 시니어 관점에서 진단하고, 테마별 점진 커밋으로 안전하게 전면 리팩토링한다.

**트리거:** "진단", "리팩토링", "코드 품질 개선", "시니어 수준으로" 등 코드 개선 요청 시 `refactor-orchestrate` 스킬을 사용하라. 단순 질문/단발 수정은 직접 응답 가능.

**구성:** diagnostician → refactor-planner → go-refactorer → qa-verifier (팀: `refactor-crew`). 스킬: refactor-orchestrate(오케스트레이터), go-diagnose, go-refactor, go-verify. 상세는 `.claude/agents/`, `.claude/skills/` 참조.

**변경 이력:**
| 날짜 | 변경 내용 | 대상 | 사유 |
|------|----------|------|------|
| 2026-06-22 | 초기 구성(진단+리팩토링 하네스) | agents/{diagnostician,refactor-planner,go-refactorer,qa-verifier}, skills/{refactor-orchestrate,go-diagnose,go-refactor,go-verify}, CLAUDE.md | 전면 리팩토링 요청 |

> 참고: scaffolding 전용 하네스(phase-orchestrator/go-scaffolder/phase-verifier + phase-* 스킬)는 별개 도메인이며 이 작업에서 사용하지 않는다.
