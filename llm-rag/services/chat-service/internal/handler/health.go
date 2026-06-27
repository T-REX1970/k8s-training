package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Readyz は gRPC クライアントが接続を管理するため常に ready を返す
var Readyz gin.HandlerFunc = func(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
