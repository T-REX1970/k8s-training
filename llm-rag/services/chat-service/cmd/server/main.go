package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/user/llm-rag/services/chat-service/internal/config"
	"github.com/user/llm-rag/services/chat-service/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()

	srv, conns, err := server.New(cfg, logger)
	if err != nil {
		logger.Error("server_init_failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx, srv, logger, cfg.ShutdownTimeout); err != nil {
		logger.Error("server_failed", "error", err)
		os.Exit(1)
	}
}
