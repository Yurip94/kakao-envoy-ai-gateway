# Envoy AI Gateway v0.4 Baseline

이 폴더는 v0.4 baseline을 로컬 Kind 클러스터에서 실행하기 위한 보조 스크립트를 보관합니다.

## 파일

- `install.sh`: Envoy Gateway v1.5.0과 Envoy AI Gateway v0.4.0을 설치합니다.
- `smoke-test.sh`: 공식 v0.4 basic 예제를 적용하고 mock backend 응답을 확인합니다.

## 실행 순서

Kind 클러스터 생성:

```bash
kind create cluster --config deploy/kind/v0.4-cluster.yaml
```

v0.4 baseline 설치:

```bash
deploy/gateway/v0.4/install.sh
```

smoke test:

```bash
deploy/gateway/v0.4/smoke-test.sh
```

## 주의

- 이 baseline은 API key가 필요 없는 공식 mock backend 예제를 사용합니다.
- 실제 provider 연동은 v0.5 target과 Memory ExtProc 검증 이후 별도 Seed에서 다룹니다.
- 설치 중 공식 chart나 values 경로가 변경되어 실패하면 `docs/issues/`에 기록합니다.
