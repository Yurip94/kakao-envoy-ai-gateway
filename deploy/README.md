# Deploy

Kubernetes 배포 예제는 이 폴더 아래에 분리해서 작성합니다.

- `kind/`: 로컬 Kind 클러스터 구성
- `gateway/`: Envoy Gateway와 Envoy AI Gateway 설정
- `redis/`: Redis 배포 설정

`gateway/` 주요 샘플:

- `v0.5-openrouter-sample.yaml`: AIGatewayRoute + AIServiceBackend 기반 OpenRouter 연동 샘플
- `v0.5-openrouter-direct-sample.yaml`: HTTPRoute + Backend 직결 우회 샘플 (메모리 주입 검증용)
- `v0.5-upstage-direct-sample.yaml`: HTTPRoute + Backend 직결 Upstage 연동 샘플
