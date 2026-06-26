package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/user/llm-rag/services/embedding-service/internal/config"
	"github.com/user/llm-rag/services/embedding-service/internal/handler"
	"github.com/user/llm-rag/services/embedding-service/internal/middleware"
)

func New(cfg config.Config, logger *slog.Logger) *http.Server {
	embedHandler := handler.NewEmbedHandler(cfg.OllamaBaseURL, cfg.OllamaModel)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(middleware.StructuredLogger(logger))

	router.GET("/healthz", handler.Healthz)
	router.GET("/readyz", handler.Readyz(cfg.OllamaBaseURL))
	router.POST("/embed", embedHandler.Handle)

	return &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 60 * time.Second,
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
