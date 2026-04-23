package memory

import (
	"context"
	"errors"
	"time"

	"github.com/Yurip94/kakao-envoy-ai-gateway/internal/openai"
)

var ErrMissingSessionID = errors.New("missing session id")

type Store interface {
	Load(ctx context.Context, sessionID string, limit int) ([]openai.Message, error)
	Append(ctx context.Context, sessionID string, messages []openai.Message, ttl time.Duration, limit int) error
}
