package openai

import (
	"encoding/json"
	"errors"
)

var ErrAssistantMessageNotFound = errors.New("assistant message not found")

type ChatResponse struct {
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message Message `json:"message"`
}

func ExtractAssistantMessage(body []byte) (Message, error) {
	var resp ChatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return Message{}, err
	}

	for _, choice := range resp.Choices {
		if choice.Message.Role == "assistant" {
			return choice.Message, nil
		}
	}

	return Message{}, ErrAssistantMessageNotFound
}
