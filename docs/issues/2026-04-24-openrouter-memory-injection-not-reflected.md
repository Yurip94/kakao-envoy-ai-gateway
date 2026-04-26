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

## 원인 (확정)

Envoy가 ExtProc `CONTINUE_AND_REPLACE` body mutation을 적용할 때, 원래 요청의 `content-length` 헤더와 교체된 body 크기가 맞지 않으면 두 가지 결과 중 하나가 발생한다.

1. ExtProc가 `content-length` HeaderMutation으로 직접 업데이트를 시도하면: Envoy가 해당 HeaderMutation을 거부(`rejected_header_mutations` 증가)하고, body mutation도 함께 무시된다. upstream에는 원본 body가 전달된다.
2. ExtProc가 body만 교체하고 `content-length`를 업데이트하지 않으면: Envoy가 `mismatch_between_content_length_and_the_length_of_the_mutated_body` 에러로 500을 반환한다.

## 해결

`internal/extproc/processor.go`의 RequestHeaders 처리 단계에서 세션 ID가 있을 때 `content-length` 헤더를 미리 제거한다. body mutation이 적용될 때 Envoy가 새 body 크기를 기준으로 `content-length`를 자동으로 계산하게 된다.

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

body mutation이 upstream에 실제 반영되는 것을 `mutated_body_len` 로그와 HTTP 200 응답으로 확인했다.

## 관련 파일

- `internal/extproc/processor.go`
- `deploy/gateway/v0.5-openrouter-sample.yaml`
- `docs/openrouter-provider.md`
