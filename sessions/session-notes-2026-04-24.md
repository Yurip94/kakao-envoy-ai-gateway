# 세션 기록 - 2026-04-24

## 목적

이번 세션은 Envoy AI Gateway v0.4 → v0.5 마이그레이션과 LLM 대화 메모리 PoC 구현을 완료하는 것을 목표로 진행했다.

## 진행한 Seed 요약

### Seed 1: ExtProc gRPC 서버 구현

Codex 위임 후 Claude가 직접 적용·검증.

- `internal/extproc/processor.go` — gRPC 스트리밍 핸들러 구현
  - request_headers: x-session-id 추출
  - request_body: Redis 히스토리 조회 → messages 병합 → body mutation
  - response_body: assistant 메시지 추출 → Redis 저장
  - fail-open, pass-through 정책 구현
- `cmd/memory-extproc/main.go` — gRPC 서버 기동 코드
- `go.mod` — `go-control-plane`, `grpc` 의존성 추가 (실제 요구 버전: grpc v1.67.1)

결과: `go build ./...`, `go test ./...` 전부 통과

### Seed 2: 로컬 통합 환경

- `Dockerfile` — multi-stage 빌드 (golang:1.22-alpine → alpine:3.20)
- `docker-compose.yml` — Redis + memory-extproc

### Seed 3: v0.5 ExtProc 연결 방식 확정

공식 문서 조사로 핵심 구조 확정:

| CRD | 역할 |
|-----|------|
| `GatewayConfig.spec.extProc.kubernetes` | AI Gateway 내장 ExtProc 환경변수·리소스 설정 |
| `EnvoyExtensionPolicy` | 커스텀 gRPC ExtProc 서비스를 Envoy에 연결 |

- `deploy/gateway/v0.5-gateway-config-sample.yaml` — `EnvoyExtensionPolicy` 추가
- `deploy/memory-extproc/deployment.yaml` — K8s Deployment + Service 생성

### Seed 4: Kind 클러스터 + v0.4 baseline smoke

기존 `ai-gateway-v04` 클러스터 재사용 (이전 세션에서 생성).

- v0.4 basic 예제 apply → smoke test curl HTTP 200 ✅

### Seed 5a: v0.4 → v0.5 마이그레이션

```
Envoy Gateway:    v1.5.0 → v1.6.0
AI Gateway CRD:   v0.4.0 → v0.5.0
AI Gateway ctrl:  v0.4.0 → v0.5.0
```

**발견 이슈**: GatewayConfig 실제 CRD 스키마가 문서와 다름
- 문서: `spec.extProc.env`
- 실제: `spec.extProc.kubernetes.env`
- 이슈 파일: `docs/issues/2026-04-24-gatewayconfig-extproc-schema.md`
- 관련 파일 전부 수정 완료

v0.5 GatewayConfig apply → `gatewayconfig created` ✅
v0.5 smoke test → HTTP 200 ✅

### Seed 5b: Redis + memory-extproc 배포 + end-to-end 검증

1. Redis 배포: `bitnami/redis` helm, auth 없음, persistence 없음
2. memory-extproc 이미지 빌드 → Kind 로드 → Deployment apply
3. EnvoyExtensionPolicy apply (Policy Accepted)
4. end-to-end 세션 메모리 테스트

**발견 이슈**: body mutation 시 Content-Length 불일치 → 500 에러
- 원인: body 교체 후 Content-Length 헤더 미갱신
- 해결: `continueRequestBody()`에 `HeaderMutation`으로 `content-length` 업데이트
- 이슈 파일: `docs/issues/2026-04-24-content-length-mismatch.md`

**검증 결과**

| 시나리오 | 결과 |
|----------|------|
| Turn 1 → Redis에 user+assistant 저장 | ✅ |
| Turn 2 → 히스토리 병합 주입 후 요청 | ✅ HTTP 200 |
| 세션 격리 (session-001 vs session-002) | ✅ |
| Redis 히스토리 직접 확인 | ✅ |

### Seed 6: 배포 파일 정합성

- `deploy/memory-extproc/deployment.yaml` — namespace `ai-gateway-system` → `default` 수정, resources 추가
- `deploy/redis/values.yaml` — helm values 신규 생성
- `docs/architecture.md` — 미결 사항 → 확정된 결정으로 전환

## 확정된 주요 결정 사항

| 항목 | 결정 |
|------|------|
| 구현 방향 | Option A: Custom ExtProc |
| ExtProc 연결 CRD | `EnvoyExtensionPolicy` (GatewayConfig만으로는 불가) |
| GatewayConfig 실제 스키마 | `spec.extProc.kubernetes.env` (문서와 다름) |
| user 메시지 저장 시점 | request_body 단계 (mutation 시점) |
| streaming 대응 | 1차 PoC 제외, 후속 과제 |
| Redis 장애 정책 | fail-open |
| 세션 ID 누락 정책 | pass-through |

## 발견한 이슈 목록

| 파일 | 내용 |
|------|------|
| `docs/issues/2026-04-24-gatewayconfig-extproc-schema.md` | GatewayConfig 실제 스키마가 공식 문서와 다름 |
| `docs/issues/2026-04-24-content-length-mismatch.md` | body mutation 시 Content-Length 불일치 500 에러 |

## 완료된 후속 작업

### Seed 7: 장애 시나리오 테스트

- `internal/extproc/processor_test.go` 신규 작성
  - `x-session-id` 누락 시 pass-through
  - missing session fail-closed 정책
  - Redis load 실패 시 fail-open
  - Redis fail-closed 정책
  - 히스토리 병합 시 `MAX_HISTORY_LENGTH` 기준 trimming
  - response body에서 assistant 메시지 저장
  - request headers에서 `x-session-id` 추출
- `internal/memory/redis_store_test.go` 보강
  - TTL 만료 후 세션 히스토리 제거 확인
- 검증:
  - `go test ./...` 통과
  - `go build ./...` 통과

### Seed 8: 운영 체크리스트 작성

- `docs/ops-checklist.md` 신규 작성
- 운영 전 점검 항목 정리:
  - 배포 구조와 namespace 정합성
  - v0.5 CRD 스키마 확인
  - Memory ExtProc 배포/연결 확인
  - Redis auth, persistence, TTL, max history 정책
  - Secret, 세션 ID, 로그 민감정보 관리
  - Redis 장애, 세션 ID 누락, 잘못된 JSON 처리 정책
  - 관측성, 성능, 비용, body buffering 영향
  - 기본/장애 테스트 시나리오
  - streaming, semantic memory, multi-tenant 등 운영 범위 밖 항목
- README에서 운영 체크리스트 문서 링크 추가

## 남은 작업

- 커밋 단위 분리 및 PR 준비
- Streaming/SSE 응답 메모리 처리 별도 설계
- Long-term/Semantic Memory 후속 설계
- 운영 수준 인증/인가, 관측성, Redis HA 정책 구체화

## 후속 재검증

### v0.5 샘플 namespace 정합성 보정

마지막 세션 내용을 다시 읽고 현재 저장소/클러스터 상태를 확인하는 과정에서,
검증된 클러스터는 `default` namespace 기준인데 `deploy/gateway/v0.5-gateway-config-sample.yaml` 일부 예시는 `ai-gateway-system` 기준으로 남아 있는 것을 발견했다.

- 이슈 기록: `docs/issues/2026-04-24-v05-sample-namespace-drift.md`
- 수정 파일:
  - `deploy/gateway/v0.5-gateway-config-sample.yaml`
  - `docs/migration-v0.4-to-v0.5.md`
  - `README.md`
- 재검증:
  - `go test ./...` 통과
  - `go build ./...` 통과
  - YAML 파싱 통과
  - `kubectl apply --dry-run=server` 통과
  - 순차 end-to-end 요청 HTTP 200, Redis 저장 순서 `user -> assistant -> user -> assistant` 확인

### 프로젝트 목적 쉬운 설명 문서화

사용자가 “회사 AI 사용의 공용 정문과 정문 옆 메모장 비서” 비유가 프로젝트 이유를 잘 설명하는지 확인했다.
이에 따라 프로젝트의 큰 그림을 쉽게 공유하기 위한 문서를 추가했다.

- 추가 파일: `docs/project-purpose-simple.md`
- README에서 해당 문서를 참조하도록 링크 추가
- 핵심 설명:
  - Envoy AI Gateway는 회사 AI 사용의 공용 정문
  - Memory ExtProc는 정문 옆에서 사용자별 대화 히스토리를 챙기는 비서
  - Redis는 비서의 메모장
  - v0.5 전환 이유는 메모리 내장이 아니라 Custom ExtProc와 GatewayConfig 기반 확장 구조를 활용하기 위함

## 현재 클러스터 상태 (2026-04-24 기준)

- 클러스터: `ai-gateway-v04` (Kind, K8s v1.32)
- Envoy Gateway: v1.6.0
- AI Gateway: v0.5.0
- namespace: `default`
- 실행 중인 pod:
  - `redis-master-0`
  - `memory-extproc-*`
  - `envoy-ai-gateway-basic-testupstream-*`
  - `envoy-default-envoy-ai-gateway-basic-*`
