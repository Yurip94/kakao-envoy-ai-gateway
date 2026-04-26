# v0.5 Memory PoC 재현 절차

이 문서는 `main` 기준으로 Envoy AI Gateway v0.5 + Custom Memory ExtProc + Redis 세션 메모리 PoC를 로컬 Kind 환경에서 다시 검증하는 절차입니다.

## 목적

재현 목표는 다음입니다.

- Envoy Gateway v1.6.x와 Envoy AI Gateway v0.5.0 설치
- Redis 배포
- `memory-extproc` 이미지 빌드 및 Kind 로드
- `EnvoyExtensionPolicy`로 Gateway와 memory-extproc 연결
- 같은 `x-session-id`에서 두 번째 요청에 이전 대화가 저장/조회되는지 확인
- Redis에 `user -> assistant -> user -> assistant` 순서로 메시지가 저장되는지 확인

## 전제

필요 도구:

- Docker Desktop
- kind
- kubectl
- helm
- Go 1.22+
- curl
- ruby 또는 YAML 파싱이 가능한 대체 도구

확인 명령:

```bash
docker version
kind version
kubectl version --client
helm version
go version
```

## 중요한 구분

이 저장소에는 두 가지 검증 경로가 있습니다.

| 경로 | 목적 | API key |
|------|------|---------|
| mock backend smoke | Gateway와 memory-extproc 연결, Redis 저장 흐름 검증 | 불필요 |
| 실제 provider target sample | v0.5 GatewayConfig/Backend/AIServiceBackend 샘플 검증 | provider 설정 필요 |

이 문서는 우선 **mock backend smoke** 기준으로 PoC를 재현합니다.
실제 provider 연동은 별도 Seed에서 다룹니다.

## 1. 클러스터 생성

기존 클러스터가 없다면 생성합니다.

```bash
kind create cluster --config deploy/kind/v0.4-cluster.yaml
```

이미 같은 이름의 클러스터가 있으면 현재 컨텍스트를 확인합니다.

```bash
kubectl config current-context
kubectl get nodes
```

예상:

```text
kind-ai-gateway-v04
```

## 2. Envoy Gateway v1.6.x 설치

```bash
helm upgrade -i eg oci://docker.io/envoyproxy/gateway-helm \
  --version v1.6.0 \
  -n envoy-gateway-system \
  --create-namespace \
  -f https://raw.githubusercontent.com/envoyproxy/ai-gateway/v0.5.0/manifests/envoy-gateway-values.yaml

kubectl wait --timeout=2m \
  -n envoy-gateway-system \
  deployment/envoy-gateway \
  --for=condition=Available
```

## 3. Envoy AI Gateway v0.5.0 설치

```bash
helm upgrade -i aieg-crd oci://docker.io/envoyproxy/ai-gateway-crds-helm \
  --version v0.5.0 \
  -n envoy-ai-gateway-system \
  --create-namespace

helm upgrade -i aieg oci://docker.io/envoyproxy/ai-gateway-helm \
  --version v0.5.0 \
  -n envoy-ai-gateway-system \
  --create-namespace

kubectl wait --timeout=2m \
  -n envoy-ai-gateway-system \
  deployment/ai-gateway-controller \
  --for=condition=Available
```

CRD 스키마를 확인합니다.

```bash
kubectl explain gatewayconfig.spec.extProc.kubernetes \
  --api-version=aigateway.envoyproxy.io/v1alpha1
```

`env`와 `resources`가 보이면 정상입니다.

## 4. v0.5 기본 smoke 리소스 적용

API key가 필요 없는 mock backend 예제를 적용합니다.

```bash
kubectl apply -f https://raw.githubusercontent.com/envoyproxy/ai-gateway/v0.5.0/examples/basic/basic.yaml
```

Gateway가 준비될 때까지 기다립니다.

```bash
kubectl wait pods --timeout=3m \
  -l gateway.envoyproxy.io/owning-gateway-name=envoy-ai-gateway-basic,gateway.envoyproxy.io/owning-gateway-namespace=default \
  -n envoy-gateway-system \
  --for=condition=Ready
```

Envoy Service 이름을 확인합니다.

```bash
kubectl get svc -n envoy-gateway-system \
  --selector=gateway.envoyproxy.io/owning-gateway-namespace=default,gateway.envoyproxy.io/owning-gateway-name=envoy-ai-gateway-basic
```

## 5. Redis 배포

PoC에서는 auth와 persistence를 끈 Redis를 사용합니다.
운영 환경에서는 그대로 사용하지 않습니다.

```bash
helm repo add bitnami https://charts.bitnami.com/bitnami
helm repo update

helm upgrade -i redis bitnami/redis \
  -n default \
  -f deploy/redis/values.yaml

kubectl wait --timeout=3m \
  -n default \
  pod/redis-master-0 \
  --for=condition=Ready
```

## 6. memory-extproc 이미지 빌드 및 로드

```bash
docker build -t memory-extproc:latest .
kind load docker-image memory-extproc:latest --name ai-gateway-v04
```

주의:

- 현재 Dockerfile은 PoC 기준 이미지입니다.
- 대상 Kind node와 이미지 아키텍처가 맞지 않아 Pod가 뜨지 않으면 `kubectl logs`에서 `exec format error` 여부를 확인합니다.

## 7. memory-extproc 배포

```bash
kubectl apply -f deploy/memory-extproc/deployment.yaml

kubectl wait --timeout=2m \
  -n default \
  deployment/memory-extproc \
  --for=condition=Available

kubectl logs -n default deploy/memory-extproc --tail=50
```

예상 로그:

```text
memory-extproc gRPC server listening on :50051
```

## 8. EnvoyExtensionPolicy 연결

mock backend Gateway 이름은 `envoy-ai-gateway-basic`입니다.
따라서 memory-extproc 연결 정책은 이 Gateway를 대상으로 적용합니다.

```bash
kubectl apply -f - <<'EOF'
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: EnvoyExtensionPolicy
metadata:
  name: memory-extproc-policy
  namespace: default
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: envoy-ai-gateway-basic
  extProc:
    - backendRefs:
        - name: memory-extproc
          port: 50051
      processingMode:
        request:
          body: Buffered
        response:
          body: Buffered
EOF
```

정책이 Accepted 되었는지 확인합니다.

```bash
kubectl describe envoyextensionpolicy -n default memory-extproc-policy
```

예상:

```text
Type: Accepted
Status: True
```

## 9. Port-forward

Envoy Service 이름을 변수로 잡습니다.

```bash
ENVOY_SERVICE="$(
  kubectl get svc -n envoy-gateway-system \
    --selector=gateway.envoyproxy.io/owning-gateway-namespace=default,gateway.envoyproxy.io/owning-gateway-name=envoy-ai-gateway-basic \
    -o jsonpath='{.items[0].metadata.name}'
)"

kubectl port-forward -n envoy-gateway-system "svc/${ENVOY_SERVICE}" 18084:80
```

다른 터미널에서 이후 curl을 실행합니다.

## 10. 대화 메모리 E2E 검증

테스트 세션을 초기화합니다.

```bash
kubectl exec -n default redis-master-0 -- \
  redis-cli DEL 'memory:session:demo-memory:messages'
```

첫 번째 요청:

```bash
curl -sS -H "Content-Type: application/json" \
  -H "x-session-id: demo-memory" \
  -d '{
        "model": "some-cool-self-hosted-model",
        "messages": [
          {
            "role": "user",
            "content": "내 이름은 홍길동이야"
          }
        ]
      }' \
  http://localhost:18084/v1/chat/completions
```

두 번째 요청:

```bash
curl -sS -H "Content-Type: application/json" \
  -H "x-session-id: demo-memory" \
  -d '{
        "model": "some-cool-self-hosted-model",
        "messages": [
          {
            "role": "user",
            "content": "내 이름이 뭐라고 했지?"
          }
        ]
      }' \
  http://localhost:18084/v1/chat/completions
```

Redis 저장 내용을 확인합니다.

```bash
kubectl exec -n default redis-master-0 -- \
  redis-cli LRANGE 'memory:session:demo-memory:messages' 0 -1
```

예상 구조:

```text
{"role":"user","content":"내 이름은 홍길동이야"}
{"role":"assistant","content":"..."}
{"role":"user","content":"내 이름이 뭐라고 했지?"}
{"role":"assistant","content":"..."}
```

mock backend 응답 내용은 고정 의미를 보장하지 않습니다.
중요한 검증 포인트는 Redis 저장 순서와 두 번째 요청에서 HTTP 200이 유지되는지입니다.

## 11. 세션 격리 확인

다른 session id로 요청합니다.

```bash
curl -sS -H "Content-Type: application/json" \
  -H "x-session-id: demo-memory-other" \
  -d '{
        "model": "some-cool-self-hosted-model",
        "messages": [
          {
            "role": "user",
            "content": "나는 다른 세션이야"
          }
        ]
      }' \
  http://localhost:18084/v1/chat/completions
```

Redis key가 분리되는지 확인합니다.

```bash
kubectl exec -n default redis-master-0 -- redis-cli KEYS 'memory:session:*'
```

`demo-memory`와 `demo-memory-other`가 별도 key로 보이면 정상입니다.

## 12. 로컬 검증 명령

코드와 매니페스트를 함께 확인합니다.

```bash
go test ./...
go build ./...
ruby -e 'require "yaml"; ARGV.each { |f| YAML.load_stream(File.read(f)); puts "ok #{f}" }' \
  deploy/gateway/v0.4-sample.yaml \
  deploy/gateway/v0.5-gateway-config-sample.yaml \
  deploy/memory-extproc/deployment.yaml \
  deploy/redis/values.yaml
```

## 13. 정리

Port-forward는 실행 중인 터미널에서 `Ctrl-C`로 종료합니다.

클러스터까지 삭제하려면 다음 명령을 사용합니다.

```bash
kind delete cluster --name ai-gateway-v04
```

## 문제 해결

### `unsupported path: /`

브라우저로 `http://localhost:18084`에 접근하면 `unsupported path: /`가 나올 수 있습니다.
이는 정상입니다.
Gateway는 `/v1/chat/completions` POST 요청을 테스트 대상으로 사용합니다.

### `mismatch_between_content_length_and_the_length_of_the_mutated_body`

body mutation 후 `content-length`가 갱신되지 않으면 발생합니다.
현재 구현은 RequestHeaders 단계에서 `content-length`를 제거하고 body를 교체해 전달합니다.
관련 기록은 `docs/issues/2026-04-24-content-length-mismatch.md`를 참고합니다.

### `GatewayConfig.spec.extProc.env` 오류

v0.5.0 실제 CRD에서는 `spec.extProc.kubernetes.env`를 사용합니다.
관련 기록은 `docs/issues/2026-04-24-gatewayconfig-extproc-schema.md`를 참고합니다.

### namespace 불일치

`EnvoyExtensionPolicy`, 대상 Gateway, `memory-extproc` Service의 namespace 관계를 확인합니다.
관련 기록은 `docs/issues/2026-04-24-v05-sample-namespace-drift.md`를 참고합니다.
