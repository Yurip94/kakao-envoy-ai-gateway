# GatewayConfig apiVersion 문서 충돌

## 상태

- 상태: resolved
- 발견일: 2026-04-23
- 해결일: 2026-04-23
- 관련 Seed: Seed 4 - 샘플 기반 v0.4 to v0.5 마이그레이션 문서/매니페스트 작성

## 배경

Envoy AI Gateway v0.4 baseline을 새로 정의하고 v0.5 target으로 전환하는 greenfield migration PoC 문서를 작성하던 중 `GatewayConfig`의 `apiVersion` 예시가 문서마다 다르게 표시되는 것을 발견했다.

프로젝트 README 초안에는 `GatewayConfig` 예시가 `aigateway.envoyproxy.io/v1beta1`로 작성되어 있었다.
반면 Envoy AI Gateway v0.5 공식 문서와 v0.5 release notes의 예시는 `aigateway.envoyproxy.io/v1alpha1`를 사용한다.

## 증상

README의 v0.5 예시:

```yaml
apiVersion: aigateway.envoyproxy.io/v1beta1
kind: GatewayConfig
```

공식 v0.5 문서 기준 예시:

```yaml
apiVersion: aigateway.envoyproxy.io/v1alpha1
kind: GatewayConfig
```

## 영향

- v0.5 target 매니페스트가 실제 설치된 CRD와 맞지 않을 수 있다.
- README, migration 문서, deploy 샘플 사이의 정합성이 깨질 수 있다.
- 이후 `kubectl apply --dry-run=server` 검증 시 CRD version mismatch가 발생할 수 있다.

## 원인

확인된 원인은 README 초안이 공식 v0.5 문서의 최신 예시와 다른 `GatewayConfig` apiVersion을 사용하고 있었기 때문이다.

공식 v0.5 문서 기준:

- `GatewayConfig` 예시는 `apiVersion: aigateway.envoyproxy.io/v1alpha1`
- Gateway는 `aigateway.envoyproxy.io/gateway-config` annotation으로 같은 namespace의 GatewayConfig를 참조

## 해결

다음 파일을 공식 v0.5 문서 기준과 맞췄다.

- `README.md`
- `docs/migration-v0.4-to-v0.5.md`
- `deploy/gateway/v0.5-gateway-config-sample.yaml`

적용한 결정:

- v0.5 샘플 매니페스트와 README 예시는 `aigateway.envoyproxy.io/v1alpha1`를 사용한다.
- 실제 클러스터 검증 전에는 설치된 CRD 버전을 `kubectl explain`으로 다시 확인한다.
- 문서에는 설치된 CRD schema가 샘플과 다르면 클러스터의 CRD를 우선한다는 주의 문구를 남긴다.

## 검증

실행한 검증:

```bash
rg -n "v1beta1" README.md AGENTS.md docs/*.md deploy
```

결과:

```text
no matches
```

추가 검증:

```bash
git diff --check
```

결과:

```text
no output
```

## 남은 리스크

- 아직 Envoy AI Gateway v0.5가 설치된 Kubernetes 클러스터에서 server dry-run을 실행하지 못했다.
- 실제 설치된 CRD가 공식 문서 예시와 다를 가능성은 낮지만, 클러스터 검증 단계에서 다시 확인해야 한다.

## 관련 파일

- `README.md`
- `docs/migration-v0.4-to-v0.5.md`
- `deploy/gateway/v0.5-gateway-config-sample.yaml`

## 참고 자료

- [Envoy AI Gateway v0.5 Release Notes](https://aigateway.envoyproxy.io/release-notes/v0.5/)
- [Gateway Configuration](https://aigateway.envoyproxy.io/docs/0.5/capabilities/gateway-config/)
