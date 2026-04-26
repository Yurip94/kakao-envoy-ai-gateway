# Body Mutation이 Upstream에 미반영 — content-length 보호 헤더 문제

## 상태

- 상태: open (Codex 이관)
- 발견일: 2026-04-26
- 관련 Seed: Seed 10 - OpenRouter 실제 Provider 연동

## 현상

memory-extproc 로그는 Turn 2에서 히스토리가 정상 주입됨을 보여준다.

```
Turn 2: session=openrouter-demo history_len=2, merged_msgs=3, mutated_body_len=287
```

그러나 OpenRouter 응답의 `usage.prompt_tokens=15`로, upstream에는 원본 body(히스토리 없음)가 전달되고 있다. Turn 2에서 "내 이름이 뭐라고 했지?"라는 단일 메시지만 전송된 셈이다.

## 근본 원인 확인

Envoy는 request body processing mode가 `BUFFERED`일 때 `content-length` 헤더를 보호(read-only)로 취급한다. ExtProc에서 `content-length`를 수정하거나 삭제하는 모든 시도가 거부된다.

Envoy stats 확인:

```
http.http-10080.ext_proc.rejected_header_mutations: 2  (openrouter gateway, 2 요청 기준)
http.http-10080.ext_proc.rejected_header_mutations: 26 (mock gateway, 52 streams 기준)
```

요청마다 정확히 1개씩 거부됨 → 우리 ExtProc의 `RemoveHeaders: ["content-length"]`가 거부되고 있음.

## 시도한 해결 방법 (모두 실패)

### 시도 1: body phase에서 content-length OVERWRITE (SetHeaders)

```go
common.HeaderMutation = &extprocv3.HeaderMutation{
    SetHeaders: []*corev3.HeaderValueOption{{
        Header: &corev3.HeaderValue{Key: "content-length", Value: fmt.Sprintf("%d", len(mutatedBody))},
    }},
}
```

결과: `rejected_header_mutations` 증가. Envoy가 HeaderMutation을 거부하면서 body mutation도 함께 버림 → upstream에 원본 body 전달 → HTTP 200이지만 히스토리 미주입.

### 시도 2: content-length 조작 없이 body만 CONTINUE_AND_REPLACE

content-length HeaderMutation을 제거하고 body mutation만 전송.

결과: HTTP 500 `mismatch_between_content_length_and_the_length_of_the_mutated_body`.

### 시도 3 (현재 코드): header phase에서 RemoveHeaders

RequestHeaders 처리 단계에서 세션 ID가 있을 때 `content-length`를 미리 제거 시도.

```go
// processor.go:continueRequestHeadersRemoveContentLength()
HeaderMutation: &extprocv3.HeaderMutation{
    RemoveHeaders: []string{"content-length"},
}
```

결과:
- `rejected_header_mutations` 여전히 발생 (BUFFERED 모드에서 content-length는 어떤 방식으로도 수정/삭제 불가)
- mock backend: HTTP 200 (testupstream은 body 내용 무관하게 고정 응답 반환하므로 검증 불가)
- OpenRouter: HTTP 200이지만 `prompt_tokens=15` → 원본 body 전달 중

**결론: 현재 코드도 실제로는 body mutation이 upstream에 미반영 상태임.**

## 현재 코드 상태

`internal/extproc/processor.go` 핵심 부분:

```go
// RequestHeaders 단계: 세션 ID 있으면 content-length 제거 시도 (거부되지만)
return continueRequestHeadersRemoveContentLength(), context.WithValue(ctx, sessionIDContextKey{}, sessionID), nil

// body 단계: CONTINUE_AND_REPLACE로 merged body 전송 (upstream에 미반영)
return continueRequestBody(mutatedBody), ctx, nil

func continueRequestBody(mutatedBody []byte) *extprocv3.ProcessingResponse {
    common := &extprocv3.CommonResponse{Status: extprocv3.CommonResponse_CONTINUE}
    if len(mutatedBody) > 0 {
        common.Status = extprocv3.CommonResponse_CONTINUE_AND_REPLACE
        common.BodyMutation = &extprocv3.BodyMutation{
            Mutation: &extprocv3.BodyMutation_Body{Body: mutatedBody},
        }
    }
    // HeaderMutation 없음
    return &extprocv3.ProcessingResponse{...}
}
```

## 환경 정보

- Envoy Gateway: v1.6.0
- AI Gateway: v0.5.0
- EnvoyExtensionPolicy processingMode: `request.body: Buffered`, `response.body: Buffered`
- 필터 체인 순서 (config_dump 확인):
  1. `envoy.filters.http.ext_proc/aigateway` (AI Gateway 내장, disabled: true → route-level 활성화)
  2. `envoy.filters.http.ext_proc/.../openrouter-memory-extproc-policy/extproc/0` (우리 ExtProc, disabled: true → route-level 활성화)
  3. `envoy.filters.http.router`
- 두 ExtProc 모두 route-level typedPerFilterConfig `{}` 로 활성화됨

## 다음 단계 후보 (Codex 이관)

### 후보 A: BUFFERED 대신 STREAMED 또는 NONE + 사이드카

`processingMode.request.body`를 `NONE`으로 설정하면 body buffering 없이 ExtProc가 동작한다. body mutation은 직접 할 수 없으므로, body mutation을 Envoy 외부(사이드카 프록시 등)에서 처리하는 구조로 변경해야 한다.

### 후보 B: Lua 필터 또는 WASM으로 body 교체

EnvoyPatchPolicy로 Lua 필터를 listener에 주입해서 Redis에서 히스토리를 읽어 body를 Lua 레벨에서 교체한다. Redis 접근이 동기 방식으로 가능한지 확인 필요.

### 후보 C: ExtProc가 body를 읽고, 별도 헤더로 hint 전달

memory-extproc는 body를 읽고 merged messages를 별도 헤더(예: `x-memory-injected-messages`)로 Envoy에 전달한다. 이후 별도 필터(Lua 또는 WASM)가 해당 헤더를 읽고 body를 교체한다. content-length는 그 필터에서 계산.

### 후보 D: Gateway 앞단에 별도 HTTP proxy 배치

Envoy ExtProc를 거치지 않고, Gateway 앞단에 별도 HTTP 프록시(Go 서버)를 두어 body를 직접 교체하고 올바른 content-length를 설정한 뒤 Envoy Gateway로 전달한다. 가장 단순하지만 구조가 복잡해진다.

### 후보 E: Envoy `allow_mode_override`와 응답 단계 활용

AI Gateway 내장 ExtProc가 `allow_mode_override: true`로 설정되어 있음. 이 옵션을 활용해 처리 순서나 방식을 변경할 수 있는지 확인.

## 검증에 사용할 echo-backend

실제 upstream 도달 body를 확인하기 위한 echo-backend가 클러스터에 배포되어 있다.

```bash
# echo-backend는 받은 request body를 그대로 JSON으로 반환
# pod: echo-backend-57f7478645-drggc (default namespace)
# service: echo-backend:80

# 직접 포트포워드로 확인 (bypass Gateway)
kubectl port-forward -n default pod/echo-backend-57f7478645-drggc 18090:8080
curl -X POST http://localhost:18090/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"test","messages":[{"role":"user","content":"hello"}]}'
# 응답의 request.body.messages를 확인하면 upstream 도달 body 검증 가능
```

echo-backend를 Gateway 경유로 라우팅하면 ExtProc 후 실제 upstream 도달 body를 직접 확인할 수 있다.

## 관련 파일

- `internal/extproc/processor.go`
- `deploy/gateway/v0.5-openrouter-sample.yaml`
- `docs/issues/2026-04-24-openrouter-memory-injection-not-reflected.md` (이전 이슈, resolved로 마킹됐으나 실제 미해결)
