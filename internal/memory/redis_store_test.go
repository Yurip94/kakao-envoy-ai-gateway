package memory

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Yurip94/kakao-envoy-ai-gateway/internal/openai"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisStoreAppendAndLoad(t *testing.T) {
	ctx := context.Background()
	store, server := newTestRedisStore(t)

	sessionID := "session-1"
	messages := []openai.Message{
		{Role: "user", Content: []byte(`"hello"`)},
		{Role: "assistant", Content: []byte(`"hi"`)},
	}

	if err := store.Append(ctx, sessionID, messages, time.Hour, 10); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	got, err := store.Load(ctx, sessionID, 10)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}

	if got[0].Role != "user" || string(got[0].Content) != `"hello"` {
		t.Fatalf("unexpected first message: role=%q content=%q", got[0].Role, string(got[0].Content))
	}

	if got[1].Role != "assistant" || string(got[1].Content) != `"hi"` {
		t.Fatalf("unexpected second message: role=%q content=%q", got[1].Role, string(got[1].Content))
	}

	if ttl := server.TTL(RedisSessionKey(sessionID)); ttl <= 0 {
		t.Fatalf("expected positive ttl, got %s", ttl)
	}
}

func TestRedisStoreAppendTrimsOldMessages(t *testing.T) {
	ctx := context.Background()
	store, _ := newTestRedisStore(t)

	sessionID := "session-trim"
	messages := []openai.Message{
		{Role: "user", Content: []byte(`"one"`)},
		{Role: "assistant", Content: []byte(`"two"`)},
		{Role: "user", Content: []byte(`"three"`)},
	}

	if err := store.Append(ctx, sessionID, messages, time.Hour, 2); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	got, err := store.Load(ctx, sessionID, 10)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}

	if string(got[0].Content) != `"two"` || string(got[1].Content) != `"three"` {
		t.Fatalf("expected newest messages, got %q and %q", string(got[0].Content), string(got[1].Content))
	}
}

func TestRedisStoreLoadAppliesLimit(t *testing.T) {
	ctx := context.Background()
	store, _ := newTestRedisStore(t)

	sessionID := "session-load-limit"
	messages := []openai.Message{
		{Role: "user", Content: []byte(`"one"`)},
		{Role: "assistant", Content: []byte(`"two"`)},
		{Role: "user", Content: []byte(`"three"`)},
	}

	if err := store.Append(ctx, sessionID, messages, time.Hour, 0); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	got, err := store.Load(ctx, sessionID, 2)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}

	if string(got[0].Content) != `"two"` || string(got[1].Content) != `"three"` {
		t.Fatalf("expected latest two messages, got %q and %q", string(got[0].Content), string(got[1].Content))
	}
}

func TestRedisStoreRejectsMissingSessionID(t *testing.T) {
	ctx := context.Background()
	store, _ := newTestRedisStore(t)

	_, loadErr := store.Load(ctx, " ", 10)
	if !errors.Is(loadErr, ErrMissingSessionID) {
		t.Fatalf("expected ErrMissingSessionID from Load, got %v", loadErr)
	}

	appendErr := store.Append(ctx, "", []openai.Message{{Role: "user", Content: []byte(`"hello"`)}}, time.Hour, 10)
	if !errors.Is(appendErr, ErrMissingSessionID) {
		t.Fatalf("expected ErrMissingSessionID from Append, got %v", appendErr)
	}
}

func TestRedisStoreAppendEmptyMessagesDoesNothing(t *testing.T) {
	ctx := context.Background()
	store, server := newTestRedisStore(t)

	sessionID := "session-empty"
	if err := store.Append(ctx, sessionID, nil, time.Hour, 10); err != nil {
		t.Fatalf("Append returned error: %v", err)
	}

	if server.Exists(RedisSessionKey(sessionID)) {
		t.Fatal("expected no key for empty append")
	}
}

func newTestRedisStore(t *testing.T) (*RedisStore, *miniredis.Miniredis) {
	t.Helper()

	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	return NewRedisStore(client), server
}
