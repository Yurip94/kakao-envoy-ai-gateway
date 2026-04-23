#!/usr/bin/env bash
set -euo pipefail

AIGW_VERSION="${AIGW_VERSION:-v0.4.0}"
EG_NAMESPACE="${EG_NAMESPACE:-envoy-gateway-system}"
GATEWAY_NAME="${GATEWAY_NAME:-envoy-ai-gateway-basic}"
GATEWAY_NAMESPACE="${GATEWAY_NAMESPACE:-default}"
PORT_FORWARD_PORT="${PORT_FORWARD_PORT:-8080}"

kubectl apply -f "https://raw.githubusercontent.com/envoyproxy/ai-gateway/${AIGW_VERSION}/examples/basic/basic.yaml"

for _ in {1..30}; do
  POD_COUNT="$(
    kubectl get pods -n "${EG_NAMESPACE}" \
      -l "gateway.envoyproxy.io/owning-gateway-name=${GATEWAY_NAME},gateway.envoyproxy.io/owning-gateway-namespace=${GATEWAY_NAMESPACE}" \
      --no-headers 2>/dev/null | wc -l | tr -d ' '
  )"
  if [[ "${POD_COUNT}" != "0" ]]; then
    break
  fi
  sleep 2
done

kubectl wait pods --timeout=3m \
  -l "gateway.envoyproxy.io/owning-gateway-name=${GATEWAY_NAME},gateway.envoyproxy.io/owning-gateway-namespace=${GATEWAY_NAMESPACE}" \
  -n "${EG_NAMESPACE}" \
  --for=condition=Ready

ENVOY_SERVICE="$(
  kubectl get svc -n "${EG_NAMESPACE}" \
    --selector="gateway.envoyproxy.io/owning-gateway-namespace=${GATEWAY_NAMESPACE},gateway.envoyproxy.io/owning-gateway-name=${GATEWAY_NAME}" \
    -o jsonpath='{.items[0].metadata.name}'
)"

if [[ -z "${ENVOY_SERVICE}" ]]; then
  echo "Failed to find Envoy service for gateway ${GATEWAY_NAME}" >&2
  exit 1
fi

kubectl port-forward -n "${EG_NAMESPACE}" "svc/${ENVOY_SERVICE}" "${PORT_FORWARD_PORT}:80" >/tmp/kakao-envoy-v04-port-forward.log 2>&1 &
PORT_FORWARD_PID="$!"
trap 'kill "${PORT_FORWARD_PID}" >/dev/null 2>&1 || true' EXIT

sleep 3

curl -fsS -H "Content-Type: application/json" \
  -d '{
        "model": "some-cool-self-hosted-model",
        "messages": [
          {
            "role": "system",
            "content": "Hi."
          }
        ]
      }' \
  "http://localhost:${PORT_FORWARD_PORT}/v1/chat/completions"

echo
