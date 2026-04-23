package openai

import (
	"encoding/json"
	"errors"
)

var ErrMissingMessages = errors.New("missing messages array")

type Message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type ChatRequest struct {
	Messages []Message `json:"messages"`
}

func ParseChatRequest(body []byte) (ChatRequest, error) {
	var req ChatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return ChatRequest{}, err
	}
	if req.Messages == nil {
		return ChatRequest{}, ErrMissingMessages
	}
	return req, nil
}

func MergeMessages(history []Message, current []Message, limit int) []Message {
	merged := make([]Message, 0, len(history)+len(current))
	merged = append(merged, history...)
	merged = append(merged, current...)

	if limit <= 0 || len(merged) <= limit {
		return merged
	}

	return merged[len(merged)-limit:]
}
