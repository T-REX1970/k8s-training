package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/user/llm-rag/services/gateway-api/internal/config"
	"github.com/user/llm-rag/services/gateway-api/internal/handler"
	"github.com/user/llm-rag/services/gateway-api/internal/middleware"
)

func New(cfg config.Config, logger *slog.Logger) (*http.Server, error) {
	chatProxy, err := handler.ChatProxy(cfg.ChatServiceURL)
	if err != nil {
		return nil, err
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(middleware.StructuredLogger(logger))
	router.Use(middleware.RateLimit(cfg.RateLimitRPS, cfg.RateLimitBurst))

	router.GET("/healthz", handler.Healthz)
	router.GET("/readyz", handler.Readyz(cfg.ChatServiceURL))
	router.POST("/api/chat", chatProxy)

	return &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 60 * time.Second,
	}, nil
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
