# 다음 Seed 후보

이 문서는 `main` 기준으로 PR #1 merge 이후 이어갈 작업 후보를 정리합니다.
작업을 시작할 때는 `AGENTS.md`의 Seed 운영 규칙에 따라 사용자에게 먼저 브리핑하고 진행합니다.

## 현재 기준 상태

- Envoy AI Gateway v0.5 기반 Memory ExtProc PoC가 `main`에 merge됨
- Redis 기반 short-term memory 저장/조회 구현 완료
- v0.5 GatewayConfig, EnvoyExtensionPolicy, Redis, memory-extproc 매니페스트 작성 완료
- 장애 시나리오 단위 테스트 작성 완료
- 운영 전 체크리스트 작성 완료

## Seed 9: PoC 재현 절차 문서화

### 목표

다른 사람이 `main`만 받아도 Kind 클러스터에서 v0.5 + Redis + memory-extproc 흐름을 다시 검증할 수 있게 합니다.

### 변경 대상

- `docs/reproduce-v05-memory-poc.md`
- 필요 시 `README.md` 링크
- 세션 기록

### 완료 기준

- 로컬 도구 확인부터 Kind 클러스터, v0.5 설치, Redis, memory-extproc, Gateway 연결, curl 검증까지 순서가 문서화되어야 합니다.
- mock backend 기반 smoke와 Redis 히스토리 확인 방법이 포함되어야 합니다.
- 실제 provider/API key가 필요한 경로와 mock 경로가 구분되어야 합니다.

### 범위 밖

- 자동화 스크립트 작성
- CI 구성
- 운영용 Helm chart 작성

## Seed 10: OpenRouter 실제 Provider 연동

### 목표

mock backend가 아니라 OpenRouter 실제 Provider를 통해 v0.5 Gateway + memory-extproc + Redis 흐름을 검증합니다.

### 변경 대상

- `deploy/gateway/v0.5-openrouter-sample.yaml`
- `examples/requests/openrouter-first-turn.json`
- `examples/requests/openrouter-second-turn.json`
- `docs/openrouter-provider.md`
- 필요 시 `README.md` 링크
- 세션 기록

### 완료 기준

- OpenRouter upstream 설정과 API key Secret 주입 방식이 문서화되어야 합니다.
- 실제 API key를 저장소에 남기지 않아야 합니다.
- `stream: false` 기준 2-turn 메모리 검증 요청 예제가 있어야 합니다.
- API key가 있는 환경에서 적용/검증 가능한 매니페스트가 있어야 합니다.

### 범위 밖

- API key를 저장소에 저장
- Streaming/SSE 응답 저장
- 운영 수준 Secret rotation 정책

## Seed 11: 재현 절차 자동화 스크립트

### 목표

Seed 9와 Seed 10 문서를 바탕으로 반복 명령을 스크립트화합니다.

### 후보 파일

- `deploy/gateway/v0.5/install.sh`
- `deploy/gateway/v0.5/smoke-test.sh`
- `deploy/memory-extproc/build-and-load.sh`

### 완료 기준

- 수동 명령 없이 기본 smoke test를 재현할 수 있어야 합니다.
- 실패 시 어느 단계에서 막혔는지 출력해야 합니다.

## Seed 12: Streaming/SSE 메모리 처리 설계

### 목표

현재 `Buffered` response body 기반 처리로는 streaming 응답을 그대로 다루기 어렵기 때문에, streaming 응답에서 assistant 메시지를 어떻게 저장할지 설계합니다.

### 변경 대상

- `docs/streaming-memory-design.md`
- 필요 시 `docs/architecture.md`

### 완료 기준

- 현재 방식이 streaming과 충돌하는 이유를 설명합니다.
- 가능한 대안을 비교합니다.
- 1차 구현 후보와 제외할 범위를 정합니다.

### 범위 밖

- 실제 streaming parser 구현
- Provider별 SSE 차이 전체 구현

## Seed 13: 운영 정책 구체화

### 목표

운영 체크리스트의 미결정을 실제 정책 초안으로 바꿉니다.

### 후보 주제

- Redis auth/persistence/HA
- Secret 관리
- session ID 발급/검증
- 로그 마스킹
- 관측성/메트릭
- fail-open vs fail-closed 서비스별 정책

## 추천 순서

1. Seed 9: PoC 재현 절차 문서화
2. Seed 10: OpenRouter 실제 Provider 연동
3. Seed 11: 재현 절차 자동화 스크립트
4. Seed 12: Streaming/SSE 메모리 처리 설계
5. Seed 13: 운영 정책 구체화
