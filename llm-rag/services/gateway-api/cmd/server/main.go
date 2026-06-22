package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/user/llm-rag/services/gateway-api/internal/config"
	"github.com/user/llm-rag/services/gateway-api/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()

	srv, err := server.New(cfg, logger)
	if err != nil {
		logger.Error("server_init_failed", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx, srv, logger, cfg.ShutdownTimeout); err != nil {
		logger.Error("server_failed", "error", err)
		os.Exit(1)
	}
}
