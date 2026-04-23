package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Yurip94/kakao-envoy-ai-gateway/internal/openai"
	"github.com/redis/go-redis/v9"
)

const redisSessionKeyFormat = "memory:session:%s:messages"

type RedisStore struct {
	client redis.Cmdable
}

func NewRedisStore(client redis.Cmdable) *RedisStore {
	return &RedisStore{client: client}
}

func RedisSessionKey(sessionID string) string {
	return fmt.Sprintf(redisSessionKeyFormat, sessionID)
}

func (s *RedisStore) Load(ctx context.Context, sessionID string, limit int) ([]openai.Message, error) {
	if strings.TrimSpace(sessionID) == "" {
		return nil, ErrMissingSessionID
	}

	key := RedisSessionKey(sessionID)
	start := int64(0)
	if limit > 0 {
		start = int64(-limit)
	}

	items, err := s.client.LRange(ctx, key, start, -1).Result()
	if err != nil {
		return nil, err
	}

	messages := make([]openai.Message, 0, len(items))
	for _, item := range items {
		var msg openai.Message
		if err := json.Unmarshal([]byte(item), &msg); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

func (s *RedisStore) Append(ctx context.Context, sessionID string, messages []openai.Message, ttl time.Duration, limit int) error {
	if strings.TrimSpace(sessionID) == "" {
		return ErrMissingSessionID
	}
	if len(messages) == 0 {
		return nil
	}

	key := RedisSessionKey(sessionID)
	values := make([]any, 0, len(messages))
	for _, msg := range messages {
		encoded, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		values = append(values, encoded)
	}

	if err := s.client.RPush(ctx, key, values...).Err(); err != nil {
		return err
	}

	if limit > 0 {
		if err := s.client.LTrim(ctx, key, int64(-limit), -1).Err(); err != nil {
			return err
		}
	}

	if ttl > 0 {
		return s.client.Expire(ctx, key, ttl).Err()
	}

	return nil
}
