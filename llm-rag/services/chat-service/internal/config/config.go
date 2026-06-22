package config

import (
	"os"
	"time"
)

type Config struct {
	Port            string
	LLMServiceURL   string
	ShutdownTimeout time.Duration
}

func Load() Config {
	return Config{
		Port:            getEnv("PORT", "8080"),
		LLMServiceURL:   getEnv("LLM_SERVICE_URL", "http://llm-service:8080"),
		ShutdownTimeout: 10 * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
