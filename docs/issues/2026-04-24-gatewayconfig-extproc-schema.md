# GatewayConfig spec.extProc 실제 스키마가 문서와 다름

## 상태

- 상태: resolved
- 발견일: 2026-04-24
- 해결일: 2026-04-24
- 관련 Seed: Seed 5a - v0.4 → v0.5 마이그레이션

## 배경

v0.5 GatewayConfig 샘플을 클러스터에 apply하는 과정에서 발견.
공식 문서 예시와 실제 CRD 스키마가 달랐음.

## 증상

```text
Error from server (BadRequest): GatewayConfig in version "v1alpha1" cannot be handled as a GatewayConfig:
strict decoding error: unknown field "spec.extProc.env"
```

## 원인

공식 문서와 README 예시가 `spec.extProc.env`, `spec.extProc.resources`로 표기하고 있으나,
실제 설치된 v0.5.0 CRD 스키마는 `spec.extProc.kubernetes.env`, `spec.extProc.kubernetes.resources`임.

`kubectl explain gatewayconfig.spec.extProc.kubernetes` 로 확인:
- `env`: 환경변수 목록
- `resources`: CPU/메모리 requests/limits
- `image`, `imageRepository`, `securityContext`, `volumeMounts`

## 해결

샘플 파일에서 필드 경로 수정:

```yaml
# 수정 전 (잘못된 스키마)
spec:
  extProc:
    env: [...]
    resources: {...}

# 수정 후 (실제 스키마)
spec:
  extProc:
    kubernetes:
      env: [...]
      resources: {...}
```

## 검증

```bash
kubectl apply -f - <<EOF
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
          value: "redis://..."
EOF
```

결과: `gatewayconfig.aigateway.envoyproxy.io/memory-enabled-config created` ✅

## 영향 및 수정 완료 파일

| 파일 | 수정 내용 | 상태 |
|------|-----------|------|
| `deploy/gateway/v0.5-gateway-config-sample.yaml` | `spec.extProc.env` → `spec.extProc.kubernetes.env` | ✅ 완료 |
| `docs/migration-v0.4-to-v0.5.md` | Step 2 예상 결과, Step 3 전체 예시 교체 + 주의 문구 추가 | ✅ 완료 |
| `README.md` | Step 2 GatewayConfig 예시 `env`/`resources` 필드 순서 정렬 | ✅ 완료 |

## 남은 리스크

- 공식 문서가 업데이트되지 않은 상태로 남아있을 수 있음
- 설치된 CRD 버전에 따라 스키마가 다를 수 있으므로 항상 `kubectl explain`으로 먼저 확인할 것

## 관련 파일

- `deploy/gateway/v0.5-gateway-config-sample.yaml`
- `docs/migration-v0.4-to-v0.5.md`
