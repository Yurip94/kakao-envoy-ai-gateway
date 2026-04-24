package extproc

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"

	"github.com/Yurip94/kakao-envoy-ai-gateway/internal/openai"
)

func TestProcessorMissingSessionIDPassThrough(t *testing.T) {
	store := &fakeStore{}
	processor := NewProcessor(testConfig(), store)

	resp, _, err := processor.handleRequestBody(context.Background(), &extprocv3.HttpBody{
		Body: []byte(`{"messages":[{"role":"user","content":"hello"}]}`),
	})
	if err != nil {
		t.Fatalf("handleRequestBody returned error: %v", err)
	}

	common := requestBodyCommon(t, resp)
	if common.GetStatus() != extprocv3.CommonResponse_CONTINUE {
		t.Fatalf("expected CONTINUE, got %s", common.GetStatus())
	}
	if common.GetBodyMutation() != nil {
		t.Fatalf("expected no body mutation, got %v", common.GetBodyMutation())
	}
	if store.loadCalls != 0 || store.appendCalls != 0 {
		t.Fatalf("expected no store calls, got load=%d append=%d", store.loadCalls, store.appendCalls)
	}
}

func TestProcessorMissingSessionIDCanFailClosed(t *testing.T) {
	cfg := testConfig()
	cfg.MissingSessionPolicy = "fail-closed"
	processor := NewProcessor(cfg, &fakeStore{})

	_, _, err := processor.handleRequestBody(context.Background(), &extprocv3.HttpBody{
		Body: []byte(`{"messages":[{"role":"user","content":"hello"}]}`),
	})
	if err == nil {
		t.Fatal("expected error for missing session id with fail-closed policy")
	}
}

func TestProcessorRedisLoadFailureFailOpen(t *testing.T) {
	store := &fakeStore{loadErr: errors.New("redis unavailable")}
	processor := NewProcessor(testConfig(), store)
	ctx := context.WithValue(context.Background(), sessionIDContextKey{}, "session-1")

	resp, _, err := processor.handleRequestBody(ctx, &extprocv3.HttpBody{
		Body: []byte(`{"model":"test","messages":[{"role":"user","content":"hello"}]}`),
	})
	if err != nil {
		t.Fatalf("handleRequestBody returned error: %v", err)
	}

	common := requestBodyCommon(t, resp)
	if common.GetStatus() != extprocv3.CommonResponse_CONTINUE_AND_REPLACE {
		t.Fatalf("expected CONTINUE_AND_REPLACE, got %s", common.GetStatus())
	}
	if got := mutatedMessages(t, common); len(got) != 1 || string(got[0].Content) != `"hello"` {
		t.Fatalf("expected current message only after load failure, got %#v", got)
	}
	if store.appendCalls != 1 {
		t.Fatalf("expected user message append, got %d calls", store.appendCalls)
	}
}

func TestProcessorRedisFailureCanFailClosed(t *testing.T) {
	cfg := testConfig()
	cfg.RedisFailurePolicy = "fail-closed"
	processor := NewProcessor(cfg, &fakeStore{loadErr: errors.New("redis unavailable")})
	ctx := context.WithValue(context.Background(), sessionIDContextKey{}, "session-1")

	_, _, err := processor.handleRequestBody(ctx, &extprocv3.HttpBody{
		Body: []byte(`{"messages":[{"role":"user","content":"hello"}]}`),
	})
	if err == nil {
		t.Fatal("expected redis load error with fail-closed policy")
	}
}

func TestProcessorRequestBodyMergesAndTrimsHistory(t *testing.T) {
	store := &fakeStore{
		history: []openai.Message{
			{Role: "user", Content: []byte(`"one"`)},
			{Role: "assistant", Content: []byte(`"two"`)},
			{Role: "user", Content: []byte(`"three"`)},
		},
	}
	cfg := testConfig()
	cfg.MaxHistoryLength = 3
	processor := NewProcessor(cfg, store)
	ctx := context.WithValue(context.Background(), sessionIDContextKey{}, "session-1")

	resp, _, err := processor.handleRequestBody(ctx, &extprocv3.HttpBody{
		Body: []byte(`{"model":"test","messages":[{"role":"user","content":"four"}]}`),
	})
	if err != nil {
		t.Fatalf("handleRequestBody returned error: %v", err)
	}

	got := mutatedMessages(t, requestBodyCommon(t, resp))
	if len(got) != 3 {
		t.Fatalf("expected 3 merged messages, got %d", len(got))
	}

	wantContents := []string{`"two"`, `"three"`, `"four"`}
	for i, want := range wantContents {
		if string(got[i].Content) != want {
			t.Fatalf("message %d content: want %s, got %s", i, want, string(got[i].Content))
		}
	}

	if len(store.appended) != 1 || string(store.appended[0].Content) != `"four"` {
		t.Fatalf("expected current user message appended, got %#v", store.appended)
	}
	if store.lastTTL != cfg.MemoryTTL {
		t.Fatalf("expected ttl %s, got %s", cfg.MemoryTTL, store.lastTTL)
	}
}

func TestProcessorResponseBodyStoresAssistantMessage(t *testing.T) {
	store := &fakeStore{}
	cfg := testConfig()
	processor := NewProcessor(cfg, store)
	ctx := context.WithValue(context.Background(), sessionIDContextKey{}, "session-1")

	resp, _, err := processor.handleResponseBody(ctx, &extprocv3.HttpBody{
		Body: []byte(`{"choices":[{"message":{"role":"assistant","content":"hi"}}]}`),
	})
	if err != nil {
		t.Fatalf("handleResponseBody returned error: %v", err)
	}
	if responseBodyCommon(t, resp).GetStatus() != extprocv3.CommonResponse_CONTINUE {
		t.Fatal("expected response body to continue")
	}
	if len(store.appended) != 1 {
		t.Fatalf("expected one assistant message appended, got %d", len(store.appended))
	}
	if store.appended[0].Role != "assistant" || string(store.appended[0].Content) != `"hi"` {
		t.Fatalf("unexpected assistant message: %#v", store.appended[0])
	}
	if store.lastTTL != cfg.MemoryTTL {
		t.Fatalf("expected ttl %s, got %s", cfg.MemoryTTL, store.lastTTL)
	}
}

func TestProcessorRequestHeadersExtractSessionID(t *testing.T) {
	processor := NewProcessor(testConfig(), &fakeStore{})

	resp, nextCtx, err := processor.handleRequest(context.Background(), &extprocv3.ProcessingRequest{
		Request: &extprocv3.ProcessingRequest_RequestHeaders{
			RequestHeaders: &extprocv3.HttpHeaders{
				Headers: &corev3.HeaderMap{
					Headers: []*corev3.HeaderValue{
						{Key: "X-Session-ID", Value: " session-1 "},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("handleRequest returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected continue response")
	}
	if got := sessionIDFromContext(nextCtx); got != "session-1" {
		t.Fatalf("expected session-1, got %q", got)
	}
}

type fakeStore struct {
	history     []openai.Message
	loadErr     error
	appendErr   error
	loadCalls   int
	appendCalls int
	appended    []openai.Message
	lastTTL     time.Duration
	lastLimit   int
}

func (s *fakeStore) Load(_ context.Context, _ string, _ int) ([]openai.Message, error) {
	s.loadCalls++
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	out := make([]openai.Message, len(s.history))
	copy(out, s.history)
	return out, nil
}

func (s *fakeStore) Append(_ context.Context, _ string, messages []openai.Message, ttl time.Duration, limit int) error {
	s.appendCalls++
	if s.appendErr != nil {
		return s.appendErr
	}
	s.appended = append(s.appended, messages...)
	s.lastTTL = ttl
	s.lastLimit = limit
	return nil
}

func testConfig() Config {
	return Config{
		ListenAddr:           ":50051",
		RedisURL:             "redis://localhost:6379",
		MemoryTTL:            time.Hour,
		MaxHistoryLength:     20,
		SessionHeader:        "x-session-id",
		RedisFailurePolicy:   "fail-open",
		MissingSessionPolicy: "pass-through",
	}
}

func requestBodyCommon(t *testing.T, resp *extprocv3.ProcessingResponse) *extprocv3.CommonResponse {
	t.Helper()

	bodyResp := resp.GetRequestBody()
	if bodyResp == nil || bodyResp.GetResponse() == nil {
		t.Fatalf("expected request body response, got %#v", resp)
	}
	return bodyResp.GetResponse()
}

func responseBodyCommon(t *testing.T, resp *extprocv3.ProcessingResponse) *extprocv3.CommonResponse {
	t.Helper()

	bodyResp := resp.GetResponseBody()
	if bodyResp == nil || bodyResp.GetResponse() == nil {
		t.Fatalf("expected response body response, got %#v", resp)
	}
	return bodyResp.GetResponse()
}

func mutatedMessages(t *testing.T, common *extprocv3.CommonResponse) []openai.Message {
	t.Helper()

	mutation := common.GetBodyMutation()
	if mutation == nil {
		t.Fatal("expected body mutation")
	}

	var payload struct {
		Messages []openai.Message `json:"messages"`
	}
	if err := json.Unmarshal(mutation.GetBody(), &payload); err != nil {
		t.Fatalf("failed to unmarshal mutated body: %v", err)
	}
	return payload.Messages
}
