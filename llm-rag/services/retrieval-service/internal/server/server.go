package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/user/llm-rag/services/retrieval-service/internal/config"
	"github.com/user/llm-rag/services/retrieval-service/internal/embedclient"
	"github.com/user/llm-rag/services/retrieval-service/internal/handler"
	"github.com/user/llm-rag/services/retrieval-service/internal/middleware"
	"github.com/user/llm-rag/services/retrieval-service/internal/vectorstore"
)

func New(cfg config.Config, logger *slog.Logger) *http.Server {
	embedder := embedclient.New(cfg.EmbeddingServiceURL)
	store := vectorstore.New(cfg.QdrantURL)

	docIndex := handler.NewDocumentIndex()
	docHandler := handler.NewDocumentHandler(embedder, store, docIndex)
	searchHandler := handler.NewSearchHandler(embedder, store)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(middleware.StructuredLogger(logger))

	router.GET("/healthz", handler.Healthz)
	router.GET("/readyz", handler.Readyz(embedder, store))
	router.POST("/documents", docHandler.Ingest)
	router.GET("/documents", docHandler.List)
	router.POST("/search", searchHandler.Handle)

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
