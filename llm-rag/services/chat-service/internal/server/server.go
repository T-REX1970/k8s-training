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
)

func New(cfg config.Config, logger *slog.Logger) *http.Server {
	chatHandler := handler.NewChatHandler(cfg.LLMServiceURL, cfg.RetrievalServiceURL)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(middleware.StructuredLogger(logger))

	router.GET("/healthz", handler.Healthz)
	router.GET("/readyz", handler.Readyz(cfg.LLMServiceURL))
	router.POST("/chat", chatHandler.Handle)
	router.POST("/chat/stream", chatHandler.HandleStream)

	return &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 130 * time.Second,
	}
}

// Run starts the HTTP server and blocks until ctx is cancelled, then
// performs a graceful shutdown bounded by shutdownTimeout.
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
