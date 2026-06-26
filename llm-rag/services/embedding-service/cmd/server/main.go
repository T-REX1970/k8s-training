package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/user/llm-rag/services/embedding-service/internal/config"
	"github.com/user/llm-rag/services/embedding-service/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()
	srv := server.New(cfg, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx, srv, logger, cfg.ShutdownTimeout); err != nil {
		logger.Error("server_failed", "error", err)
		os.Exit(1)
	}
}
