package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/user/llm-rag/services/retrieval-service/internal/embedclient"
	"github.com/user/llm-rag/services/retrieval-service/internal/vectorstore"
)

func Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Readyz reports ready only if embedding-service is up and the Qdrant
// collection is reachable (EnsureCollection doubles as the readiness check
// here, since it's a cheap no-op once the collection already exists).
func Readyz(embedder *embedclient.Client, store *vectorstore.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if err := embedder.Healthy(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "reason": "embedding-service unreachable"})
			return
		}
		if err := store.EnsureCollection(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "reason": "qdrant unreachable"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	}
}
