package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Readyz reports ready only if Ollama itself is reachable.
func Readyz(ollamaBaseURL string) gin.HandlerFunc {
	client := &http.Client{Timeout: 2 * time.Second}
	return func(c *gin.Context) {
		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, ollamaBaseURL+"/api/tags", nil)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "reason": err.Error()})
			return
		}

		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "reason": "ollama unreachable"})
			return
		}
		defer resp.Body.Close()

		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	}
}
