# OpenRouter Backend TLS 누락

## 상태

- 상태: resolved
- 발견일: 2026-04-24
- 해결일: 2026-04-24
- 관련 Seed: Seed 10 - OpenRouter 실제 Provider 연동

## 배경

OpenRouter 실제 Provider 연동을 위해 `deploy/gateway/v0.5-openrouter-sample.yaml`을 적용하고, 로컬 포트포워딩 후 `/v1/chat/completions`로 첫 요청을 보냈다.

## 증상

첫 실제 호출이 HTTP 400으로 실패했다.

```text
The plain HTTP request was sent to HTTPS port
```

## 영향

- OpenRouter upstream 연결이 실패했다.
- Redis와 memory-extproc 배포 상태와는 별개로, 외부 HTTPS Provider로 정상 라우팅할 수 없었다.

## 원인

`Backend`가 `openrouter.ai:443`을 가리키고 있었지만, Gateway가 backend로 TLS를 사용하도록 지정하는 `BackendTLSPolicy`가 없었다.
그 결과 OpenRouter의 HTTPS 포트에 plain HTTP 요청이 전달되었다.

## 해결

`deploy/gateway/v0.5-openrouter-sample.yaml`에 `Backend.spec.tls.caCertificateRefs`를 추가했다.

고정 외부 HTTPS Provider는 `Backend`의 FQDN endpoint로 두고, `Backend.spec.tls`에서 CA bundle을 지정한다.

```yaml
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: Backend
spec:
  endpoints:
    - fqdn:
        hostname: openrouter.ai
        port: 443
  tls:
    caCertificateRefs:
      - group: ""
        kind: ConfigMap
        name: openrouter-ca
```

## 검증

다음 검증을 수행한다.

```bash
kubectl create configmap openrouter-ca -n default --from-file=ca.crt=/tmp/openrouter-chain.pem --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f deploy/gateway/v0.5-openrouter-sample.yaml
curl -sS -H "Host: openrouter.ai" -H "Content-Type: application/json" -H "x-session-id: openrouter-demo" --data @examples/requests/openrouter-first-turn.json http://localhost:18085/v1/chat/completions
```

예상 결과:

```text
OpenRouter HTTPS port plain HTTP 오류가 발생하지 않아야 한다.
```

## 남은 리스크

- 로컬에서 생성한 `openrouter-ca`는 OpenRouter/Cloudflare 인증서 체인 변경 시 갱신해야 한다.
- 운영 또는 운영 유사 환경에서는 인증서 체인 고정 대신 신뢰 가능한 CA bundle 관리 방식을 별도로 정해야 한다.
- 실제 호출 성공 여부는 OpenRouter API key, 모델 권한, 계정 크레딧 상태에 영향을 받는다.

## 관련 파일

- `deploy/gateway/v0.5-openrouter-sample.yaml`
- `docs/openrouter-provider.md`
