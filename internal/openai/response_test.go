package openai

import (
	"errors"
	"testing"
)

func TestExtractAssistantMessage(t *testing.T) {
	t.Run("extracts assistant message", func(t *testing.T) {
		body := []byte(`{
			"choices":[
				{"message":{"role":"assistant","content":"answer"}}
			]
		}`)

		msg, err := ExtractAssistantMessage(body)
		if err != nil {
			t.Fatalf("ExtractAssistantMessage returned error: %v", err)
		}

		if msg.Role != "assistant" {
			t.Fatalf("expected role assistant, got %q", msg.Role)
		}

		if string(msg.Content) != `"answer"` {
			t.Fatalf("expected content %q, got %q", `"answer"`, string(msg.Content))
		}
	})

	t.Run("extracts first assistant message among mixed roles", func(t *testing.T) {
		body := []byte(`{
			"choices":[
				{"message":{"role":"user","content":"ignore"}},
				{"message":{"role":"assistant","content":"picked"}},
				{"message":{"role":"assistant","content":"later"}}
			]
		}`)

		msg, err := ExtractAssistantMessage(body)
		if err != nil {
			t.Fatalf("ExtractAssistantMessage returned error: %v", err)
		}

		if string(msg.Content) != `"picked"` {
			t.Fatalf("expected content %q, got %q", `"picked"`, string(msg.Content))
		}
	})

	t.Run("returns not found error when assistant message does not exist", func(t *testing.T) {
		body := []byte(`{
			"choices":[
				{"message":{"role":"user","content":"hello"}}
			]
		}`)

		_, err := ExtractAssistantMessage(body)
		if !errors.Is(err, ErrAssistantMessageNotFound) {
			t.Fatalf("expected ErrAssistantMessageNotFound, got %v", err)
		}
	})

	t.Run("returns json error for invalid body", func(t *testing.T) {
		body := []byte(`{"choices":[`)

		_, err := ExtractAssistantMessage(body)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
