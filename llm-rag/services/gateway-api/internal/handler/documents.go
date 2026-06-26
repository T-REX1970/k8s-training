package handler

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/user/llm-rag/services/gateway-api/internal/middleware"
)

// DocumentsProxy forwards /api/documents and /api/search to retrieval-service,
// stripping the /api prefix so the downstream URL matches retrieval-service's routes.
func DocumentsProxy(retrievalServiceURL string) (gin.HandlerFunc, error) {
	target, err := url.Parse(retrievalServiceURL)
	if err != nil {
		return nil, err
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	baseDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		baseDirector(req)
		req.URL.Path = strings.TrimPrefix(req.URL.Path, "/api")
		req.Host = target.Host
	}

	return func(c *gin.Context) {
		c.Request.Header.Set(middleware.RequestIDHeader, c.GetString("request_id"))
		proxy.ServeHTTP(c.Writer, c.Request)
	}, nil
}
