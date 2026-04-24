# Envoy AI Gateway v0.4 to v0.5 Migration

이 문서는 Envoy AI Gateway v0.4 baseline을 새로 정의한 뒤 v0.5 target으로 전환하는 greenfield migration PoC 가이드입니다.
이 프로젝트는 기존 운영 매니페스트를 받아서 변환하는 brownfield migration이 아니라, v0.4 출발점을 직접 만들고 v0.5로 업그레이드하는 과정을 산출물로 남기는 것을 목표로 합니다.

## 기준

- Envoy AI Gateway: v0.5.0
- Envoy Gateway: v1.6.x
- Envoy Proxy: v1.36.4
- Gateway API: v1.4.0
- Kubernetes: v1.32+
- 기준 확인일: 2026-04-23

## 공식 v0.5 변경사항 요약

Envoy AI Gateway v0.5에서 이 프로젝트와 직접 관련 있는 변경사항은 다음입니다.

| 항목 | v0.4 방식 | v0.5 방식 |
|------|-----------|-----------|
| ExtProc 리소스 설정 | `filterConfig.externalProcessor.resources` | `GatewayConfig.spec.extProc.resources` |
| ExtProc 환경변수 설정 | controller/global 또는 route 인접 설정 | `GatewayConfig.spec.extProc.env` |
| Gateway별 ExtProc 설정 | 제한적 | Gateway annotation으로 `GatewayConfig` 참조 |
| OpenAI 호환 prefix | `schema.version`에 prefix 값을 넣는 방식 | `schema.prefix` |
| Body Mutation | 제한적/신규 기능 전환 전 | backend 또는 route의 `bodyMutation` |

주의: 공식 v0.5 문서의 `GatewayConfig` 예시는 `apiVersion: aigateway.envoyproxy.io/v1alpha1`입니다.
실제 매니페스트는 설치된 CRD 버전을 확인한 뒤 맞춰야 합니다.

## 마이그레이션 목표

1. v0.4 baseline 매니페스트를 먼저 정의합니다.
2. 같은 Gateway/API routing 목적을 유지하면서 v0.5 target 매니페스트로 전환합니다.
3. deprecated 설정을 새 v0.5 필드로 옮깁니다.
4. Memory ExtProc PoC에서 필요한 환경변수를 `GatewayConfig`에 둡니다.
5. Gateway는 annotation으로 `GatewayConfig`를 참조합니다.
6. OpenAI 호환 provider endpoint prefix는 `schema.prefix`를 사용합니다.
7. `schema.version`과 `filterConfig.externalProcessor.resources`는 v0.5 target에 남기지 않습니다.

## v0.4 Baseline

Baseline 파일:

- `deploy/gateway/v0.4-sample.yaml`

실행 계획:

- `docs/v0.4-baseline-plan.md`

이 파일은 우리가 처음부터 만드는 v0.4 출발점입니다.
v0.5에서 deprecated 되는 설정을 의도적으로 포함해, 이후 target과 비교할 수 있게 합니다.

핵심 전환 대상:

```yaml
filterConfig:
  externalProcessor:
    resources:
      limits:
        memory: "512Mi"

schema:
  name: OpenAI
  version: "/v1beta/openai"
```

v0.5에서는 다음처럼 바꿉니다.

## v0.5 Target

Target 파일:

- `deploy/gateway/v0.5-gateway-config-sample.yaml`

이 파일은 v0.4 baseline과 같은 목적을 유지하면서 v0.5 방식으로 재작성한 목표 상태입니다.

핵심 변경:

```yaml
apiVersion: aigateway.envoyproxy.io/v1alpha1
kind: GatewayConfig
metadata:
  name: memory-enabled-config
  namespace: default
spec:
  extProc:
    kubernetes:
      env:
        - name: REDIS_URL
          value: "redis://redis-master.default.svc.cluster.local:6379"
      resources:
        requests:
          cpu: "100m"
          memory: "128Mi"
        limits:
          cpu: "500m"
          memory: "512Mi"
---
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: ai-gateway
  namespace: default
  annotations:
    aigateway.envoyproxy.io/gateway-config: memory-enabled-config
```

그리고 OpenAI 호환 prefix는 다음처럼 전환합니다.

```yaml
schema:
  name: OpenAI
  prefix: "/v1beta/openai"
```

## 단계별 변환 절차

### 1. v0.4 baseline 작성

먼저 v0.4 기준 출발점을 명시합니다.

```bash
kubectl apply --dry-run=client -f deploy/gateway/v0.4-sample.yaml
```

예상 결과:

- Gateway, Backend, AIServiceBackend, AIGatewayRoute 형태가 YAML로 유효해야 합니다.
- `schema.version`과 `filterConfig.externalProcessor.resources`가 baseline에 존재해야 합니다.

### 2. v0.5 CRD와 버전 확인

v0.5 target 검증 전에는 설치된 CRD 버전을 확인합니다.

```bash
kubectl get crd | grep aigateway
kubectl explain gatewayconfig.spec --api-version=aigateway.envoyproxy.io/v1alpha1
```

예상 결과:

- `GatewayConfig` CRD가 존재해야 합니다.
- `spec.extProc.kubernetes.env`와 `spec.extProc.kubernetes.resources`가 설명되어야 합니다.
- 확인 명령: `kubectl explain gatewayconfig.spec.extProc.kubernetes`

### 3. GatewayConfig 생성

기존 `filterConfig.externalProcessor.resources`에 있던 resource requests/limits를 `GatewayConfig.spec.extProc.kubernetes.resources`로 옮깁니다.

> ⚠️ **주의**: 공식 문서 예시와 달리 실제 v0.5.0 CRD 스키마는 `spec.extProc.kubernetes` 하위에 `env`와 `resources`가 있습니다.
> 적용 전 반드시 `kubectl explain gatewayconfig.spec.extProc.kubernetes`로 확인하세요.
> 관련 이슈: `docs/issues/2026-04-24-gatewayconfig-extproc-schema.md`

Memory ExtProc PoC에 필요한 값도 같은 곳에 둡니다.

```yaml
apiVersion: aigateway.envoyproxy.io/v1alpha1
kind: GatewayConfig
metadata:
  name: memory-enabled-config
  namespace: default
spec:
  extProc:
    kubernetes:
      env:
        - name: REDIS_URL
          value: "redis://redis-master.default.svc.cluster.local:6379"
        - name: MEMORY_TTL_SECONDS
          value: "3600"
        - name: MAX_HISTORY_LENGTH
          value: "20"
      resources:
        requests:
          cpu: "100m"
          memory: "128Mi"
        limits:
          cpu: "500m"
          memory: "512Mi"
```

### 4. Gateway에서 GatewayConfig 참조

Gateway metadata annotation에 GatewayConfig 이름을 지정합니다.

```yaml
metadata:
  annotations:
    aigateway.envoyproxy.io/gateway-config: memory-enabled-config
```

제약:

- `GatewayConfig`는 참조하는 `Gateway`와 같은 namespace에 있어야 합니다.

### 5. schema.version 제거

OpenAI 호환 endpoint prefix를 `schema.version`에 넣던 설정은 제거합니다.

Before:

```yaml
schema:
  name: OpenAI
  version: "/v1beta/openai"
```

After:

```yaml
schema:
  name: OpenAI
  prefix: "/v1beta/openai"
```

### 6. Body Mutation 위치 확인

v0.5에서는 request body의 top-level JSON 필드를 backend 또는 route 단위로 mutation할 수 있습니다.

제약:

- top-level JSON field만 지원합니다.
- nested path는 지원하지 않습니다.
- `set`, `remove` 항목은 각각 최대 16개입니다.

Memory PoC에서는 `messages` 배열 내부를 직접 부분 수정하지 않고, Custom ExtProc가 전체 `messages` 배열을 병합해 요청 본문을 재구성하는 방향을 사용합니다.

## 샘플 검증 명령

로컬에서는 먼저 YAML 문법을 확인합니다.

```bash
ruby -e 'require "yaml"; ARGV.each { |f| YAML.load_stream(File.read(f)); puts "ok #{f}" }' \
  deploy/gateway/v0.4-sample.yaml \
  deploy/gateway/v0.5-gateway-config-sample.yaml
```

Kubernetes 클러스터가 준비되어 있을 때는 v0.5 target을 server dry-run으로 검증합니다.

```bash
kubectl apply --dry-run=server -f deploy/gateway/v0.5-gateway-config-sample.yaml
```

예상 결과:

```text
gatewayconfig.aigateway.envoyproxy.io/memory-enabled-config configured (server dry run)
gateway.gateway.networking.k8s.io/ai-gateway configured (server dry run)
backend.gateway.envoyproxy.io/openai-compatible-backend configured (server dry run)
aiservicebackend.aigateway.envoyproxy.io/openai-compatible-ai-backend configured (server dry run)
aigatewayroute.aigateway.envoyproxy.io/openai-compatible-route configured (server dry run)
envoyextensionpolicy.gateway.envoyproxy.io/memory-extproc-policy configured (server dry run)
```

실제 CRD schema가 샘플과 다르면 설치된 Envoy AI Gateway v0.5 CRD를 우선합니다.

## GatewayConfig vs EnvoyExtensionPolicy 역할 구분

v0.5에서 ExtProc 관련 설정은 목적에 따라 두 CRD로 나뉩니다.

| CRD | 역할 |
|-----|------|
| `GatewayConfig.spec.extProc` | AI Gateway **내장** ExtProc 컨테이너의 환경변수·리소스 설정 |
| `EnvoyExtensionPolicy` | **커스텀** gRPC ExtProc 서비스를 Envoy에 연결 |

즉 `GatewayConfig`만으로는 우리 memory-extproc gRPC 서버를 Envoy에 연결할 수 없습니다.
`EnvoyExtensionPolicy`로 backendRefs에 memory-extproc Service를 지정해야 합니다.

```yaml
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: EnvoyExtensionPolicy
metadata:
  name: memory-extproc-policy
  namespace: default
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: ai-gateway
  extProc:
    - backendRefs:
        - name: memory-extproc
          port: 50051
      processingMode:
        request:
          body: Buffered
        response:
          body: Buffered
```

배포 순서:

1. `deploy/memory-extproc/deployment.yaml` — memory-extproc Deployment + Service
2. `deploy/gateway/v0.5-gateway-config-sample.yaml` — GatewayConfig, Gateway, Backend, AIServiceBackend, AIGatewayRoute, EnvoyExtensionPolicy

## Baseline에서 Target으로 비교할 항목

- v0.4 baseline에는 `filterConfig.externalProcessor.resources`가 있고 v0.5 target에는 없어야 합니다.
- v0.4 baseline에는 `schema.version`이 있고 v0.5 target에는 `schema.prefix`가 있어야 합니다.
- v0.5 target에는 `GatewayConfig`와 Gateway annotation이 있어야 합니다.
- Gateway namespace와 GatewayConfig namespace가 일치해야 합니다.
- v0.5 target에는 `EnvoyExtensionPolicy`로 memory-extproc Service를 연결합니다.
- Body Mutation은 provider 또는 route의 top-level JSON 필드 변경 용도로만 사용해야 합니다.

## 완료 기준

- v0.4 baseline과 v0.5 target 샘플이 함께 존재합니다.
- v0.4 baseline에는 전환 대상 deprecated 설정이 명시되어 있습니다.
- v0.5 target에는 `filterConfig.externalProcessor.resources`가 없습니다.
- v0.5 target에는 `schema.version`이 없습니다.
- Gateway가 `aigateway.envoyproxy.io/gateway-config` annotation을 사용합니다.
- `schema.prefix`가 OpenAI 호환 endpoint prefix를 표현합니다.
- `EnvoyExtensionPolicy`가 memory-extproc Service(port 50051)를 참조합니다.
- memory-extproc Deployment + Service 매니페스트가 `deploy/memory-extproc/`에 존재합니다.

## 참고 자료

- [Envoy AI Gateway v0.5 Release Notes](https://aigateway.envoyproxy.io/release-notes/v0.5/)
- [Gateway Configuration](https://aigateway.envoyproxy.io/docs/0.5/capabilities/gateway-config/)
- [Header and Body Mutations](https://aigateway.envoyproxy.io/docs/capabilities/traffic/header-body-mutations/)
- [Envoy Gateway External Processing](https://gateway.envoyproxy.io/docs/tasks/extensibility/ext-proc/)
