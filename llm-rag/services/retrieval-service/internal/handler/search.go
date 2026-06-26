package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/user/llm-rag/services/retrieval-service/internal/embedclient"
	"github.com/user/llm-rag/services/retrieval-service/internal/vectorstore"
)

const defaultTopK = 3

type searchRequest struct {
	Text string `json:"text" binding:"required"`
	TopK int    `json:"top_k"`
}

type chunkJSON struct {
	Text  string  `json:"text"`
	DocID string  `json:"doc_id"`
	Title string  `json:"title"`
	Score float64 `json:"score"`
}

type searchResponse struct {
	Chunks []chunkJSON `json:"chunks"`
}

type SearchHandler struct {
	embedder *embedclient.Client
	store    *vectorstore.Store
}

func NewSearchHandler(embedder *embedclient.Client, store *vectorstore.Store) *SearchHandler {
	return &SearchHandler{embedder: embedder, store: store}
}

func (h *SearchHandler) Handle(c *gin.Context) {
	var req searchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	topK := req.TopK
	if topK <= 0 {
		topK = defaultTopK
	}

	vector, err := h.embedder.Embed(c.Request.Context(), req.Text)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "embedding failed: " + err.Error()})
		return
	}

	chunks, err := h.store.Search(c.Request.Context(), vector, topK)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "search failed: " + err.Error()})
		return
	}

	out := make([]chunkJSON, 0, len(chunks))
	for _, ch := range chunks {
		out = append(out, chunkJSON{Text: ch.Text, DocID: ch.DocID, Title: ch.Title, Score: ch.Score})
	}
	c.JSON(http.StatusOK, searchResponse{Chunks: out})
}
