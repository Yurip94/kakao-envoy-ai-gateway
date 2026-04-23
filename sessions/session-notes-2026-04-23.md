# 세션 기록 - 2026-04-23

## 목적

이번 세션은 `kakao-envoy` 프로젝트의 초기 방향을 맞추고, 기존 `README.md`를 기준으로 실제 PoC 구현을 시작하기 위한 작업 규칙과 실행 순서를 정리하기 위해 진행했다.

## 확인한 프로젝트 성격

- 이 저장소는 Envoy AI Gateway v0.5 기반 LLM 대화 메모리 PoC를 새로 구현하기 위한 프로젝트다.
- 기존 `README.md`는 킥오프/기획 문서 성격이며, 다음 두 가지를 핵심 목표로 둔다.
  - Envoy AI Gateway v0.4에서 v0.5로 마이그레이션
  - v0.5 확장 기능을 활용한 LLM 대화 메모리 PoC 구현
- Envoy AI Gateway는 대화 메모리를 내장하지 않으므로, 메모리는 직접 구현해야 한다.
- 대화 메모리의 1차 범위는 Short-term Memory다.
- Long-term Memory, Semantic Memory, MCP 세션 메모리는 후속 또는 선택 범위다.

## 주요 대화 흐름

### 1. README 검토

사용자는 `README.md`를 읽고 어떤 프로젝트인지 검토해달라고 요청했다.

검토 결과:

- 프로젝트는 실제 코드 구현체라기보다 초기 기획 문서 상태였다.
- 목표는 Envoy AI Gateway v0.5 업그레이드와 대화 메모리 PoC 구현이다.
- 핵심 설계 선택지는 다음 두 가지였다.
  - Option A: Custom External Processor
  - Option B: Body Mutation + 외부 Memory Service
- Option A는 구현 난이도가 높지만, Gateway 내부 흐름에서 메모리 조회/주입/저장을 처리할 수 있어 프로젝트 목적에 더 잘 맞는다.
- Option B는 빠른 데모에는 유리하지만 클라이언트 책임이 커지고, Envoy AI Gateway 확장성 검증 범위가 줄어든다.

### 2. 프로젝트 방향 확정

사용자는 다음 내용을 확정했다.

1. 실제 PoC 코드를 이 저장소에 새로 만들 예정이다.
2. `README.md`를 바탕으로 구현할 예정이다.
3. 구현 방식은 Option A로 정면 돌파한다.
4. 기존 `AGENTS.md`는 다른 프로젝트에서 가져온 것이므로, 현재 프로젝트와 하네스에 맞게 수정해야 한다.

이에 따라 `AGENTS.md`를 현재 프로젝트 기준으로 다시 작성했다.

반영한 핵심:

- `README.md`를 프로젝트 기준 문서로 지정
- Option A: Custom ExtProc를 기본 구현 방향으로 지정
- Option B는 fallback 후보로만 유지
- Memory ExtProc, Redis, OpenAI 호환 `messages`, `x-session-id` 기준을 명시
- Git 저장소가 아닐 수 있음을 반영해 Git/PR 규칙을 완화
- 테스트 및 장애 시나리오 기준 추가

### 3. 초기 실행 순서 결정

다음 4단계 순서로 진행하기로 했다.

1. PoC 언어와 저장소 골격을 정한다.
2. ExtProc 구현 방식을 확정한다.
3. `docs/architecture.md`를 작성한다.
4. Kind + Helm 배포 매니페스트를 준비한다.

1번에 대한 결정:

- 기본 언어는 Go로 정했다.
- 이유:
  - Envoy/gRPC/protobuf 생태계와 잘 맞는다.
  - ExtProc 서비스 구현에 자연스럽다.
  - 컨테이너 배포와 Kubernetes 통합 검증에 적합하다.

권장 초기 구조:

```text
.
├── README.md
├── AGENTS.md
├── cmd/
│   └── memory-extproc/
├── internal/
│   ├── memory/
│   ├── openai/
│   └── extproc/
├── deploy/
│   ├── kind/
│   ├── gateway/
│   └── redis/
├── examples/
│   └── requests/
└── docs/
    ├── architecture.md
    └── migration-v0.4-to-v0.5.md
```

### 4. Ouroboros 방식 참고

사용자는 다음 저장소를 참고해 앞으로의 대화 방식에 적용하면 좋겠다고 말했다.

- <https://github.com/Q00/ouroboros>
- <https://github.com/Q00/ouroboros/blob/main/README.ko.md>

설치는 요구하지 않았다.

적용하기로 한 방식:

- `ouroboros`의 명세 우선 워크플로우를 대화 방식에만 참고한다.
- 별도 설치나 외부 도구 실행은 하지 않는다.
- 다음 루프를 작업 방식에 반영한다.

```text
Interview -> Seed -> Execute -> Evaluate
```

각 단계의 의미:

- Interview: 구현 전에 숨은 가정, 목표, 제약, 성공 기준을 확인한다.
- Seed: 확인된 내용을 실행 가능한 작은 명세로 정리한다.
- Execute: 명세에 맞춰 최소 범위로 구현하거나 문서를 수정한다.
- Evaluate: 기계적 검증, 의미적 검토, 사용자 목표와의 정합성을 확인한다.

이 내용은 `AGENTS.md`에도 반영했다.

### 5. Kubernetes 사용 시점 확인

사용자는 첫 번째 단계부터 실행하면 되는지, Kubernetes를 설치하지 않아도 괜찮은지 질문했다.

정리한 답변:

- 지금 당장은 Kubernetes가 없어도 된다.
- 첫 번째 단계는 Go 프로젝트 골격과 로컬 코드 구조를 만드는 작업이다.
- Kubernetes는 Envoy AI Gateway v0.5 배포와 마이그레이션 통합 검증 단계에서 사용한다.

단계별 구분:

```text
지금:
Go ExtProc 코드 골격 + 메모리 로직 구현

조금 뒤:
Dockerfile + 로컬 Redis 통합

그 다음:
Kind/Kubernetes 설치 및 Envoy AI Gateway v0.5 배포

최종:
v0.4 설정을 v0.5 설정으로 마이그레이션하고,
Memory ExtProc가 GatewayConfig 기반으로 붙는지 검증
```

### 6. v0.4 -> v0.5 마이그레이션 여부 확인

사용자는 이 프로젝트가 Envoy AI Gateway v0.4에서 v0.5로 마이그레이션하는 것이 맞는지 확인했다.

정리한 답변:

- 맞다.
- 이 프로젝트는 두 축으로 진행된다.

1. Envoy AI Gateway v0.4 -> v0.5 마이그레이션
2. v0.5 기반 Memory ExtProc PoC 구현

마이그레이션 대상 예시:

- `filterConfig.externalProcessor.resources` -> `GatewayConfig` CRD
- `schema.version` -> `schema.prefix`
- Kubernetes v1.32+
- Envoy Gateway v1.6.x
- Envoy Proxy v1.36.4
- Gateway API v1.4.0

주의점:

- 현재 저장소에는 실제 v0.4 설정 파일이 아직 없다.
- 따라서 마이그레이션 작업은 다음 둘 중 하나로 진행해야 한다.
  - 실제 v0.4 설정 파일을 나중에 받아서 변환한다.
  - README의 AS-IS/TO-BE 예시를 기준으로 샘플 v0.4 설정과 v0.5 설정을 만든다.

권장 다음 작업:

- v0.4 설정 샘플과 v0.5 GatewayConfig 샘플을 함께 만든다.
- 이렇게 하면 “마이그레이션 축”과 “Memory ExtProc 구현 축”이 분명히 분리된다.

## 현재까지의 결정 사항

- 기본 구현 방식: Option A, Custom ExtProc
- 기본 언어: Go
- 기본 세션 헤더: `x-session-id`
- 기본 메시지 포맷: OpenAI 호환 `messages`
- 기본 저장소: Redis
- 기본 메모리 범위: Short-term Memory
- Kubernetes 사용 시점: Gateway 통합 및 마이그레이션 검증 단계
- 대화/작업 방식: `Interview -> Seed -> Execute -> Evaluate`

## 다음 작업 후보

1. Go 설치 여부 확인 후 프로젝트 골격 생성
2. `go.mod` 초기화
3. `cmd/memory-extproc` 엔트리포인트 생성
4. `internal/openai` 메시지 타입과 병합 로직 작성
5. `internal/memory` Redis 저장소 인터페이스 설계
6. `docs/architecture.md` 작성
7. v0.4/v0.5 Gateway 설정 샘플 작성

## 남은 확인 사항

- 실제 v0.4 설정 파일이 존재하는지 여부
- LLM Provider를 OpenAI 호환 API로만 둘지, Anthropic 등도 추상화할지 여부
- Redis 장애 시 요청을 fail-open으로 통과시킬지, fail-closed로 차단할지 여부
- `x-session-id` 누락 시 임시 세션을 만들지, 오류로 처리할지 여부
- ExtProc에서 response body까지 처리할 때 AI Gateway/Envoy 설정상 필요한 processing mode

