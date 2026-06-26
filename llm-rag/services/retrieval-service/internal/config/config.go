package config

import (
	"os"
	"time"
)

type Config struct {
	Port                string
	EmbeddingServiceURL string
	QdrantURL           string
	ShutdownTimeout     time.Duration
}

func Load() Config {
	return Config{
		Port:                getEnv("PORT", "8080"),
		EmbeddingServiceURL: getEnv("EMBEDDING_SERVICE_URL", "http://embedding-service:8080"),
		QdrantURL:           getEnv("QDRANT_URL", "http://qdrant:6333"),
		ShutdownTimeout:     10 * time.Second,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
