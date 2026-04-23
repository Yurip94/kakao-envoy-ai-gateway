package openai

import (
	"errors"
	"testing"
)

func TestParseChatRequest(t *testing.T) {
	t.Run("parses valid request", func(t *testing.T) {
		body := []byte(`{"messages":[{"role":"user","content":"hello"}]}`)

		req, err := ParseChatRequest(body)
		if err != nil {
			t.Fatalf("ParseChatRequest returned error: %v", err)
		}

		if len(req.Messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(req.Messages))
		}

		if req.Messages[0].Role != "user" {
			t.Fatalf("expected role user, got %q", req.Messages[0].Role)
		}

		if string(req.Messages[0].Content) != `"hello"` {
			t.Fatalf("expected content %q, got %q", `"hello"`, string(req.Messages[0].Content))
		}
	})

	t.Run("returns missing messages error when field does not exist", func(t *testing.T) {
		body := []byte(`{"model":"gpt-4o-mini"}`)

		_, err := ParseChatRequest(body)
		if !errors.Is(err, ErrMissingMessages) {
			t.Fatalf("expected ErrMissingMessages, got %v", err)
		}
	})

	t.Run("returns json error for invalid body", func(t *testing.T) {
		body := []byte(`{"messages":[`)

		_, err := ParseChatRequest(body)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestMergeMessages(t *testing.T) {
	history := []Message{
		{Role: "user", Content: []byte(`"h1"`)},
		{Role: "assistant", Content: []byte(`"h2"`)},
	}
	current := []Message{
		{Role: "user", Content: []byte(`"c1"`)},
		{Role: "assistant", Content: []byte(`"c2"`)},
	}

	t.Run("keeps history first and current next", func(t *testing.T) {
		got := MergeMessages(history, current, 10)
		want := []string{`"h1"`, `"h2"`, `"c1"`, `"c2"`}

		if len(got) != len(want) {
			t.Fatalf("expected %d messages, got %d", len(want), len(got))
		}

		for i := range want {
			if string(got[i].Content) != want[i] {
				t.Fatalf("unexpected content at %d: want %q, got %q", i, want[i], string(got[i].Content))
			}
		}
	})

	t.Run("trims from oldest when limit exceeded", func(t *testing.T) {
		got := MergeMessages(history, current, 3)
		want := []string{`"h2"`, `"c1"`, `"c2"`}

		if len(got) != len(want) {
			t.Fatalf("expected %d messages, got %d", len(want), len(got))
		}

		for i := range want {
			if string(got[i].Content) != want[i] {
				t.Fatalf("unexpected content at %d: want %q, got %q", i, want[i], string(got[i].Content))
			}
		}
	})

	t.Run("does not trim when limit is zero or negative", func(t *testing.T) {
		gotZero := MergeMessages(history, current, 0)
		gotNegative := MergeMessages(history, current, -1)
		wantLen := len(history) + len(current)

		if len(gotZero) != wantLen {
			t.Fatalf("expected %d messages for zero limit, got %d", wantLen, len(gotZero))
		}

		if len(gotNegative) != wantLen {
			t.Fatalf("expected %d messages for negative limit, got %d", wantLen, len(gotNegative))
		}
	})
}
