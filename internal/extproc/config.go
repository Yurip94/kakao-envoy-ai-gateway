package extproc

import (
	"os"
	"strconv"
	"time"
)

const (
	defaultListenAddr           = ":50051"
	defaultRedisURL             = "redis://localhost:6379"
	defaultMemoryTTLSeconds     = 3600
	defaultMaxHistoryLength     = 20
	defaultSessionHeader        = "x-session-id"
	defaultRedisFailurePolicy   = "fail-open"
	defaultMissingSessionPolicy = "pass-through"
)

type Config struct {
	ListenAddr           string
	RedisURL             string
	MemoryTTL            time.Duration
	MaxHistoryLength     int
	SessionHeader        string
	RedisFailurePolicy   string
	MissingSessionPolicy string
}

func LoadConfigFromEnv() Config {
	return Config{
		ListenAddr:           envString("LISTEN_ADDR", defaultListenAddr),
		RedisURL:             envString("REDIS_URL", defaultRedisURL),
		MemoryTTL:            time.Duration(envInt("MEMORY_TTL_SECONDS", defaultMemoryTTLSeconds)) * time.Second,
		MaxHistoryLength:     envInt("MAX_HISTORY_LENGTH", defaultMaxHistoryLength),
		SessionHeader:        envString("SESSION_HEADER", defaultSessionHeader),
		RedisFailurePolicy:   envString("REDIS_FAILURE_POLICY", defaultRedisFailurePolicy),
		MissingSessionPolicy: envString("MISSING_SESSION_POLICY", defaultMissingSessionPolicy),
	}
}

func envString(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}

	return parsed
}
