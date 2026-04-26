# Envoy AI Gateway v0.5 Memory PoC 발자취 보고서

이 문서는 `kakao-envoy-ai-gateway` 프로젝트의 기획 의도부터 구현, 검증, 이슈 대응, 현재 상태와 남은 과제까지를 하나로 정리한 통합 보고서입니다.  
발표 자료의 베이스 문서로 사용할 수 있도록, "무엇을 왜 했고 어떻게 검증했는지"를 순서 중심으로 기록합니다.

---

## 1) 프로젝트 한 줄 요약

우리는 Envoy AI Gateway v0.5를 기반으로, Gateway 자체에 내장되지 않은 대화 메모리 기능을 **Custom External Processor + Redis**로 직접 구현해 실제 LLM Provider(OpenRouter/Upstage)에서 동작까지 검증했다.

---

## 2) 왜 필요한 프로젝트인가 (의의)

### 배경 문제

- LLM Chat API는 기본적으로 stateless다.
- 같은 사용자가 연속으로 대화해도, 이전 턴이 자동으로 기억되지 않는다.
- 기업 환경에서 공용 AI 게이트웨이를 운영할 때는 다음이 동시에 필요하다.
  - 요청 표준화
  - 보안/정책 제어
  - provider 전환 유연성
  - 세션 맥락 유지(대화 메모리)

### 우리가 선택한 해결 방향

- Envoy AI Gateway를 공용 관문으로 사용
- Memory ExtProc를 "대화 맥락 주입기"로 배치
- Redis를 세션 히스토리 저장소로 사용

### 프로젝트 의의

1. 메모리 내장형 제품 의존 없이, 표준 게이트웨이 확장으로 메모리 기능을 확보
2. provider를 바꿔도(OpenRouter/Upstage) 메모리 계층을 공통 재사용
3. v0.4 -> v0.5 마이그레이션과 메모리 PoC를 동시에 달성해 실무 전환 기반 확보

---

## 3) 목표와 범위

### Core 목표

- Envoy AI Gateway v0.5 전환
- Redis 기반 short-term memory PoC 구현
- 요청 `messages` 병합 + 응답 assistant 저장 end-to-end 동작
- Kubernetes(Kind)에서 재현 가능한 배포/검증 절차 확보

### Out of Scope (의도적으로 미포함)

- 운영 수준 보안/관측성 완성
- Long-term/Semantic memory
- Streaming/SSE 저장 완성
- 멀티테넌트 정책 완성

---

## 4) 작업 방식 (어떻게 진행했는가)

프로젝트 전반을 아래 루프로 운용했다.

1. Interview: 구현을 막는 질문만 짧게 확인
2. Seed: 작은 실행 단위로 목표/파일/완료기준 정의
3. Execute: Seed 범위 내 최소 변경으로 구현
4. Evaluate: 테스트/의미 검증/리스크 점검

실무적으로는 "코드 구현 + 문서 동시 갱신 + 이슈 로그 분리"를 원칙으로 유지했다.

---

## 5) 전체 진행 타임라인

## 2026-04-23: 방향 확정과 실행 규칙 정리

- README 중심으로 프로젝트 성격 재정의
- 기본 아키텍처를 Option A(Custom ExtProc)로 확정
- 언어를 Go로 확정
- `AGENTS.md`를 현재 프로젝트 맞춤 규칙으로 재작성
- Seed 기반 실행 순서 수립

핵심 산출:

- `AGENTS.md` 정비
- 초기 구조/원칙 확정
- `sessions/session-notes-2026-04-23.md`

## 2026-04-24: 구현과 v0.5 마이그레이션 본작업

### Seed 1~2: 구현 기초

- ExtProc gRPC 서버 구현
  - request headers: `x-session-id` 추출
  - request body: Redis load + messages 병합 + body mutation
  - response body: assistant 추출 + Redis append
- 로컬 실행 구성(Dockerfile, docker-compose) 정비

### Seed 3~6: Kubernetes/마이그레이션

- v0.5 연결 구조 확정
  - `GatewayConfig.spec.extProc.kubernetes`
  - `EnvoyExtensionPolicy`로 custom extproc 연결
- Kind 환경에서 v0.4 baseline smoke -> v0.5 전환 검증
- Redis + memory-extproc + Gateway end-to-end 검증

### Seed 7~8: 안정화/운영 문서

- 장애 시나리오 단위 테스트 보강
- 운영 체크리스트 문서화
- 재현 문서/OpenRouter 문서 작성

핵심 산출:

- 구현 코드: `cmd/`, `internal/`
- 배포 파일: `deploy/`
- 아키텍처/재현/운영 문서: `docs/`
- 이슈 로그 누적: `docs/issues/`
- `sessions/session-notes-2026-04-24.md`

## 2026-04-26: OpenRouter 경로 이슈 재추적과 우회 완성

- "content-length 단독 원인" 가설 재검증
- `echo-backend` 기반으로 실제 upstream body 관찰
- 결론:
  - custom extproc mutation 자체는 동작
  - 문제는 OpenRouter의 `AIGatewayRoute -> AIServiceBackend` 경로 상호작용에 집중됨
- 우회 경로 확정:
  - `HTTPRoute -> Backend(openrouter.ai) + URLRewrite + custom extproc`
  - 실제 2-turn 메모리 회상 검증 완료

추가 확장:

- Upstage Solar Pro 2 direct 경로 추가
- OpenRouter + Upstage 이원화 운영 검증 완료

---

## 6) 기술 아키텍처 (최종 PoC 기준)

```text
Client
  -> Gateway (Envoy)
  -> Custom memory-extproc (gRPC)
     - Redis에서 session history 조회
     - 요청 messages 병합
  -> LLM Provider (OpenRouter 또는 Upstage)
  <- 응답 수신
  -> memory-extproc가 assistant 메시지 Redis 저장
```

핵심 설계 포인트:

- 세션 식별: `x-session-id`
- 저장 키: `memory:session:{id}:messages`
- TTL 및 최대 히스토리 길이 제한
- Redis 장애: fail-open
- 세션 누락: pass-through

---

## 7) 어떤 순서/방법으로 구현했는가 (실행 관점)

1. 도메인 모델 확정
  - OpenAI 호환 `messages` 구조체/파서
2. Store 인터페이스 설계
  - Redis 구현체와 테스트 분리
3. ExtProc 파이프라인 구현
  - headers -> request body -> response body
4. 단위 테스트로 기본/장애 시나리오 검증
5. Kind + Helm + CRD 실환경 검증
6. Provider 연동(OpenRouter -> Upstage 확장)
7. 이슈별 재현 문서 + 해결/우회 문서화

---

## 8) 주요 문제와 해결 방법

이 섹션은 단순 "문제 목록"이 아니라, 실제로 팀이 어떤 순서와 기준으로 장애를 줄였는지 보여주는 핵심 파트다.  
각 항목은 다음 프레임으로 정리했다.

1. 발견 맥락
2. 관측된 증상(로그/에러)
3. 원인 추적 과정(가설과 반증)
4. 최종 해결 또는 우회
5. 검증 방식과 결과
6. 남긴 교훈

## 이슈 A: GatewayConfig 스키마가 문서와 달랐던 문제

### 1) 발견 맥락

v0.5 전환 중 `GatewayConfig`를 apply하던 Seed에서 가장 먼저 막힌 문제였다.  
프로젝트 문서와 공식 예시를 신뢰하고 적용했지만, 클러스터가 스키마 에러를 반환했다.

### 2) 관측된 증상

- `spec.extProc.env` 경로를 사용한 YAML이 apply 단계에서 거부됨
- 에러 메시지: `unknown field "spec.extProc.env"`

### 3) 원인 추적 과정

1. YAML 오타를 먼저 의심해 필드명 재확인
2. CRD 버전 드리프트 가능성을 의심
3. `kubectl explain gatewayconfig.spec.extProc.kubernetes`로 실제 스키마 조회
4. 결과: 실제 설치된 v0.5.0 CRD는 `spec.extProc.kubernetes.env` 구조를 요구

즉, 문제는 코드가 아니라 "문서 기준값과 실제 설치 CRD 사이의 불일치"였다.

### 4) 최종 해결

- 배포 파일에서 `spec.extProc.env` -> `spec.extProc.kubernetes.env`로 수정
- `resources` 필드도 동일하게 `kubernetes` 하위로 이동
- 관련 문서와 예시 전체 동기화

### 5) 검증 방식과 결과

1. 최소 GatewayConfig YAML을 별도 apply
2. `gatewayconfig created` 확인
3. 기존 샘플 재적용 후 Gateway 연동 smoke 재실행
4. v0.5 경로 HTTP 200 확인

### 6) 교훈

- "공식 문서에 있으니 맞다"는 가정은 버전 경계에서 위험하다.
- Gateway/CRD 계열 작업은 반드시 `kubectl explain`을 1차 진실 소스로 사용해야 한다.

관련 기록: `docs/issues/2026-04-24-gatewayconfig-extproc-schema.md`

## 이슈 B: body mutation 후 500 (`content-length` mismatch)

### 1) 발견 맥락

memory-extproc를 붙인 뒤, 1턴 호출은 성공했지만 2턴(히스토리 병합)에서만 500이 발생했다.  
즉, "요청 본문 길이가 달라지는 상황"에서만 깨지는 문제였다.

### 2) 관측된 증상

- Envoy 응답 코드 500
- `response_code_details: mismatch_between_content_length_and_the_length_of_the_mutated_body`

### 3) 원인 추적 과정

1. Redis 조회 실패 여부 확인 -> 정상
2. JSON 파싱/병합 로직 확인 -> 정상
3. 실제 mutated body 길이 로그 추가 -> 2턴에서 길이 증가 확인
4. 요청 헤더와 body 치환 방식 비교 -> `Content-Length`가 원본 길이로 남아 있음을 확인

문제 본질은 "본문은 교체됐는데 길이 헤더는 원본값"이라는 프로토콜 불일치였다.

### 4) 최종 해결

- request body 교체 시점(`CONTINUE_AND_REPLACE`)에서 header 처리 정책 정비
- 초기에는 `content-length` 직접 업데이트 방식으로 해결해 500을 멈춤
- 이후 OpenRouter 재검증 단계에서 보호 헤더 제약이 드러나며,
  request headers 단계에서 `content-length` 제거 + 전송 경로 재검증 방식으로 보완

중요 포인트는 "한 번에 완벽 해결"이 아니라 "에러 유형별로 처리 정책을 분리"한 것이다.

### 5) 검증 방식과 결과

1. 동일 세션 2턴 호출 반복
2. 1턴/2턴 모두 HTTP 200 확인
3. Redis 저장 순서 확인 (`user -> assistant -> user -> assistant`)
4. extproc debug 로그로 `merged_msgs`, `mutated_body_len` 확인

### 6) 교훈

- body mutation 계열 문제는 애플리케이션 로직보다 HTTP 프레이밍과 Envoy 필터 제약이 더 중요할 때가 많다.
- "코드가 맞다"와 "프록시 체인에서 유효하다"는 별개의 검증 단계다.

관련 기록: `docs/issues/2026-04-24-content-length-mismatch.md`

## 이슈 C: OpenRouter HTTPS 연결 실패 (TLS/CA/Host)

### 1) 발견 맥락

mock backend 검증이 끝난 뒤 실제 provider로 전환하면서 첫 실서비스 호출에서 막혔다.  
즉, 비즈니스 로직 이전에 네트워크/신뢰체인 레벨에서 실패한 케이스다.

### 2) 관측된 증상

- `400 The plain HTTP request was sent to HTTPS port`
- 환경에 따라 `CERTIFICATE_VERIFY_FAILED`

### 3) 원인 추적 과정

1. API 키 문제 가능성 점검 -> 키는 정상
2. 경로(prefix) 문제 가능성 점검 -> 경로는 정상
3. backend transport 레벨 점검 -> 443 endpoint 사용 중이나 TLS 참조 누락 확인
4. 인증서 체인/Host 헤더 요구사항 확인 -> 로컬 Kind에서는 CA 체인 주입 필요

결론적으로 문제는 인증이 아니라 upstream TLS handshake 설정이었다.

### 4) 최종 해결

1. `Backend.spec.tls.caCertificateRefs` 설정
2. `openrouter-ca` ConfigMap 생성/참조
3. 포트포워딩 호출 시 `Host: openrouter.ai` 헤더 명시

### 5) 검증 방식과 결과

1. TLS 적용 전후 동일 요청 비교
2. 400/503 계열 에러 소거 확인
3. OpenRouter 실응답 HTTP 200 확인
4. 이후 메모리 경로 검증으로 단계 이동

### 6) 교훈

- 외부 provider 연동에서는 애플리케이션보다 먼저 "L4/L7 신뢰 설정"이 안정화되어야 한다.
- "키를 넣었는데 안 된다"는 대부분 TLS/Host/SNI 계층 문제일 수 있다.

관련 기록: `docs/issues/2026-04-24-openrouter-backend-tls.md`

## 이슈 D: OpenRouter 경로에서 메모리 미반영 (가장 어려운 문제)

### 1) 발견 맥락

겉으로 보면 모든 것이 정상처럼 보였다.

- OpenRouter 호출 성공(HTTP 200)
- Redis 저장 성공
- memory-extproc 로그에서 `merged_msgs` 증가 확인

그런데 2턴 답변은 1턴 내용을 기억하지 못했다.

### 2) 관측된 증상

- 2턴 응답에서 이름 회상 실패
- `usage.prompt_tokens`가 기대보다 낮음(히스토리 주입 부재 정황)
- Redis에는 `user/assistant`가 정상 누적됨

### 3) 원인 추적 과정 (가설 -> 반증)

1. 가설 A: `content-length` 보호 헤더 때문에 mutation 자체가 실패
2. 검증 A-1: `test-gateway + HTTPRoute -> echo-backend`
3. 결과 A-1: 같은 custom extproc 코드로 upstream body에 병합 `messages`가 반영됨
4. 검증 A-2: `openrouter-ai-gateway`에 임시 echo 라우트 추가
5. 결과 A-2: 이 경로에서도 병합 반영됨
6. 최종 판단: "mutation 자체 불가"는 반증됨
7. 새로운 가설: `AIGatewayRoute -> AIServiceBackend` 경로 내부 처리와 custom extproc 상호작용 문제

즉, 문제는 extproc 단일 기능이 아니라 "AI Gateway 내장 처리 단계와의 조합"으로 좁혀졌다.

### 4) 해결 전략

완전 원인 제거와 서비스 지속 사이에서, 다음 투트랙으로 결정했다.

1. 추적 트랙:
  - 기존 AIGatewayRoute 경로의 원인 조사 지속
2. 운영 트랙:
  - 실제 동작이 검증된 direct route 경로를 우회안으로 제품화

우회안:

- `HTTPRoute -> Backend(openrouter.ai) + URLRewrite + custom extproc`

### 5) 검증 방식과 결과

1. 우회 매니페스트 적용
2. 2턴 호출 테스트
3. 2턴 회상 성공 확인
4. `prompt_tokens` 증가 확인
5. Redis 누적 순서 확인

결과적으로 "핵심 PoC 목표(실 provider 메모리 동작 입증)"는 달성했다.

### 6) 교훈

- 분산 시스템 디버깅은 "한 가지 원인으로 설명되는가"를 끝까지 반증해야 한다.
- 완전 해결 전에도 검증된 우회 경로를 공식 산출물로 남기면 프로젝트 모멘텀을 유지할 수 있다.

관련 기록:

- `docs/issues/2026-04-24-openrouter-memory-injection-not-reflected.md`
- `docs/issues/2026-04-26-body-mutation-content-length-blocked.md`

## 이슈 E: Upstage direct 최초 호출 400 (HTTPS 포트 평문 요청)

### 1) 발견 맥락

OpenRouter 이원화가 성공한 뒤 Upstage direct를 붙였을 때, 첫 호출에서 바로 400이 발생했다.

### 2) 관측된 증상

- `400 The plain HTTP request was sent to HTTPS port`

### 3) 원인 추적 과정

1. API 키/모델명 문제 가능성 점검
2. 요청 형식은 OpenAI 호환으로 정상 확인
3. backend TLS 설정 누락 확인

OpenRouter에서 겪었던 TLS 패턴이 Upstage에서도 재현된 사례였다.

### 4) 최종 해결

1. `Backend(upstage-backend)`에 TLS 참조 추가
2. `upstage-ca` ConfigMap 생성 후 `caCertificateRefs` 연결
3. 재배포 후 동일 요청 재시험

### 5) 검증 방식과 결과

1. 1턴: 정상 응답(모델 `solar-pro2-251215`)
2. 2턴: 이름 회상 성공
3. `prompt_tokens` 증가
4. Redis 저장 확인

### 6) 교훈

- provider가 달라도 외부 HTTPS endpoint 연동 이슈의 패턴은 반복된다.
- 그래서 이번에는 "이슈 재발 방지 문서"를 즉시 `docs/upstage-provider.md`에 반영했다.

관련 기록:

- `docs/upstage-provider.md`
- `deploy/gateway/v0.5-upstage-direct-sample.yaml`

---

## 9) 실제 검증 방법 (실검증)

검증은 의미/정량/저장 3축으로 수행했다.

1. 의미 검증
  - 1턴: "내 이름은 ..."
  - 2턴: "내 이름이 뭐라고 했지?"
  - 2턴에서 이름 회상 여부 확인

2. 정량 검증
  - provider 응답의 `usage.prompt_tokens` 비교
  - 2턴 토큰 증가 시 히스토리 주입 정황 확인

3. 저장 검증
  - Redis `LRANGE`로 메시지 순서 확인
  - `user -> assistant -> user -> assistant` 확인

추가로 provider 분리 검증:

- `or-demo`와 `up-demo`를 분리해 세션 혼선 여부 확인

---

## 10) 현재 완료 상태

## 완료된 항목

- v0.5 환경 전환 및 배포 샘플
- custom memory-extproc 구현
- Redis 세션 메모리 저장/조회
- OpenRouter direct 경로 실검증
- Upstage Solar Pro 2 direct 경로 실검증
- OpenRouter + Upstage 이원화 운영 검증
- 재현/운영/이슈 문서 체계 구축

## 실용적으로 사용 가능한 경로

- OpenRouter: `deploy/gateway/v0.5-openrouter-direct-sample.yaml`
- Upstage: `deploy/gateway/v0.5-upstage-direct-sample.yaml`

---

## 11) 남은 문제와 리스크

1. OpenRouter AIGatewayRoute 경로 미반영 이슈
- `AIGatewayRoute -> AIServiceBackend` 조합에서 custom body mutation 최종 반영 문제 추적 필요

2. Streaming/SSE 응답 메모리 저장
- 현재 PoC는 `stream: false` 중심

3. 운영화 과제
- 보안(키 로테이션/권한 분리)
- 관측성(메트릭/트레이싱)
- Redis HA/지속성 정책
- 멀티테넌시 정책

4. 문서 최신화
- 세션 기록과 최신 우회 경로 내용을 주기적으로 동기화 필요

---

## 12) 왜 이 방식이 유효했는가 (회고)

이번 프로젝트에서 효과적이었던 점:

1. Seed 단위 분할
- 큰 목표를 작게 쪼개 진행해서 병목을 빠르게 발견

2. "코드 + 문서 + 이슈 로그" 동시 운영
- 실패/재현/해결 맥락이 누적되어 같은 실수를 줄임

3. mock -> 실제 provider 순차 검증
- 로직 자체 문제와 provider 경로 문제를 분리 가능

4. 우회 경로를 제품화
- 원인 미해결 이슈가 있어도, 검증 가능한 운영 경로를 먼저 확보

---

## 13) 발표용 요약 (짧은 버전)

우리 프로젝트는 Envoy AI Gateway v0.5 위에 custom extproc와 Redis를 붙여 LLM 대화 메모리를 직접 구현한 PoC다.  
핵심은 요청 시 히스토리를 `messages`에 병합하고, 응답의 assistant 메시지를 Redis에 누적해 같은 세션의 다음 요청에 다시 주입하는 흐름이다.

진행 과정에서 GatewayConfig 스키마 불일치, body mutation/content-length 이슈, OpenRouter TLS 이슈, OpenRouter 특정 경로의 메모리 미반영 같은 문제를 만났고, 각각 재현-원인분리-문서화-수정 또는 우회로 대응했다.  
특히 OpenRouter AIGatewayRoute 경로는 아직 추적 중이지만, direct route 우회로는 실검증까지 완료해 PoC 목표(실제 provider에서 메모리 동작 입증)는 달성했다.

최종적으로 OpenRouter와 Upstage를 이원화해 동시 운영 가능성을 검증했고, provider가 달라도 메모리 계층을 공통 재사용할 수 있음을 확인했다.

---

## 14) 관련 문서 맵

- 기준 문서: `README.md`, `AGENTS.md`
- 아키텍처: `docs/architecture.md`
- 재현 절차: `docs/reproduce-v05-memory-poc.md`
- OpenRouter: `docs/openrouter-provider.md`
- Upstage: `docs/upstage-provider.md`
- 이원화: `docs/dual-provider-setup.md`
- 운영 체크: `docs/ops-checklist.md`
- 이슈 기록: `docs/issues/*.md`
- 세션 기록: `sessions/session-notes-2026-04-23.md`, `sessions/session-notes-2026-04-24.md`
