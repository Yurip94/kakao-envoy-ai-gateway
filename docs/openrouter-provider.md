# OpenRouter Provider 연동

이 문서는 Envoy AI Gateway v0.5 Memory PoC를 mock backend가 아니라 OpenRouter 실제 Provider로 연결하는 절차를 정리합니다.

## 목표

- Envoy AI Gateway v0.5를 OpenRouter의 OpenAI-compatible Chat Completions API와 연결합니다.
- 기존 Custom `memory-extproc`와 Redis를 그대로 사용해 `x-session-id` 기반 2-turn 대화 메모리를 검증합니다.
- API 키는 Kubernetes Secret으로만 주입하고 저장소에는 남기지 않습니다.

## 왜 OpenRouter인가

OpenRouter는 OpenAI 호환 형태의 Chat Completions API를 제공합니다.
따라서 현재 PoC가 다루는 `messages` 배열, `model`, `stream: false` 요청 구조를 크게 바꾸지 않고 실제 Provider 연결을 검증할 수 있습니다.

```text
Client
  -> Envoy AI Gateway v0.5
  -> memory-extproc
  -> Redis
  -> OpenRouter /api/v1/chat/completions
```

## 전제 조건

- Kind 클러스터가 준비되어 있어야 합니다.
- Envoy Gateway v1.6.x와 Envoy AI Gateway v0.5.0이 설치되어 있어야 합니다.
- Redis와 `memory-extproc`가 `default` namespace에 배포되어 있어야 합니다.
- 자세한 기본 재현 절차는 `docs/reproduce-v05-memory-poc.md`를 먼저 따릅니다.

## API 키 Secret 생성

실제 API 키는 절대 문서, 매니페스트, 로그, 커밋에 남기지 않습니다.

```bash
export OPENROUTER_API_KEY="sk-or-..."

kubectl create secret generic openrouter-api-key \
  -n default \
  --from-literal=apiKey="$OPENROUTER_API_KEY"
```

`BackendSecurityPolicy`는 `apiKey`라는 Secret key를 읽어 OpenRouter upstream 인증 헤더로 사용합니다.
Envoy AI Gateway는 기본 `Authorization` 헤더에 주입할 때 `Bearer ` prefix를 붙이므로, Secret에는 `Bearer `를 제외한 원본 OpenRouter API key만 넣습니다.

## 매니페스트 적용

```bash
kubectl apply -f deploy/gateway/v0.5-openrouter-sample.yaml
```

이 샘플은 현재 검증 클러스터의 GatewayClass인 `envoy-ai-gateway-basic`을 기준으로 합니다.
다른 설치 방식에서 GatewayClass 이름이 다르면 `deploy/gateway/v0.5-openrouter-sample.yaml`의 `spec.gatewayClassName`을 해당 이름으로 바꿉니다.

OpenRouter 인증서 검증에 사용할 CA ConfigMap을 준비합니다.
로컬 PoC에서는 OpenRouter 인증서 체인을 받아 `openrouter-ca` ConfigMap을 생성합니다.

```bash
openssl s_client -connect openrouter.ai:443 -servername openrouter.ai -showcerts </dev/null \
  | awk '/BEGIN CERTIFICATE/,/END CERTIFICATE/{print}' > /tmp/openrouter-chain.pem

kubectl create configmap openrouter-ca \
  -n default \
  --from-file=ca.crt=/tmp/openrouter-chain.pem \
  --dry-run=client \
  -o yaml | kubectl apply -f -
```

상태를 확인합니다.

```bash
kubectl get gateway openrouter-ai-gateway -n default
kubectl get backend openrouter-backend -n default
kubectl get aiservicebackend openrouter-ai-backend -n default
kubectl get backendsecuritypolicy openrouter-api-key-policy -n default
kubectl get envoyextensionpolicy openrouter-memory-extproc-policy -n default
```

## Gateway 포트 포워딩

Envoy Gateway가 생성한 Service 이름을 찾습니다.

```bash
OPENROUTER_GATEWAY_SERVICE="$(
  kubectl get svc -n envoy-gateway-system \
    --selector=gateway.envoyproxy.io/owning-gateway-namespace=default,gateway.envoyproxy.io/owning-gateway-name=openrouter-ai-gateway \
    -o jsonpath='{.items[0].metadata.name}'
)"

echo "$OPENROUTER_GATEWAY_SERVICE"
```

로컬 포트로 연결합니다.

```bash
kubectl port-forward -n envoy-gateway-system \
  "svc/${OPENROUTER_GATEWAY_SERVICE}" \
  18085:80
```

## 2-turn 메모리 검증

먼저 기존 세션 히스토리를 비웁니다.

```bash
kubectl exec -n default redis-master-0 -- \
  redis-cli DEL "memory:session:openrouter-demo:messages"
```

첫 번째 요청입니다.

```bash
curl -sS \
  -H "Content-Type: application/json" \
  -H "Host: openrouter.ai" \
  -H "x-session-id: openrouter-demo" \
  --data @examples/requests/openrouter-first-turn.json \
  http://localhost:18085/v1/chat/completions
```

두 번째 요청입니다.

```bash
curl -sS \
  -H "Content-Type: application/json" \
  -H "Host: openrouter.ai" \
  -H "x-session-id: openrouter-demo" \
  --data @examples/requests/openrouter-second-turn.json \
  http://localhost:18085/v1/chat/completions
```

Redis 저장 결과를 확인합니다.

```bash
kubectl exec -n default redis-master-0 -- \
  redis-cli LRANGE "memory:session:openrouter-demo:messages" 0 -1
```

기대 흐름은 다음 순서입니다.

```text
user      첫 번째 요청
assistant 첫 번째 응답
user      두 번째 요청
assistant 두 번째 응답
```

주의:

- Redis에 위 순서대로 저장되는 것과 "OpenRouter upstream에 병합 body가 최종 반영되는 것"은 다를 수 있습니다.
- 2026-04-26 기준으로 OpenRouter `AIGatewayRoute -> AIServiceBackend` 경로에서는 Turn 2가 Turn 1을 기억하지 못하는 미해결 이슈가 있습니다.
- 최신 상태는 `docs/issues/2026-04-26-body-mutation-content-length-blocked.md`를 참고합니다.

## 동작 가능한 우회 경로 (검증 완료)

아래 우회 경로는 2026-04-26 기준으로 실제 OpenRouter 2-turn 메모리 동작을 확인했습니다.

```text
HTTPRoute -> Backend(openrouter.ai) + URLRewrite(/v1 -> /api/v1)
  + EnvoyExtensionPolicy(custom memory-extproc)
```

적용:

```bash
kubectl apply -f deploy/gateway/v0.5-openrouter-direct-sample.yaml
```

이 경로에서는 API 키를 클라이언트 요청 헤더로 직접 전달합니다.

```bash
export OPENROUTER_API_KEY="sk-or-..."
```

```bash
OPENROUTER_DIRECT_GATEWAY_SERVICE="$(
  kubectl get svc -n envoy-gateway-system \
    --selector=gateway.envoyproxy.io/owning-gateway-namespace=default,gateway.envoyproxy.io/owning-gateway-name=openrouter-direct-gateway \
    -o jsonpath='{.items[0].metadata.name}'
)"

kubectl port-forward -n envoy-gateway-system \
  "svc/${OPENROUTER_DIRECT_GATEWAY_SERVICE}" \
  18087:80
```

```bash
kubectl exec -n default redis-master-0 -- \
  redis-cli DEL "memory:session:openrouter-direct-demo:messages"

curl -sS \
  -H "Content-Type: application/json" \
  -H "Host: openrouter.ai" \
  -H "Authorization: Bearer ${OPENROUTER_API_KEY}" \
  -H "x-session-id: openrouter-direct-demo" \
  --data @examples/requests/openrouter-first-turn.json \
  http://localhost:18087/v1/chat/completions

curl -sS \
  -H "Content-Type: application/json" \
  -H "Host: openrouter.ai" \
  -H "Authorization: Bearer ${OPENROUTER_API_KEY}" \
  -H "x-session-id: openrouter-direct-demo" \
  --data @examples/requests/openrouter-second-turn.json \
  http://localhost:18087/v1/chat/completions
```

검증 기준:

- Turn 2 응답이 Turn 1의 사용자 이름을 맞히면 성공
- OpenRouter 응답의 `usage.prompt_tokens`가 Turn 1 대비 증가하면 성공
- Redis에는 `user -> assistant -> user -> assistant` 순서로 저장되어야 함

## 문제 해결

### 401 또는 인증 실패

- `openrouter-api-key` Secret이 `default` namespace에 있는지 확인합니다.
- Secret key 이름이 반드시 `apiKey`인지 확인합니다.
- Secret 값에 `Bearer ` prefix를 직접 넣지 않았는지 확인합니다.
- OpenRouter API 키가 유효한지 확인합니다.

### 404 또는 경로 오류

- 클라이언트 요청은 `/v1/chat/completions`로 보냅니다.
- `AIServiceBackend.spec.schema.prefix`는 OpenRouter upstream 경로인 `/api/v1`로 둡니다.

### 400 The plain HTTP request was sent to HTTPS port

- OpenRouter는 443 포트에서 HTTPS를 기대합니다.
- `Backend.spec.tls.caCertificateRefs`가 `openrouter-ca` ConfigMap을 참조하는지 확인합니다.
- `openrouter-ca` ConfigMap이 있고 `ca.crt` key가 있는지 확인합니다.
- 포트포워딩으로 호출할 때는 `Host: openrouter.ai` 헤더를 포함합니다.

### 503 CERTIFICATE_VERIFY_FAILED

- 로컬 Kind PoC에서 Envoy data plane의 기본 system CA trust path로 검증이 실패할 수 있습니다.
- 이 경우 위 절차처럼 OpenRouter 인증서 체인을 `openrouter-ca` ConfigMap으로 주입합니다.

### 모델 오류

- 예제는 `openai/gpt-4o-mini`를 사용합니다.
- 계정 권한, 라우팅, 크레딧 상태에 따라 사용 가능한 모델이 달라질 수 있으므로 필요하면 OpenRouter 모델 ID를 교체합니다.

### 메모리가 저장되지 않음

- `x-session-id` 헤더가 요청에 포함되어 있는지 확인합니다.
- `memory-extproc` 로그에서 request body, response body 처리 오류를 확인합니다.
- 현재 PoC는 `stream: false`를 기준으로 검증합니다. Streaming/SSE 응답 저장은 후속 Seed에서 별도로 설계합니다.

### Redis에는 저장되는데 OpenRouter가 이전 turn을 기억하지 못함

- 현재 알려진 미해결 케이스입니다.
- `memory-extproc` 로그에 `merged_msgs`가 증가해도 OpenRouter 응답이 이전 turn을 반영하지 않을 수 있습니다.
- 이 경우 `docs/issues/2026-04-26-body-mutation-content-length-blocked.md`의 재검증 절차를 따라, `echo-backend` 경유 검증과 OpenRouter 경유 검증을 분리해 확인합니다.
- 우선 기능 검증이 급하면 `deploy/gateway/v0.5-openrouter-direct-sample.yaml` 우회 경로를 사용합니다.

## 참고 자료

- OpenRouter API Reference: https://openrouter.ai/docs/api/reference/overview/
- OpenRouter Chat Completion: https://openrouter.ai/docs/api/api-reference/chat/send-chat-completion-request
- Envoy AI Gateway API Reference: https://aigateway.envoyproxy.io/docs/api/
