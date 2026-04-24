# 운영 전 체크리스트

이 문서는 Envoy AI Gateway v0.5 + Custom Memory ExtProc + Redis 기반 대화 메모리 PoC를 실제 서비스 환경에 적용하기 전에 확인해야 할 항목을 정리합니다.

현재 저장소의 산출물은 **운영 배포본이 아니라 PoC**입니다.
따라서 이 체크리스트는 "지금 바로 운영에 넣는다"가 아니라, 운영 수준으로 올리기 전에 무엇을 닫아야 하는지 확인하는 기준입니다.

## 현재 PoC 기준

| 항목 | 현재 값 |
|------|---------|
| Envoy AI Gateway | v0.5.0 |
| Envoy Gateway | v1.6.x |
| Kubernetes | v1.32+ |
| Gateway API | v1.4.0 |
| Memory 구현 방식 | Option A: Custom ExtProc |
| ExtProc 연결 방식 | `EnvoyExtensionPolicy` |
| 메모리 저장소 | Redis |
| 세션 식별자 | `x-session-id` |
| Redis 장애 정책 | `fail-open` |
| 세션 ID 누락 정책 | `pass-through` |
| 응답 처리 모드 | `Buffered` |
| Streaming 응답 | 1차 PoC 범위 제외 |

## 1. 배포 구조

- [ ] Envoy Gateway와 Envoy AI Gateway controller 설치 namespace를 구분했는가?
- [ ] Gateway, GatewayConfig, AIGatewayRoute, Backend, AIServiceBackend가 같은 의도된 namespace에 있는가?
- [ ] `EnvoyExtensionPolicy`가 대상 Gateway와 같은 namespace에서 올바른 `targetRefs`를 가리키는가?
- [ ] `EnvoyExtensionPolicy.extProc.backendRefs`가 실제 `memory-extproc` Service 이름과 port `50051`을 가리키는가?
- [ ] Redis Service 주소가 `REDIS_URL`과 일치하는가?
- [ ] PoC처럼 `default` namespace를 쓸지, 서비스 전용 namespace를 만들지 결정했는가?

확인 명령 예시:

```bash
kubectl get gatewayconfig,envoyextensionpolicy,gateway,aigatewayroute,aiservicebackend,backend -A
kubectl get svc -A | grep -E 'memory-extproc|redis'
kubectl describe envoyextensionpolicy -n default memory-extproc-policy
```

## 2. 버전과 CRD 스키마

- [ ] Envoy AI Gateway v0.5 CRD가 설치되어 있는가?
- [ ] `GatewayConfig` apiVersion이 설치된 CRD와 일치하는가?
- [ ] `GatewayConfig.spec.extProc.kubernetes.env`와 `resources` 필드가 실제 CRD에서 설명되는가?
- [ ] v0.4 deprecated 설정인 `filterConfig.externalProcessor.resources`가 v0.5 target에 남아 있지 않은가?
- [ ] `schema.version` 대신 `schema.prefix`를 사용하는가?
- [ ] 샘플 YAML을 server dry-run으로 검증했는가?

확인 명령 예시:

```bash
kubectl get crd | grep aigateway
kubectl explain gatewayconfig.spec.extProc.kubernetes --api-version=aigateway.envoyproxy.io/v1alpha1
kubectl apply --dry-run=server -f deploy/gateway/v0.5-gateway-config-sample.yaml
```

## 3. Memory ExtProc

- [ ] `memory-extproc` 이미지가 대상 클러스터 아키텍처에 맞게 빌드되었는가?
- [ ] `memory-extproc` Pod가 `Running` 상태인가?
- [ ] gRPC listen 주소가 `LISTEN_ADDR=:50051` 또는 의도한 값인가?
- [ ] `SESSION_HEADER`가 클라이언트가 보내는 헤더와 일치하는가?
- [ ] request body 처리에서 `messages` 병합이 수행되는가?
- [ ] response body 처리에서 assistant 메시지 저장이 수행되는가?
- [ ] body 교체 시 `content-length`가 함께 갱신되는가?
- [ ] 잘못된 JSON 요청을 mutation 없이 통과시키는가?
- [ ] assistant 메시지 추출 실패 시 요청 흐름을 깨지 않고 저장만 건너뛰는가?

확인 명령 예시:

```bash
kubectl logs -n default deploy/memory-extproc --tail=100
kubectl get pods -n default -l app=memory-extproc
```

## 4. Redis

- [ ] 운영 환경에서 Redis auth를 활성화할지 결정했는가?
- [ ] 운영 환경에서 Redis persistence를 활성화할지 결정했는가?
- [ ] Redis HA 또는 managed Redis 사용 여부를 결정했는가?
- [ ] `REDIS_URL`에 민감정보가 포함될 경우 Secret으로 관리하는가?
- [ ] `MEMORY_TTL_SECONDS` 기본값 `3600`이 서비스 요구사항에 맞는가?
- [ ] `MAX_HISTORY_LENGTH` 기본값 `20`이 토큰 비용과 품질 사이에서 적절한가?
- [ ] Redis key 패턴 `memory:session:{session_id}:messages`가 서비스 정책에 맞는가?
- [ ] TTL 만료 후 히스토리가 제거되는지 검증했는가?
- [ ] 서로 다른 session ID의 히스토리가 섞이지 않는지 검증했는가?

PoC의 `deploy/redis/values.yaml`은 auth와 persistence를 끈 상태입니다.
운영 환경에서는 그대로 쓰지 말고 반드시 보안과 내구성 정책을 다시 결정해야 합니다.

## 5. 보안과 민감정보

- [ ] LLM Provider API key를 매니페스트에 평문으로 넣지 않는가?
- [ ] Redis password, API token, provider credential은 Kubernetes Secret 또는 외부 Secret 관리 도구를 사용하는가?
- [ ] 로그에 요청/응답 전문을 남기지 않는가?
- [ ] 대화 히스토리에 개인정보가 포함될 수 있음을 전제로 TTL과 삭제 정책을 정했는가?
- [ ] `x-session-id`가 추측 가능한 값일 경우 세션 혼선을 막을 방법이 있는가?
- [ ] 서비스별/사용자별 권한을 Gateway 또는 상위 인증 계층에서 확인하는가?
- [ ] 운영 로그와 이슈 문서에 민감정보를 그대로 붙여넣지 않는 절차가 있는가?

## 6. 실패 정책

- [ ] Redis 장애 시 `fail-open`을 유지할지, 특정 서비스에서는 `fail-closed`가 필요한지 결정했는가?
- [ ] `x-session-id` 누락 시 `pass-through`를 유지할지, 세션 필수 서비스는 차단할지 결정했는가?
- [ ] Memory ExtProc 장애 시 Gateway 요청을 어떻게 처리할지 정했는가?
- [ ] 요청 body가 너무 크거나 `messages`가 비정상일 때의 정책을 정했는가?
- [ ] LLM 응답이 OpenAI 호환 형태가 아닐 때 저장을 건너뛰는 것이 허용되는가?
- [ ] 장애 상황을 사용자 응답, Gateway 로그, ExtProc 로그 중 어디에서 확인할지 정했는가?

현재 PoC 기본값:

```text
REDIS_FAILURE_POLICY=fail-open
MISSING_SESSION_POLICY=pass-through
```

## 7. 관측성

- [ ] Memory ExtProc 로그에 stage, session 존재 여부, 오류 원인이 남는가?
- [ ] 요청/응답 전문 대신 요약 정보만 남기는가?
- [ ] Redis load/append 실패 건수를 관측할 수 있는가?
- [ ] body mutation 성공/실패 건수를 관측할 수 있는가?
- [ ] Gateway route별 요청 수, 오류율, 지연 시간을 볼 수 있는가?
- [ ] ExtProc gRPC 지연 시간이 전체 LLM 요청 지연에 얼마나 영향을 주는지 측정할 수 있는가?
- [ ] 운영 환경에서 로그 retention과 접근 권한이 정해져 있는가?

후속 구현 후보:

- Prometheus metrics endpoint
- structured logging
- trace ID propagation
- session ID hash logging

## 8. 성능과 비용

- [ ] `processingMode.request.body: Buffered`가 요청 크기와 지연 시간에 미치는 영향을 확인했는가?
- [ ] `processingMode.response.body: Buffered`가 응답 크기와 지연 시간에 미치는 영향을 확인했는가?
- [ ] `MAX_HISTORY_LENGTH` 증가로 인한 토큰 비용 증가를 산정했는가?
- [ ] 큰 대화 히스토리에서 Redis 조회/직렬화 비용을 측정했는가?
- [ ] 서비스별로 다른 TTL과 history length가 필요한지 검토했는가?
- [ ] 부하 테스트 기준 요청 수, 동시성, p95/p99 latency 목표를 정했는가?

주의:

- 히스토리를 많이 주입할수록 LLM 입력 토큰이 증가합니다.
- body buffering은 streaming 응답과 궁합이 좋지 않으므로 별도 설계가 필요합니다.

## 9. 테스트

운영 전 최소 확인 시나리오:

- [ ] 같은 `x-session-id`에서 두 번째 요청에 첫 번째 대화가 주입된다.
- [ ] 서로 다른 `x-session-id`의 히스토리가 섞이지 않는다.
- [ ] TTL 만료 후 이전 히스토리가 주입되지 않는다.
- [ ] `MAX_HISTORY_LENGTH` 초과 시 오래된 메시지가 제거된다.
- [ ] Redis 장애 시 정책대로 fail-open 또는 fail-closed 된다.
- [ ] `x-session-id` 누락 시 정책대로 pass-through 또는 fail-closed 된다.
- [ ] 잘못된 JSON 요청이 Gateway 흐름을 깨지 않는다.
- [ ] assistant 메시지 추출 실패 시 저장만 건너뛰고 응답은 통과한다.
- [ ] v0.5 target 매니페스트가 server dry-run을 통과한다.
- [ ] 실제 Gateway 경로 `/v1/chat/completions`로 smoke test가 성공한다.

현재 저장소에서 기본 검증:

```bash
go test ./...
go build ./...
ruby -e 'require "yaml"; ARGV.each { |f| YAML.load_stream(File.read(f)); puts "ok #{f}" }' \
  deploy/gateway/v0.4-sample.yaml \
  deploy/gateway/v0.5-gateway-config-sample.yaml \
  deploy/memory-extproc/deployment.yaml \
  deploy/redis/values.yaml
```

## 10. 운영 범위 밖 항목

아래 항목은 현재 PoC 범위 밖입니다.
운영 도입 전에 별도 Seed 또는 별도 프로젝트로 다뤄야 합니다.

- [ ] Streaming/SSE 응답 메모리 처리
- [ ] Long-term Memory
- [ ] Semantic Memory와 vector search
- [ ] 멀티 테넌트 정책
- [ ] 서비스별 모델 사용량 과금 정책
- [ ] 운영 수준 인증/인가
- [ ] 자동 복구와 rollout 전략
- [ ] Redis 백업/복구 전략
- [ ] 개인정보 삭제 요청 처리

## 최종 Go/No-Go 기준

운영 환경 반영 전 최소 기준:

- [ ] 매니페스트가 실제 클러스터 CRD 스키마와 일치한다.
- [ ] Gateway, EnvoyExtensionPolicy, memory-extproc Service의 namespace 관계가 일치한다.
- [ ] Redis 보안, persistence, HA 정책이 결정되어 있다.
- [ ] 민감정보가 Secret으로 관리된다.
- [ ] 기본 대화 메모리 시나리오와 장애 시나리오가 모두 통과한다.
- [ ] 로그에 개인정보와 요청/응답 전문이 남지 않는다.
- [ ] body buffering으로 인한 성능 영향이 허용 범위 안이다.
- [ ] Streaming, Semantic Memory, 멀티 테넌트 등 범위 밖 항목을 운영 요구사항에서 제외하거나 별도 계획으로 분리했다.
