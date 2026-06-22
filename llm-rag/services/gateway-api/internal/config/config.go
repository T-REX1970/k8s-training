package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port            string
	ChatServiceURL  string
	RateLimitRPS    float64
	RateLimitBurst  int
	ShutdownTimeout time.Duration
}

func Load() Config {
	return Config{
		Port:            getEnv("PORT", "8080"),
		ChatServiceURL:  getEnv("CHAT_SERVICE_URL", "http://chat-service:8080"),
		RateLimitRPS:    getEnvFloat("RATE_LIMIT_RPS", 10),
		RateLimitBurst:  getEnvInt("RATE_LIMIT_BURST", 20),
		ShutdownTimeout: getEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
