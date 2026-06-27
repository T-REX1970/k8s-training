package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/user/llm-rag/services/chat-service/internal/config"
	"github.com/user/llm-rag/services/chat-service/internal/handler"
	"github.com/user/llm-rag/services/chat-service/internal/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func New(cfg config.Config, logger *slog.Logger) (*http.Server, []*grpc.ClientConn, error) {
	// gRPC 接続は lazy dial なので起動時に下流が落ちていても問題ない
	llmConn, err := grpc.NewClient(cfg.LLMServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}

	retrievalConn, err := grpc.NewClient(cfg.RetrievalServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		llmConn.Close()
		return nil, nil, err
	}

	chatHandler := handler.NewChatHandler(llmConn, retrievalConn)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(middleware.StructuredLogger(logger))

	router.GET("/healthz", handler.Healthz)
	router.GET("/readyz", handler.Readyz)
	router.POST("/chat", chatHandler.Handle)
	router.POST("/chat/stream", chatHandler.HandleStream)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 130 * time.Second,
	}
	return srv, []*grpc.ClientConn{llmConn, retrievalConn}, nil
}

func Run(ctx context.Context, srv *http.Server, logger *slog.Logger, shutdownTimeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() {
		logger.Info("server_starting", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	logger.Info("server_shutting_down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
