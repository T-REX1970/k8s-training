package config

import (
	"os"
	"time"
)

type Config struct {
	Port                 string
	LLMServiceAddr       string // gRPC: host:port
	RetrievalServiceAddr string // gRPC: host:port
	ShutdownTimeout      time.Duration
}

func Load() Config {
	return Config{
		Port:                 getEnv("PORT", "8080"),
		LLMServiceAddr:       getEnv("LLM_SERVICE_ADDR", "llm-service:9090"),
		RetrievalServiceAddr: getEnv("RETRIEVAL_SERVICE_ADDR", "retrieval-service:9090"),
		ShutdownTimeout:      10 * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
