# OpenRouter + Upstage 이원화 구성

이 문서는 OpenRouter와 Upstage를 동시에 붙여서 provider를 분리 운영하는 방법을 정리합니다.

## 운영 원칙

- OpenRouter 경로:
  `deploy/gateway/v0.5-openrouter-direct-sample.yaml`
- Upstage 경로:
  `deploy/gateway/v0.5-upstage-direct-sample.yaml`
- 두 경로 모두 같은 `memory-extproc` + Redis를 사용합니다.
- 세션 충돌 방지를 위해 provider별 `x-session-id` prefix를 분리합니다.
  - 예: `or-<id>`, `up-<id>`

## 적용 순서

```bash
openssl s_client -connect openrouter.ai:443 -servername openrouter.ai -showcerts </dev/null \
  | awk '/BEGIN CERTIFICATE/,/END CERTIFICATE/{print}' > /tmp/openrouter-chain.pem
kubectl create configmap openrouter-ca -n default --from-file=ca.crt=/tmp/openrouter-chain.pem --dry-run=client -o yaml | kubectl apply -f -

openssl s_client -connect api.upstage.ai:443 -servername api.upstage.ai -showcerts </dev/null \
  | awk '/BEGIN CERTIFICATE/,/END CERTIFICATE/{print}' > /tmp/upstage-chain.pem
kubectl create configmap upstage-ca -n default --from-file=ca.crt=/tmp/upstage-chain.pem --dry-run=client -o yaml | kubectl apply -f -

kubectl apply -f deploy/gateway/v0.5-openrouter-direct-sample.yaml
kubectl apply -f deploy/gateway/v0.5-upstage-direct-sample.yaml
```

## 포트 포워딩

```bash
OPENROUTER_DIRECT_GATEWAY_SERVICE="$(
  kubectl get svc -n envoy-gateway-system \
    --selector=gateway.envoyproxy.io/owning-gateway-namespace=default,gateway.envoyproxy.io/owning-gateway-name=openrouter-direct-gateway \
    -o jsonpath='{.items[0].metadata.name}'
)"

UPSTAGE_DIRECT_GATEWAY_SERVICE="$(
  kubectl get svc -n envoy-gateway-system \
    --selector=gateway.envoyproxy.io/owning-gateway-namespace=default,gateway.envoyproxy.io/owning-gateway-name=upstage-direct-gateway \
    -o jsonpath='{.items[0].metadata.name}'
)"

kubectl port-forward -n envoy-gateway-system "svc/${OPENROUTER_DIRECT_GATEWAY_SERVICE}" 18087:80
kubectl port-forward -n envoy-gateway-system "svc/${UPSTAGE_DIRECT_GATEWAY_SERVICE}" 18088:80
```

## 요청 엔드포인트

- OpenRouter: `http://localhost:18087/v1/chat/completions`
- Upstage: `http://localhost:18088/v1/chat/completions`

## 키 전달 방식

- OpenRouter: `Authorization: Bearer ${OPENROUTER_API_KEY}`
- Upstage: `Authorization: Bearer ${UPSTAGE_API_KEY}`

## 검증 포인트

1. 같은 provider 내 동일 `x-session-id` 2-turn에서 기억이 유지되는지 확인
2. 서로 다른 provider 간 session이 섞이지 않는지 확인
3. Redis key(`memory:session:<id>:messages`)가 의도한 provider prefix로 저장되는지 확인

## 관련 문서

- `docs/openrouter-provider.md`
- `docs/upstage-provider.md`
