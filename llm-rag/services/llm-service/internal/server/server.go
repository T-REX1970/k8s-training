package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/user/llm-rag/services/llm-service/internal/config"
	"github.com/user/llm-rag/services/llm-service/internal/handler"
	"github.com/user/llm-rag/services/llm-service/internal/middleware"
)

func New(cfg config.Config, logger *slog.Logger) *http.Server {
	generateHandler := handler.NewGenerateHandler(cfg.OllamaBaseURL, cfg.OllamaModel)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(middleware.StructuredLogger(logger))

	router.GET("/healthz", handler.Healthz)
	router.GET("/readyz", handler.Readyz(cfg.OllamaBaseURL))
	router.POST("/generate", generateHandler.Handle)
	router.POST("/generate/stream", generateHandler.HandleStream)

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
