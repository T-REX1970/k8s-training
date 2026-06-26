package config

import (
	"os"
	"time"
)

type Config struct {
	Port            string
	OllamaBaseURL   string
	OllamaModel     string
	ShutdownTimeout time.Duration
}

func Load() Config {
	return Config{
		Port:            getEnv("PORT", "8080"),
		OllamaBaseURL:   getEnv("OLLAMA_BASE_URL", "http://ollama:11434"),
		OllamaModel:     getEnv("OLLAMA_MODEL", "nomic-embed-text"),
		ShutdownTimeout: 10 * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
