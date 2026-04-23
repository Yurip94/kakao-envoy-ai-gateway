package main

import (
	"log"

	"github.com/Yurip94/kakao-envoy-ai-gateway/internal/extproc"
)

func main() {
	cfg := extproc.LoadConfigFromEnv()

	log.Printf(
		"memory-extproc starting listen_addr=%s redis_url=%s ttl_seconds=%d max_history_length=%d session_header=%s redis_failure_policy=%s missing_session_policy=%s",
		cfg.ListenAddr,
		cfg.RedisURL,
		int(cfg.MemoryTTL.Seconds()),
		cfg.MaxHistoryLength,
		cfg.SessionHeader,
		cfg.RedisFailurePolicy,
		cfg.MissingSessionPolicy,
	)

	log.Print("gRPC External Processor server is not implemented yet")
}
