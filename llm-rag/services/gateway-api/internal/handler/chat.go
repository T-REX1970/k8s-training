package handler

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

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

// ChatStreamProxy forwards POST /api/chat/stream to chat-service POST /chat/stream.
// FlushInterval=-1 disables proxy-side buffering so SSE tokens reach the browser immediately.
func ChatStreamProxy(chatServiceURL string) (gin.HandlerFunc, error) {
	target, err := url.Parse(chatServiceURL)
	if err != nil {
		return nil, err
	}

	proxy := &httputil.ReverseProxy{
		FlushInterval: -1, // flush each write immediately for SSE
	}
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = "/chat/stream"
		req.Host = target.Host
	}

	return func(c *gin.Context) {
		// Disable the server-level write deadline for this SSE connection.
		rc := http.NewResponseController(c.Writer)
		_ = rc.SetWriteDeadline(time.Time{})
		c.Request.Header.Set(middleware.RequestIDHeader, c.GetString("request_id"))
		proxy.ServeHTTP(c.Writer, c.Request)
	}, nil
}
