# Upstage Provider 연동 (Direct)

이 문서는 Upstage API를 OpenRouter와 분리된 경로로 연결하는 방법을 설명합니다.
현재 저장소 기준 권장 방식은 `HTTPRoute -> Backend(api.upstage.ai)` direct 경로입니다.

## 목표

- OpenRouter 경로와 독립적으로 Upstage 경로를 별도 Gateway로 운영합니다.
- 기존 `memory-extproc` + Redis 메모리 주입 흐름을 그대로 사용합니다.
- API 키는 클라이언트 요청 헤더(`Authorization`)로 전달합니다.

## 적용 매니페스트

먼저 Upstage 인증서 체인을 `ConfigMap`으로 준비합니다.

```bash
openssl s_client -connect api.upstage.ai:443 -servername api.upstage.ai -showcerts </dev/null \
  | awk '/BEGIN CERTIFICATE/,/END CERTIFICATE/{print}' > /tmp/upstage-chain.pem

kubectl create configmap upstage-ca \
  -n default \
  --from-file=ca.crt=/tmp/upstage-chain.pem \
  --dry-run=client \
  -o yaml | kubectl apply -f -
```

그 다음 매니페스트를 적용합니다.

```bash
kubectl apply -f deploy/gateway/v0.5-upstage-direct-sample.yaml
```

상태 확인:

```bash
kubectl get gateway upstage-direct-gateway -n default
kubectl get httproute upstage-direct-route -n default
kubectl get envoyextensionpolicy upstage-direct-memory-extproc-policy -n default
kubectl get backend upstage-backend -n default
```

## 포트 포워딩

```bash
UPSTAGE_DIRECT_GATEWAY_SERVICE="$(
  kubectl get svc -n envoy-gateway-system \
    --selector=gateway.envoyproxy.io/owning-gateway-namespace=default,gateway.envoyproxy.io/owning-gateway-name=upstage-direct-gateway \
    -o jsonpath='{.items[0].metadata.name}'
)"

kubectl port-forward -n envoy-gateway-system \
  "svc/${UPSTAGE_DIRECT_GATEWAY_SERVICE}" \
  18088:80
```

## 2-turn 메모리 검증

```bash
export UPSTAGE_API_KEY="YOUR_UPSTAGE_API_KEY"

kubectl exec -n default redis-master-0 -- \
  redis-cli DEL "memory:session:upstage-demo:messages"
```

첫 번째 요청:

```bash
curl -sS \
  -H "Content-Type: application/json" \
  -H "Host: api.upstage.ai" \
  -H "Authorization: Bearer ${UPSTAGE_API_KEY}" \
  -H "x-session-id: upstage-demo" \
  --data @examples/requests/upstage-first-turn.json \
  http://localhost:18088/v1/chat/completions
```

두 번째 요청:

```bash
curl -sS \
  -H "Content-Type: application/json" \
  -H "Host: api.upstage.ai" \
  -H "Authorization: Bearer ${UPSTAGE_API_KEY}" \
  -H "x-session-id: upstage-demo" \
  --data @examples/requests/upstage-second-turn.json \
  http://localhost:18088/v1/chat/completions
```

Redis 저장 확인:

```bash
kubectl exec -n default redis-master-0 -- \
  redis-cli LRANGE "memory:session:upstage-demo:messages" 0 -1
```

## 모델 이름 주의사항

- 예제는 `solar-pro2`를 사용합니다.
- 계정/요금제/권한에 따라 사용 가능한 모델명이 다를 수 있습니다.
- `solar-pro2`가 거부되면 동일 요청에서 `model`을 `solar-pro3`로 바꿔 재시도합니다.

## 참고

- Upstage Console API 예시(공식): https://console.upstage.ai/api-keys?api=chat
- Upstage Solar Pro 2 소개: https://www.upstage.ai/news/solar-pro-2
