package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Readyz reports ready only if the downstream chat-service is reachable.
func Readyz(chatServiceURL string) gin.HandlerFunc {
	client := &http.Client{Timeout: 2 * time.Second}
	return func(c *gin.Context) {
		req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, chatServiceURL+"/healthz", nil)
		if err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "reason": err.Error()})
			return
		}

		resp, err := client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "reason": "chat-service unreachable"})
			return
		}
		defer resp.Body.Close()

		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	}
}
