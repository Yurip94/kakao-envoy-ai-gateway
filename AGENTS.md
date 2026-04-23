# AGENTS.md

이 문서는 `kakao-envoy` 프로젝트에서 에이전트가 작업을 시작할 때 가장 먼저 확인해야 하는 공용 진입점입니다.
모든 작업은 `README.md`의 프로젝트 목표와 범위를 기준으로 진행합니다.

## 적용 범위

- 저장소 전체(`.`)에 적용합니다.
- 코드 구현, 문서 작성, 아키텍처 검토, 테스트, 실행 스크립트 작성에 공통 적용합니다.
- 이 저장소는 Envoy AI Gateway v0.5 기반 LLM 대화 메모리 PoC를 새로 구현하는 프로젝트입니다.

## 작업 시작 순서

1. 사용자 요청을 작업 유형으로 분류합니다.
   - 문서 정리
   - PoC 코드 구현
   - Kubernetes/Helm 매니페스트 작성
   - Envoy AI Gateway 설정
   - Memory ExtProc 구현
   - Redis 연동
   - 테스트/검증
2. 반드시 `README.md`를 먼저 확인해 현재 프로젝트 목표, 범위, 아키텍처 방향을 파악합니다.
3. 구현 작업이면 기본 방향을 **Option A: Custom External Processor**로 둡니다.
4. 요청 범위가 README의 Out of Scope에 해당하면, 바로 구현하지 않고 범위 조정 필요 사항을 먼저 알립니다.
5. 작업 완료 전 변경 범위, 검증 결과, 남은 리스크를 분리해서 확인합니다.

## 대화 및 명세 우선 작업 방식

이 프로젝트에서는 `ouroboros`의 명세 우선 워크플로우를 참고하되, 별도 설치나 외부 도구 실행 없이 대화 방식에만 가볍게 적용합니다.
`ouroboros` 자체를 설치하거나 `ooo` 명령을 실행하는 것이 목적이 아니라, "프롬프트를 바로 구현으로 넘기기 전에 작은 명세로 정리한다"는 작업 방식을 지속적으로 따르는 것이 목적입니다.

### 기본 루프

1. Interview: 구현 전에 숨은 가정, 목표, 제약, 성공 기준을 짧게 확인합니다.
2. Seed: 확인된 내용을 실행 가능한 작은 명세로 정리합니다.
3. Execute: 명세에 맞춰 최소 범위로 구현하거나 문서를 수정합니다.
4. Evaluate: 기계적 검증, 의미적 검토, 사용자 목표와의 정합성을 확인합니다.

### 적용 원칙

- 모호한 요청은 바로 구현하지 않고, 막히는 질문만 선별해서 먼저 묻습니다.
- 충분히 명확한 요청은 질문을 늘리지 않고 바로 실행합니다.
- 큰 작업은 작은 Seed 단위로 쪼개고, 각 단위마다 완료 기준을 둡니다.
- 작업 중 목표가 바뀌면 기존 Seed를 고집하지 않고 다시 정리합니다.
- 검증 결과는 다음 작업의 입력으로 반영합니다.

### 이 프로젝트에서의 운영 규칙

- 각 작업을 시작할 때 현재 요청이 어떤 Seed에 해당하는지 짧게 명명합니다. 예: `Seed 1: 저장소 기본 정리`, `Seed 2: 아키텍처 명세 작성`.
- Interview 단계에서는 구현을 막는 질문만 묻고, 이미 README/AGENTS/session notes에서 답을 찾을 수 있는 내용은 다시 묻지 않습니다.
- Seed 단계에서는 목표, 변경 대상 파일, 완료 기준, 범위 밖 항목을 간단히 정리합니다.
- Execute 단계에서는 Seed 범위를 벗어나는 리팩터링이나 선택 기능 구현을 임의로 추가하지 않습니다.
- Evaluate 단계에서는 변경 범위, 실행한 검증, 실행하지 못한 검증, 남은 리스크, 다음 Seed 후보를 분리해서 보고합니다.
- 세션이 길어지거나 중요한 결정이 생기면 `sessions/` 폴더의 세션 기록에 이어서 남길 수 있습니다.

## 프로젝트 기준 문서

### README.md

- 역할: 프로젝트 목표, 범위, v0.5 변경사항, 메모리 구현 방향, 일정, 참고 자료를 정의하는 기준 문서
- 우선 확인 시점:
  - 기능 우선순위 판단
  - 아키텍처 선택
  - 구현 범위 조정
  - 문서 정합성 점검
  - 테스트 시나리오 도출

### AGENTS.md

- 역할: 에이전트의 작업 절차와 구현 원칙을 정의하는 실행 규칙 문서
- README의 내용을 대체하지 않고, README를 실제 작업 규칙으로 해석하는 보조 문서로 사용합니다.

## 프로젝트 목표

- Envoy AI Gateway v0.4 기준 구성을 v0.5 기준으로 전환합니다.
- Envoy AI Gateway v0.5의 확장 기능을 활용해 LLM 대화 메모리 PoC를 구현합니다.
- Gateway 자체에 내장되지 않은 대화 메모리를 외부 저장소와 External Processor로 직접 구현합니다.
- 1차 PoC는 Short-term Memory에 집중하고, Long-term/Semantic Memory는 후속 확장으로 둡니다.

## 기본 구현 방향

### 선택 아키텍처

- 기본 선택지는 `Option A: Custom ExtProc`입니다.
- 클라이언트는 `x-session-id` 헤더와 현재 요청 메시지를 전달합니다.
- Memory ExtProc는 세션 ID를 기준으로 Redis에서 히스토리를 조회합니다.
- Memory ExtProc는 기존 히스토리와 현재 `messages`를 병합해 LLM 요청 본문을 수정합니다.
- LLM 응답 이후 assistant 메시지를 Redis에 저장합니다.

### 초기 실행 순서

1. PoC 언어와 저장소 골격을 정합니다.
   - 기본 선택: Go
   - 이유: Envoy/gRPC/protobuf 생태계와 잘 맞고, ExtProc 서비스 구현 및 컨테이너 배포가 단순합니다.
   - 초기 구조: `cmd/memory-extproc`, `internal/memory`, `internal/openai`, `internal/extproc`, `deploy`, `examples`, `docs`
2. ExtProc 구현 방식을 확정합니다.
   - 기본 선택: gRPC External Processor 서버를 직접 구현합니다.
   - 요청 경로: request headers/body 처리, Redis 히스토리 조회, `messages` 병합, body mutation 반환
   - 응답 경로: response body 처리, assistant 메시지 추출, Redis 저장
3. `docs/architecture.md`를 작성합니다.
   - README의 Option A를 실제 구현 명세로 구체화합니다.
   - 세션 모델, Redis key, TTL, 실패 정책, 테스트 시나리오를 포함합니다.
4. Kind + Helm 배포 매니페스트를 준비합니다.
   - Kind 클러스터, Envoy Gateway, AI Gateway, Redis, Memory ExtProc 배포 순서가 재현 가능해야 합니다.
   - 로컬 PoC 실행과 Kubernetes 배포를 분리해 검증할 수 있게 합니다.

### Option B 사용 조건

- Option B는 기본 방향이 아닙니다.
- ExtProc 구현이 PoC 일정 또는 기술 제약으로 막힌 경우에만 fallback 후보로 검토합니다.
- Option B로 전환하려면 먼저 사용자에게 전환 사유, 손실되는 검증 범위, 남는 산출물을 설명합니다.

## 기술 기준

### 핵심 컴포넌트

- Envoy AI Gateway v0.5
- Envoy Gateway v1.6.x
- Kubernetes v1.32+
- Gateway API v1.4.0
- Redis
- Custom External Processor

### Memory ExtProc

- 언어는 기본적으로 Go를 사용합니다.
- Python은 사용자가 명시적으로 요청하거나, 특정 검증 단계에서 Go보다 현저히 빠른 임시 프로토타입이 필요할 때만 선택합니다.
- OpenAI 호환 `messages` 배열을 기본 메시지 포맷으로 취급합니다.
- 세션 식별자는 기본적으로 `x-session-id` 헤더를 사용합니다.
- Redis key는 세션 단위로 분리하고, TTL을 적용할 수 있게 설계합니다.
- 최대 히스토리 길이, TTL, Redis URL은 환경변수로 설정 가능하게 둡니다.

### Gateway 설정

- v0.5 기준 `GatewayConfig` CRD를 사용합니다.
- deprecated 항목인 `filterConfig.externalProcessor.resources`와 `schema.version`을 새 설정에 남기지 않습니다.
- `schema.prefix`와 GatewayConfig 기반 extProc 설정을 우선합니다.
- Body Mutation은 top-level 필드 제한이 있음을 전제로 설계합니다.

## 구현 범위 관리

### Core

- v0.5 환경 구성 및 배포 예제
- GatewayConfig 기반 ExtProc 설정
- Redis 기반 세션 히스토리 저장/조회
- 요청 본문 `messages` 병합
- 응답에서 assistant 메시지 추출 및 저장
- 기본 대화 메모리 시나리오 테스트

### Optional

- MCP 세션 메모리
- Long-term Memory
- Semantic Memory
- Redis Vector Search
- 성능 벤치마크

### Out of Scope

- 프로덕션 배포
- 전체 서비스 마이그레이션
- 운영 수준 보안/관측성 완성
- 멀티 테넌트 정책 완성

## 코드 작성 원칙

- 최소 구현으로 시작하되, PoC 검증에 필요한 흐름은 end-to-end로 연결합니다.
- 저장소 구조가 아직 없으면 구현 목적이 드러나는 단순한 디렉터리 구조를 먼저 만듭니다.
- 설정값은 코드에 고정하지 말고 환경변수 또는 설정 파일로 분리합니다.
- OpenAI 호환 요청/응답 JSON은 구조체나 스키마 기반으로 다루고, 취약한 문자열 조작은 피합니다.
- Redis 장애, 세션 ID 누락, 빈 히스토리, 잘못된 JSON 같은 실패 경로를 명시적으로 처리합니다.
- 민감정보는 코드, 문서, 로그에 직접 기록하지 않습니다.

## 권장 저장소 구조

저장소 구조가 아직 확정되지 않았을 때는 아래 구성을 우선 고려합니다.

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

이 구조는 구현 언어와 프레임워크가 정해지면 조정할 수 있습니다.

## 테스트 및 검증 기준

### 기본 시나리오

- 같은 `x-session-id`에서 두 번째 요청이 첫 번째 대화를 참조할 수 있어야 합니다.
- 서로 다른 세션의 히스토리는 섞이지 않아야 합니다.
- Redis TTL 만료 후에는 이전 히스토리가 주입되지 않아야 합니다.
- 최대 히스토리 길이를 초과하면 오래된 메시지를 제한해야 합니다.

### 장애 시나리오

- `x-session-id` 누락 시 명확한 fallback 또는 오류 정책을 적용합니다.
- Redis 연결 실패 시 요청을 차단할지, 메모리 없이 통과시킬지 정책을 코드와 문서에 남깁니다.
- 요청 본문이 OpenAI 호환 형식이 아니면 안전하게 실패해야 합니다.
- LLM 응답에서 assistant 메시지를 추출할 수 없는 경우 저장을 건너뛰고 로그로 확인 가능해야 합니다.

## 문서 작성 원칙

- 문서는 Markdown으로 작성합니다.
- 구현 변경과 함께 사용법, 설정값, 테스트 방법이 바뀌면 관련 문서를 함께 갱신합니다.
- 문서에는 실제 명령어와 예상 결과를 구분해 적습니다.
- 외부 문서 내용을 인용하거나 기준으로 삼을 때는 출처 링크를 남깁니다.
- Envoy AI Gateway, Envoy Gateway, Gateway API처럼 버전 변동 가능성이 큰 정보는 필요 시 공식 문서를 확인합니다.

### 채팅 세션 기록

- 앞으로 채팅 내용이나 작업 세션 기록을 파일로 저장할 때는 저장소 루트가 아니라 `sessions/` 폴더에 저장합니다.
- 세션 기록 파일은 날짜를 포함한 Markdown 파일명을 사용합니다. 예: `sessions/session-notes-YYYY-MM-DD.md`

## Git 및 작업 방식

- 현재 저장소가 Git 저장소가 아닐 수 있으므로, Git 작업은 사용자가 명시적으로 요청했을 때만 수행합니다.
- 커밋, 브랜치, PR 생성은 사용자 요청 또는 저장소 초기화 이후에만 진행합니다.
- 사용자가 커밋을 요청하면 변경 사항을 먼저 요약하고 검증 결과를 확인한 뒤 진행합니다.
- 기존 사용자 변경사항은 되돌리지 않습니다.

## 보안 및 민감정보

- API 키, 토큰, `.env` 비밀값은 출력하거나 커밋하지 않습니다.
- 예제에는 placeholder 값을 사용합니다.
- 로그에는 요청/응답 전문을 남기지 않는 것을 기본값으로 둡니다.
- 대화 히스토리는 개인정보가 포함될 수 있으므로 TTL, 삭제 정책, 최소 저장 원칙을 고려합니다.

## 작업 완료 점검

- `README.md`의 목표와 어긋난 변경이 없는지 확인합니다.
- Option A 기준 흐름을 깨지 않았는지 확인합니다.
- 변경 파일과 변경 이유를 요약합니다.
- 실행한 테스트와 결과를 분리해서 보고합니다.
- 실행하지 못한 검증이 있으면 이유와 남은 리스크를 명시합니다.
