# v0.5 샘플 매니페스트와 검증된 클러스터 namespace 불일치

## 상태

- 상태: resolved
- 발견일: 2026-04-24
- 해결일: 2026-04-24
- 관련 Seed: 세션 재검증 - v0.5 매니페스트 정합성 확인

## 배경

이전 세션에서 Redis와 memory-extproc는 `default` namespace에 배포되어 end-to-end 검증이 완료되었다.
하지만 `deploy/gateway/v0.5-gateway-config-sample.yaml` 일부 리소스는 `ai-gateway-system` namespace를 기준으로 남아 있었다.

## 증상

현재 클러스터의 실제 검증 리소스:

```text
default/gatewayconfig memory-enabled-config
default/envoyextensionpolicy memory-extproc-policy
default/service memory-extproc
default/service redis-master
```

반면 샘플 매니페스트는 Gateway, GatewayConfig, EnvoyExtensionPolicy를 `ai-gateway-system`에 생성하도록 작성되어 있었다.
그대로 적용하면 `EnvoyExtensionPolicy`가 같은 namespace의 `memory-extproc` Service를 찾지 못할 수 있다.

## 원인

v0.5 마이그레이션 문서의 초기 예시는 컨트롤러 설치 namespace와 PoC 애플리케이션 리소스 namespace를 혼용했다.
AI Gateway 컨트롤러는 `ai-gateway-system`에 설치하지만, 이번 PoC 검증 리소스는 `default` namespace에서 구성되었다.

## 해결

- `deploy/gateway/v0.5-gateway-config-sample.yaml`의 PoC 리소스 namespace를 `default`로 통일
- Redis URL을 `redis://redis-master.default.svc.cluster.local:6379`로 정리
- README와 마이그레이션 문서의 GatewayConfig/Redis 예시를 `default` 기준으로 보정
- server dry-run, YAML 파싱, end-to-end 순차 요청 검증 재실행

## 검증

```bash
go test ./...
go build ./...
ruby -e 'require "yaml"; ARGV.each { |f| YAML.load_stream(File.read(f)); puts "ok #{f}" }' \
  deploy/gateway/v0.4-sample.yaml \
  deploy/gateway/v0.5-gateway-config-sample.yaml \
  deploy/memory-extproc/deployment.yaml \
  deploy/redis/values.yaml
kubectl apply --dry-run=server -f deploy/memory-extproc/deployment.yaml
kubectl apply --dry-run=server -f deploy/gateway/v0.5-gateway-config-sample.yaml
```

순차 end-to-end 요청 결과:

- Turn 1: HTTP 200
- Turn 2: HTTP 200
- Redis 저장 순서: `user -> assistant -> user -> assistant`

## 남은 리스크

- 공식 예제나 향후 운영 환경에서는 별도 namespace를 사용할 수 있으므로, 실제 배포 시 Gateway, GatewayConfig, EnvoyExtensionPolicy, Service의 namespace 관계를 다시 확인해야 한다.
