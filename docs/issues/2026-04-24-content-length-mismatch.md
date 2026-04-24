# ExtProc body mutation 시 Content-Length 불일치 500 에러

## 상태

- 상태: resolved
- 발견일: 2026-04-24
- 해결일: 2026-04-24
- 관련 Seed: Seed 5b - memory-extproc 배포 + end-to-end 검증

## 배경

memory-extproc가 배포된 이후 첫 번째 turn은 200, 두 번째 turn(히스토리 주입)은 500이 발생.

## 증상

Envoy 액세스 로그:

```text
"response_code":500,
"response_code_details":"mismatch_between_content_length_and_the_length_of_the_mutated_body"
```

두 번째 요청부터 memory-extproc가 Redis 히스토리를 꺼내 messages 배열에 병합해 body 크기가 커지는데,
`Content-Length` 헤더는 원본 크기 그대로 남아 있어 Envoy가 불일치를 감지하고 500을 반환.

## 원인

`continueRequestBody()`에서 `CONTINUE_AND_REPLACE`로 body를 교체할 때
`HeaderMutation`으로 `Content-Length`를 함께 업데이트하지 않았음.

## 해결

`internal/extproc/processor.go`의 `continueRequestBody()`에
mutated body 길이를 `Content-Length`로 설정하는 `HeaderMutation` 추가:

```go
common.HeaderMutation = &extprocv3.HeaderMutation{
    SetHeaders: []*corev3.HeaderValueOption{
        {
            Header: &corev3.HeaderValue{
                Key:   "content-length",
                Value: fmt.Sprintf("%d", len(mutatedBody)),
            },
        },
    },
}
```

import에 `corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"` 추가.

## 검증

```bash
# Turn 1: 이름 전달
curl -H "x-session-id: test-session-001" \
  -d '{"model":"...","messages":[{"role":"user","content":"내 이름은 홍길동이야"}]}' \
  http://localhost:18084/v1/chat/completions
# → HTTP 200

# Turn 2: 히스토리 주입 후 요청
curl -H "x-session-id: test-session-001" \
  -d '{"model":"...","messages":[{"role":"user","content":"내 이름이 뭐라고 했지?"}]}' \
  http://localhost:18084/v1/chat/completions
# → HTTP 200

# Redis 확인
kubectl exec redis-master-0 -- redis-cli LRANGE "memory:session:test-session-001:messages" 0 -1
# → user/assistant 메시지 순서대로 저장 확인
```

## 남은 리스크

- ExtProc에서 body를 교체하는 경우 항상 Content-Length를 명시적으로 업데이트해야 함
- Streaming 응답(SSE)은 1차 PoC 범위 밖이므로 별도 검토 필요

## 관련 파일

- `internal/extproc/processor.go` — `continueRequestBody()` 수정
