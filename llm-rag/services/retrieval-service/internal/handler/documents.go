package handler

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/user/llm-rag/services/retrieval-service/internal/embedclient"
	"github.com/user/llm-rag/services/retrieval-service/internal/vectorstore"
)

// maxChunkLen bounds chunk size for documents with no paragraph breaks
// (e.g. a single long line of text).
const maxChunkLen = 500

var paragraphSplitter = regexp.MustCompile(`\n\s*\n`)

type documentRequest struct {
	Title string `json:"title"`
	Text  string `json:"text" binding:"required"`
}

type documentResponse struct {
	DocumentID string `json:"document_id"`
	Chunks     int    `json:"chunks"`
}

type documentSummary struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// DocumentIndex tracks ingested document titles for the UI's listing
// endpoint. Chunk content lives in Qdrant (persisted via PVC) so search
// survives restarts; this title index is in-memory only, mirroring
// chat-service's SessionStore until Phase 3 persistence work.
type DocumentIndex struct {
	mu   sync.Mutex
	docs []documentSummary
}

func NewDocumentIndex() *DocumentIndex {
	return &DocumentIndex{}
}

func (idx *DocumentIndex) add(summary documentSummary) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.docs = append(idx.docs, summary)
}

func (idx *DocumentIndex) list() []documentSummary {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return append([]documentSummary(nil), idx.docs...)
}

type DocumentHandler struct {
	embedder *embedclient.Client
	store    *vectorstore.Store
	index    *DocumentIndex
}

func NewDocumentHandler(embedder *embedclient.Client, store *vectorstore.Store, index *DocumentIndex) *DocumentHandler {
	return &DocumentHandler{embedder: embedder, store: store, index: index}
}

func (h *DocumentHandler) Ingest(c *gin.Context) {
	var req documentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = titleFromText(req.Text)
	}

	chunks := chunkText(req.Text)
	if len(chunks) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text produced no chunks"})
		return
	}

	docID := newID()
	points := make([]vectorstore.Point, 0, len(chunks))
	for i, chunk := range chunks {
		vector, err := h.embedder.Embed(c.Request.Context(), chunk)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("embedding chunk %d failed: %v", i, err)})
			return
		}
		points = append(points, vectorstore.Point{
			ID:     newID(),
			Vector: vector,
			Payload: map[string]any{
				"doc_id":     docID,
				"title":      title,
				"chunk_text": chunk,
			},
		})
	}

	if err := h.store.Upsert(c.Request.Context(), points); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("qdrant upsert failed: %v", err)})
		return
	}

	h.index.add(documentSummary{ID: docID, Title: title})
	c.JSON(http.StatusOK, documentResponse{DocumentID: docID, Chunks: len(chunks)})
}

func (h *DocumentHandler) List(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"documents": h.index.list()})
}

func titleFromText(text string) string {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) > 40 {
		return trimmed[:40] + "..."
	}
	return trimmed
}

// chunkText splits on blank lines (paragraphs); any paragraph longer than
// maxChunkLen is further split into fixed-size pieces. This is a simple
// MVP chunking strategy - no overlap, no token-aware splitting.
func chunkText(text string) []string {
	var chunks []string
	for _, p := range paragraphSplitter.Split(text, -1) {
		p = strings.TrimSpace(p)
		for len(p) > maxChunkLen {
			chunks = append(chunks, strings.TrimSpace(p[:maxChunkLen]))
			p = p[maxChunkLen:]
		}
		if p != "" {
			chunks = append(chunks, p)
		}
	}
	return chunks
}

// newID returns a UUIDv4-formatted string, since Qdrant point ids must be
// either an unsigned integer or a valid UUID.
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand failing means the system RNG is broken
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
