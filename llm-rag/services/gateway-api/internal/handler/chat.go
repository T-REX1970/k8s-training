package handler

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/user/llm-rag/services/gateway-api/internal/middleware"
)

// ChatProxy forwards POST /api/chat to the chat-service's POST /chat,
// preserving the request id for downstream trace correlation.
func ChatProxy(chatServiceURL string) (gin.HandlerFunc, error) {
	target, err := url.Parse(chatServiceURL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	baseDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		baseDirector(req)
		req.URL.Path = "/chat"
		req.Host = target.Host
	}

	return func(c *gin.Context) {
		c.Request.Header.Set(middleware.RequestIDHeader, c.GetString("request_id"))
		proxy.ServeHTTP(c.Writer, c.Request)
	}, nil
}
