# Changelog

## [0.1.6](https://github.com/woody-un7qi3/un7qi3-cli/compare/v0.1.5...v0.1.6) (2026-06-22)


### 기능

* **run:** API switch 옵션을 개발/로컬(9200)/포트입력 3개로 ([dafc722](https://github.com/woody-un7qi3/un7qi3-cli/commit/dafc722e04080b3f549c9ce1e4a0238ad926f44d))
* **run:** forceteller-app 에 app 프로파일 추가 ([417f614](https://github.com/woody-un7qi3/un7qi3-cli/commit/417f6142d67be3c84c68977e58918e113e68fc04))
* **run:** list 대화형 실행 서브명령 추가 및 profiles→targets 리네임 ([efcd599](https://github.com/woody-un7qi3/un7qi3-cli/commit/efcd599faac03d7eb145c0a528cccc860f857cde))
* **run:** list 선택을 2단계(레포→프로파일)로 변경 ([d185d56](https://github.com/woody-un7qi3/un7qi3-cli/commit/d185d561ef5b78d629794cbfb207d8c2f649c182))
* **run:** list 흐름에 실행 전 설정 토글(switches) 추가 ([daf0250](https://github.com/woody-un7qi3/un7qi3-cli/commit/daf02504a1b4adf78e378ce838276cfb0306c11b))
* **run:** node 버전 범위(&gt;=N) 지원 — 팀원별 node 환경 흡수 ([b54fd79](https://github.com/woody-un7qi3/un7qi3-cli/commit/b54fd7946924d0b3f84b95cb16022daa8972c3e7))
* **run:** switch 에 로케일(scope) 축 추가 — kr/jp 별 apiUrl 토글 ([b63d520](https://github.com/woody-un7qi3/un7qi3-cli/commit/b63d52046e78c2623e3cadf1586a9f2a600b0fc5))
* **run:** switch 옵션에 입력 플레이스홀더({url}/{port}) 추가 ([cb59ae7](https://github.com/woody-un7qi3/un7qi3-cli/commit/cb59ae788d94a09ac204849ce6dbf4fd754ebab6))
* **run:** 단일 프로세스도 통합 TUI 로 표시 및 종료 코드 버그 수정 ([259d6fb](https://github.com/woody-un7qi3/un7qi3-cli/commit/259d6fbcb6124d468c7a07f566f9157622c7617d))
* **run:** 프로파일 용도 설명(desc) 추가 — list picker에 표시 ([aaf620c](https://github.com/woody-un7qi3/un7qi3-cli/commit/aaf620ca920f350898d96c14a7d2966f2b9eed60))
* **version:** 빌드 시각을 KST 로 표시 ([26ed207](https://github.com/woody-un7qi3/un7qi3-cli/commit/26ed207d472e740fc4a71bca5e2f0c79e40c0849))


### 버그 수정

* **run:** forceteller-app:app 을 node 20 으로 (Vite 5 요구) ([c6c22b1](https://github.com/woody-un7qi3/un7qi3-cli/commit/c6c22b1b92aeb8b798f3c7d73c8898e33fa1c55e))
* **run:** 개발 옵션은 입력 대신 기존 주소를 라벨에 노출 ([f9e754e](https://github.com/woody-un7qi3/un7qi3-cli/commit/f9e754e645a2d8b1c00fd5093616bab73db19ef4))

## [0.1.5](https://github.com/woody-un7qi3/un7qi3-cli/compare/v0.1.4...v0.1.5) (2026-06-22)


### 기능

* **log:** external-api log 대상 추가 ([bfb41ae](https://github.com/woody-un7qi3/un7qi3-cli/commit/bfb41ae1de9172ba282db54225af0a6b5bcba243))
* **log:** list 대화형 선택 서브명령 추가 및 internal/logs→internal/log 리네임 ([2128501](https://github.com/woody-un7qi3/un7qi3-cli/commit/2128501a7e13baf262c058df8d929e0803880f73))
* **log:** 대상 나열 targets 서브명령 추가 및 logs→log 리네임 ([ec5b1c9](https://github.com/woody-un7qi3/un7qi3-cli/commit/ec5b1c994935a97f707c2976f269123865d0e2ee))
* **run:** 멀티프로세스 로그를 uq log 공유 TUI 로 통합 ([ba99c3d](https://github.com/woody-un7qi3/un7qi3-cli/commit/ba99c3de987d6cd2cfa2f0f34c768ce2440f3684))

## [0.1.4](https://github.com/woody-un7qi3/un7qi3-cli/compare/v0.1.3...v0.1.4) (2026-06-22)


### 버그 수정

* **upgrade:** 릴리즈 노트에서 마크다운 굵게 표시(**) 제거 ([8675e2a](https://github.com/woody-un7qi3/un7qi3-cli/commit/8675e2ac1eddcf3e6b709c391e926edd8acc99d0))

## [0.1.3](https://github.com/woody-un7qi3/un7qi3-cli/compare/v0.1.2...v0.1.3) (2026-06-22)


### 기능

* **upgrade:** 릴리즈 노트를 터미널 평문으로 렌더링 ([17b06a4](https://github.com/woody-un7qi3/un7qi3-cli/commit/17b06a423e810cc7e692b379523dc05a7a7f73a3))
* **upgrade:** 업그레이드 후 릴리즈 노트 출력 및 진행 표시 정리 ([ae3fb2a](https://github.com/woody-un7qi3/un7qi3-cli/commit/ae3fb2ade5bb79e3077d2cc12845e875ab07e8b4))

## [0.1.2](https://github.com/woody-un7qi3/un7qi3-cli/compare/v0.1.1...v0.1.2) (2026-06-22)


### 리팩터

* 시니어 관점 전면 리팩토링 (에러계약·context·DI·전역상태·견고성) ([e04f12c](https://github.com/woody-un7qi3/un7qi3-cli/commit/e04f12c80f0e5c7d474a5cf0a903595c1a2b4ae2))

## [0.1.1](https://github.com/woody-un7qi3/un7qi3-cli/compare/v0.1.0...v0.1.1) (2026-06-22)


### 기능

* **cmd:** update/upgrade 별칭을 함께 표시하고 설명 갱신 ([b3ef62b](https://github.com/woody-un7qi3/un7qi3-cli/commit/b3ef62bc7c871f20b37044597c12b32c0fe4da1b))
