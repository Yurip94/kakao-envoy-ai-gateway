#!/usr/bin/env bash
set -euo pipefail

AIGW_VERSION="${AIGW_VERSION:-v0.4.0}"
EG_VERSION="${EG_VERSION:-v1.5.0}"
EG_NAMESPACE="${EG_NAMESPACE:-envoy-gateway-system}"
AIGW_NAMESPACE="${AIGW_NAMESPACE:-envoy-ai-gateway-system}"
EG_CONFIG_URL="${EG_CONFIG_URL:-https://raw.githubusercontent.com/envoyproxy/ai-gateway/${AIGW_VERSION}/manifests/envoy-gateway-values.yaml}"

echo "Installing Envoy Gateway ${EG_VERSION}"
helm upgrade -i eg oci://docker.io/envoyproxy/gateway-helm \
  --version "${EG_VERSION}" \
  --namespace "${EG_NAMESPACE}" \
  --create-namespace \
  -f "${EG_CONFIG_URL}"

kubectl wait --timeout=2m \
  -n "${EG_NAMESPACE}" \
  deployment/envoy-gateway \
  --for=condition=Available

echo "Installing Envoy AI Gateway CRDs ${AIGW_VERSION}"
helm upgrade -i aieg-crd oci://docker.io/envoyproxy/ai-gateway-crds-helm \
  --version "${AIGW_VERSION}" \
  --namespace "${AIGW_NAMESPACE}" \
  --create-namespace

echo "Installing Envoy AI Gateway controller ${AIGW_VERSION}"
helm upgrade -i aieg oci://docker.io/envoyproxy/ai-gateway-helm \
  --version "${AIGW_VERSION}" \
  --namespace "${AIGW_NAMESPACE}" \
  --create-namespace

kubectl wait --timeout=2m \
  -n "${AIGW_NAMESPACE}" \
  deployment/ai-gateway-controller \
  --for=condition=Available

kubectl get pods -n "${EG_NAMESPACE}"
kubectl get pods -n "${AIGW_NAMESPACE}"
kubectl get crd | grep aigateway
