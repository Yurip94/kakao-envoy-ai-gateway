# OpenRouter 경유 시 메모리 주입 미반영

## 상태

- 상태: open (재개 — mock backend는 항상 고정 응답을 반환하므로 body 반영 여부를 검증할 수 없었음. 실제 미해결 상태)
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

## 원인 (재평가 필요)

초기에는 `content-length` 불일치를 주원인으로 판단했지만, 2026-04-26 재검증에서 이 결론은 보류됐다.

- 같은 custom extproc 코드로 `HTTPRoute -> echo-backend` 경로에서는 body mutation이 실제 upstream에 반영된다.
- OpenRouter `AIGatewayRoute -> AIServiceBackend` 경로에서만 메모리 주입이 최종 upstream에 반영되지 않는 증상이 반복된다.

따라서 현 시점 우선 가설은 `content-length` 단독 이슈보다, AI Gateway 내장 처리 경로(내장 ext_proc/후속 body 재구성)와 custom extproc의 상호작용 문제다.

## 해결

해결 전 상태다.

`internal/extproc/processor.go`에서 request headers 단계에 `content-length` 제거 시도를 넣어둔 상태이지만, OpenRouter `AIGatewayRoute` 경로의 미반영 문제는 아직 남아 있다.

```go
// RequestHeaders 처리
return continueRequestHeadersRemoveContentLength(), context.WithValue(ctx, sessionIDContextKey{}, sessionID), nil

// 추가 함수
func continueRequestHeadersRemoveContentLength() *extprocv3.ProcessingResponse {
    return &extprocv3.ProcessingResponse{
        Response: &extprocv3.ProcessingResponse_RequestHeaders{
            RequestHeaders: &extprocv3.HeadersResponse{
                Response: &extprocv3.CommonResponse{
                    Status: extprocv3.CommonResponse_CONTINUE,
                    HeaderMutation: &extprocv3.HeaderMutation{
                        RemoveHeaders: []string{"content-length"},
                    },
                },
            },
        },
    }
}
```

## 검증 (2026-04-26)

mock backend 기준 2-turn E2E 테스트:

```
Turn 1: history_len=0, merged_msgs=1, mutated_body_len=110 → HTTP 200
Turn 2: history_len=2, merged_msgs=3, mutated_body_len=209 → HTTP 200
```

Redis 저장:

```text
{"role":"user","content":"내 이름은 홍길동이야"}
{"role":"assistant","content":"Nani?"}
{"role":"user","content":"내 이름이 뭐라고 했지?"}
{"role":"assistant","content":"May the Force be with you."}
```

이 결과는 "custom extproc body mutation 자체는 가능하다"를 보여주지만, OpenRouter AIGatewayRoute 경로 문제를 해결했다는 근거는 아니다.

최신 추적 내용은 아래 문서를 기준으로 본다.

- `docs/issues/2026-04-26-body-mutation-content-length-blocked.md`

## 관련 파일

- `internal/extproc/processor.go`
- `deploy/gateway/v0.5-openrouter-sample.yaml`
- `docs/openrouter-provider.md`
