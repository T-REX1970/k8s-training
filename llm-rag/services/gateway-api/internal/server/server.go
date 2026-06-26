package server

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/user/llm-rag/services/gateway-api/internal/config"
	"github.com/user/llm-rag/services/gateway-api/internal/handler"
	"github.com/user/llm-rag/services/gateway-api/internal/middleware"
	"github.com/user/llm-rag/services/gateway-api/web"
)

func New(cfg config.Config, logger *slog.Logger) (*http.Server, error) {
	chatProxy, err := handler.ChatProxy(cfg.ChatServiceURL)
	if err != nil {
		return nil, err
	}

	docsProxy, err := handler.DocumentsProxy(cfg.RetrievalServiceURL)
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
	// ドキュメント管理API: /api/documents → retrieval-service /documents
	router.POST("/api/documents", docsProxy)
	router.GET("/api/documents", docsProxy)

	if err := mountWebUI(router); err != nil {
		return nil, err
	}

	return &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 60 * time.Second,
	}, nil
}

// mountWebUI serves the embedded browser chat UI: index.html at "/" and
// its assets (app.js, style.css) under "/static". Assets are compiled into
// the binary via go:embed, so the distroless runtime image needs no
// separate static file copy step.
func mountWebUI(router *gin.Engine) error {
	indexHTML, err := web.FS.ReadFile("static/index.html")
	if err != nil {
		return err
	}

	staticFS, err := fs.Sub(web.FS, "static")
	if err != nil {
		return err
	}

	router.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	})
	router.StaticFS("/static", http.FS(staticFS))

	return nil
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
