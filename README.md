# un7qi3-cli

`uq` — un7qi3 사내 내부 CLI. Claude Code가 호출하기 좋게 설계된 결정론적 명령 트리.

## 설치 (Phase 0: 로컬)

```bash
make install          # go build → /usr/local/bin/uq
uq version            # 동작 확인
```

또는:

```bash
go install ./cmd/uq   # $GOPATH/bin/uq
```

## 기본 사용

```bash
uq --help             # 전체 명령 트리
uq doctor             # 필수 툴 점검 (git, gh, node, sdkman, java, aws, gcloud, docker)
uq version --json     # 빌드 메타데이터 (JSON)
```

## 문서

- 설계/계획 문서: `docs/`
- Phase 0 스캐폴딩: `docs/0001-phase0-scaffolding.md`
