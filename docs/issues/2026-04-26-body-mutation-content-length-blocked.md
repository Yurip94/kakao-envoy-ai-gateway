# OpenRouter AIGatewayRoute 경로에서 메모리 body mutation 미반영 (재검증)

## 상태

- 상태: open (원인 추적 지속, 우회 경로는 검증 완료)
- 발견일: 2026-04-26
- 관련 Seed: Seed 10 - OpenRouter 실제 Provider 연동

## 요약

초기에는 `content-length` 보호 헤더 때문에 body mutation이 막힌 것으로 판단했지만, 재검증 결과 **해당 가설만으로는 설명되지 않는다**.

- `memory-extproc`는 Turn 2에서 `history_len=2`, `merged_msgs=3`, `mutated_body_len=287` 로그를 남긴다.
- OpenRouter 응답은 여전히 이전 turn을 기억하지 못하고 `prompt_tokens`도 단일 질문 수준으로 낮다.
- 반면 동일 코드/동일 ExtProc로 `echo-backend` 경유 검증 시 upstream body에는 병합된 `messages`가 정상 반영된다.

즉, 문제는 "ExtProc body mutation 자체 불가"가 아니라 **OpenRouter의 AIGatewayRoute 처리 경로에서 최종 upstream body가 다시 원본 기준으로 구성될 가능성**이 더 높다.

## 재검증 결과 (2026-04-26)

### 1) `test-gateway` + `HTTPRoute -> echo-backend`

- 동일 `memory-extproc` 정책(`request.body=Buffered`, `response.body=Buffered`) 사용
- Turn 2에서 echo 응답의 `request.body.messages`에 이전 turn user 메시지가 포함됨
- `rejected_header_mutations`는 `0`

결론: custom ExtProc의 body mutation은 동작한다.

### 2) `openrouter-ai-gateway` + 임시 `HTTPRoute -> echo-backend` (`/echo-check`)

- 같은 게이트웨이(`openrouter-ai-gateway`)에 임시 HTTPRoute를 붙여 검증
- Turn 2 echo 응답에서도 병합된 `messages`가 확인됨
- 헤더는 `content-length` 대신 `transfer-encoding: chunked`로 전달됨

결론: OpenRouter 게이트웨이 자체에서도 custom ExtProc mutation은 적용된다.

### 3) `openrouter-ai-gateway` + `AIGatewayRoute -> AIServiceBackend(OpenRouter)`

- OpenRouter 실제 호출은 HTTP 200
- Redis에는 `user -> assistant -> user -> assistant` 순서로 저장됨
- 그러나 Turn 2 응답은 Turn 1 컨텍스트를 반영하지 못함

결론: 문제는 AIGatewayRoute/AIServiceBackend 경로에서 발생한다.

## 관찰 포인트

- config dump에 route-level로 아래 두 필터가 함께 활성화됨
  - `envoy.filters.http.ext_proc/aigateway`
  - `envoy.filters.http.ext_proc/envoyextensionpolicy/.../openrouter-memory-extproc-policy/...`
- 동일 dump의 다른 HTTP filter chain에서 `envoy.filters.http.header_mutation`이 `content-length`를 dynamic metadata 기반으로 다루는 설정이 확인됨
- OpenRouter 게이트웨이의 `http.http-10080.ext_proc.rejected_header_mutations` 값은 증가하지만, route cluster 단위 카운터는 `0`으로 확인되어 어떤 ext_proc가 거부를 유발하는지 분리 확인이 필요함

## 현재 판단

`content-length`는 실패를 유발할 수 있는 조건 중 하나지만, 이번 케이스의 단일 근본 원인으로 확정하기 어렵다.
현재는 **AI Gateway 내장 처리 경로가 custom body mutation 결과를 최종 upstream에 반영하지 못하게 만드는 구조적 순서/재구성 문제**를 우선 의심한다.

## 다음 단계

1. AIGatewayRoute 경로에서 최종 upstream body를 직접 캡처할 수 있는 OpenAI-compatible 테스트 backend를 붙여 body 반영 여부를 확정한다.
2. config dump 기준으로 downstream/upstream 필터 체인에서 `ext_proc/aigateway`와 custom extproc의 적용 순서, body 재구성 지점을 문서화한다.
3. 우회안 비교:
   - AIGatewayRoute 경로를 유지할지
   - Gateway 앞단 별도 프록시에서 메모리 주입할지
   - 또는 OpenRouter 검증 목적을 HTTPRoute 기반 경로로 분리할지

## 우회 경로 결과 (2026-04-26)

`HTTPRoute -> Backend(openrouter.ai) + URLRewrite + custom extproc` 경로에서 실제 OpenRouter 2-turn 메모리 동작을 확인했다.

- Turn 1 후 Turn 2 응답이 사용자 이름을 정확히 회상
- Turn 2 `prompt_tokens`가 증가해 히스토리 주입 반영 정황 확인
- Redis 저장도 `user -> assistant -> user -> assistant` 순서로 일치

관련 매니페스트:

- `deploy/gateway/v0.5-openrouter-direct-sample.yaml`

## 관련 파일

- `internal/extproc/processor.go`
- `deploy/gateway/v0.5-openrouter-sample.yaml`
- `docs/issues/2026-04-24-openrouter-memory-injection-not-reflected.md`
