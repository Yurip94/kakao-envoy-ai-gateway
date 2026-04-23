# v0.4 smoke test pod wait 타이밍 실패

## 상태

- 상태: resolved
- 발견일: 2026-04-23
- 해결일: 2026-04-23
- 관련 Seed: Seed 11 - 실제 Kind 클러스터로 v0.4 baseline 검증

## 배경

v0.4 baseline 설치 후 `deploy/gateway/v0.4/smoke-test.sh`로 공식 basic 예제를 검증하던 중 pod 대기 단계에서 실패했다.

## 증상

실패 명령:

```bash
deploy/gateway/v0.4/smoke-test.sh
```

오류:

```text
error: no matching resources found
```

## 영향

- smoke test가 gateway 응답 확인 전에 중단됨
- baseline 검증 자동화 스크립트의 안정성이 떨어짐

## 원인

스크립트에서 `kubectl wait pods ... --for=condition=Ready`를 즉시 실행했다.
예제 적용 직후에는 selector에 맞는 Envoy proxy pod가 아직 생성되지 않아 `wait`가 실패했다.

## 해결

`deploy/gateway/v0.4/smoke-test.sh`를 수정했다.

- 먼저 selector에 맞는 pod가 1개 이상 생길 때까지 polling
- pod가 확인된 뒤 `kubectl wait`로 Ready 조건 대기
- selector에 `owning-gateway-name`뿐 아니라 `owning-gateway-namespace`도 함께 사용

## 검증

정적 검증:

```bash
bash -n deploy/gateway/v0.4/smoke-test.sh
```

결과:

```text
no output
```

실행 검증:

- 같은 클러스터에서 `deploy/gateway/v0.4/smoke-test.sh` 재실행
- pod 대기 단계를 통과하고 최종 curl 응답 확인

## 남은 리스크

- Gateway rollout이 매우 느린 환경에서는 timeout(기본 3분)을 더 늘려야 할 수 있다.
- selector 스키마가 Envoy Gateway 버전에 따라 바뀌면 재조정이 필요할 수 있다.

## 관련 파일

- `deploy/gateway/v0.4/smoke-test.sh`

## 참고 자료

- [Envoy AI Gateway v0.4 Basic Usage](https://aigateway.envoyproxy.io/docs/0.4/getting-started/basic-usage)
