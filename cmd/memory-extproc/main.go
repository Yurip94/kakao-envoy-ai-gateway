package main

import (
	"context"
	"log"
	"net"

	extprocv3 "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"

	"github.com/Yurip94/kakao-envoy-ai-gateway/internal/extproc"
	"github.com/Yurip94/kakao-envoy-ai-gateway/internal/memory"
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

	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatalf("failed to parse redis url: %v", err)
	}

	redisClient := redis.NewClient(redisOpts)
	defer redisClient.Close()

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("failed to connect redis: %v", err)
	}

	store := memory.NewRedisStore(redisClient)
	processor := extproc.NewProcessor(cfg, store)

	grpcServer := grpc.NewServer()
	extprocv3.RegisterExternalProcessorServer(grpcServer, processor)

	lis, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", cfg.ListenAddr, err)
	}

	log.Printf("memory-extproc gRPC server listening on %s", cfg.ListenAddr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("gRPC server stopped: %v", err)
	}
}
