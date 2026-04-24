# OpenRouter 경유 시 메모리 주입 미반영

## 상태

- 상태: open
- 발견일: 2026-04-24
- 해결일: N/A
- 관련 Seed: Seed 10 - OpenRouter 실제 Provider 연동

## 배경

OpenRouter 실제 Provider 연동을 검증하면서 Gateway → OpenRouter 호출은 HTTP 200으로 성공했다.
또한 `memory-extproc`는 요청 user 메시지와 응답 assistant 메시지를 Redis에 저장했다.

## 증상

같은 `x-session-id`로 두 번 호출했을 때 Redis에는 대화가 아래 순서로 저장됐다.

```text
user
assistant
user
assistant
```

하지만 두 번째 OpenRouter 응답은 첫 번째 요청의 이름을 기억하지 못했다.
응답의 `usage.prompt_tokens`도 현재 요청만 처리된 것에 가까운 값으로 나타났다.

## 영향

- OpenRouter 실제 Provider 연결은 성공했지만, 실제 Provider 경유 end-to-end 메모리 검증은 아직 완료되지 않았다.
- Redis 저장 경로는 동작하지만, request body mutation이 OpenRouter upstream 요청에 반영되는지 추가 검증이 필요하다.

## 원인

현재 추정은 다음 중 하나다.

- `EnvoyExtensionPolicy`로 연결한 custom `memory-extproc`의 request body mutation이 built-in AI Gateway ExtProc 처리 순서상 upstream body에 최종 반영되지 않을 수 있다.
- custom ExtProc는 요청 body를 읽고 Redis 저장까지 수행하지만, 이후 built-in AI Gateway 처리 단계에서 원본 body 기반 변환이 다시 적용될 수 있다.
- OpenRouter 연결을 위해 `Host: openrouter.ai`와 외부 HTTPS backend 설정을 추가하면서, 기존 mock backend와 다른 처리 경로를 타고 있을 수 있다.

## 해결

아직 해결 전이다.

다음 단계에서 확인할 항목:

- Envoy config dump에서 built-in AI Gateway ExtProc와 custom memory ExtProc의 filter 순서 확인
- custom `memory-extproc`에 민감정보 없는 debug 로그를 임시 추가해 history length와 mutated message count 확인
- upstream에 실제 전달되는 body를 관찰할 수 있는 테스트 backend를 OpenRouter 앞단 대신 붙여 body mutation 적용 여부 확인
- 필요하면 memory injection 위치를 EnvoyExtensionPolicy 기반 custom ExtProc에서 AI Gateway bodyMutation 또는 별도 upstream proxy 방식으로 조정

## 검증

성공한 검증:

```bash
curl -H "Host: openrouter.ai" \
  -H "Content-Type: application/json" \
  -H "x-session-id: openrouter-demo" \
  --data @examples/requests/openrouter-first-turn.json \
  http://localhost:18085/v1/chat/completions
```

결과:

```text
HTTP_STATUS=200
```

Redis 저장 확인:

```bash
kubectl exec -n default redis-master-0 -- redis-cli LRANGE "memory:session:openrouter-demo:messages" 0 -1
```

결과:

```text
user -> assistant -> user -> assistant
```

남은 검증:

```text
두 번째 요청에서 첫 번째 대화 히스토리가 실제 OpenRouter upstream body에 포함되는지 확인
```

## 남은 리스크

- Custom ExtProc body mutation과 AI Gateway built-in ExtProc가 같은 요청 body를 다룰 때 순서 문제가 있을 수 있다.
- 실제 Provider 경유 메모리 검증은 mock backend 검증보다 더 많은 Envoy/AI Gateway filter chain 조건에 영향을 받는다.

## 관련 파일

- `internal/extproc/processor.go`
- `deploy/gateway/v0.5-openrouter-sample.yaml`
- `docs/openrouter-provider.md`
