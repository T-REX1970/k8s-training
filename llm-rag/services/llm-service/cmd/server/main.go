package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/user/llm-rag/services/llm-service/internal/config"
	"github.com/user/llm-rag/services/llm-service/internal/grpcserver"
	"github.com/user/llm-rag/services/llm-service/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()

	httpSrv := server.New(cfg, logger)
	grpcSrv := grpcserver.New(cfg.OllamaBaseURL, cfg.OllamaModel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// HTTP サーバーと gRPC サーバーを並行起動し、どちらかがエラーになったら停止
	errCh := make(chan error, 2)
	go func() { errCh <- server.Run(ctx, httpSrv, logger, cfg.ShutdownTimeout) }()
	go func() {
		logger.Info("grpc_server_starting", "port", cfg.GRPCPort)
		errCh <- grpcserver.Run(ctx, grpcSrv, cfg.GRPCPort)
	}()

	if err := <-errCh; err != nil {
		logger.Error("server_failed", "error", err)
		os.Exit(1)
	}
}
