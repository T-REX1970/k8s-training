package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type embedRequest struct {
	Text string `json:"text" binding:"required"`
}

type embedResponse struct {
	Embedding []float64 `json:"embedding"`
}

type ollamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbedResponse struct {
	Embedding []float64 `json:"embedding"`
}

type EmbedHandler struct {
	ollamaBaseURL string
	model         string
	httpClient    *http.Client
}

func NewEmbedHandler(ollamaBaseURL, model string) *EmbedHandler {
	return &EmbedHandler{
		ollamaBaseURL: ollamaBaseURL,
		model:         model,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (h *EmbedHandler) Handle(c *gin.Context) {
	var req embedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	embedding, err := h.callOllama(c, req.Text)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("ollama embeddings call failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, embedResponse{Embedding: embedding})
}

func (h *EmbedHandler) callOllama(c *gin.Context, text string) ([]float64, error) {
	body, err := json.Marshal(ollamaEmbedRequest{Model: h.model, Prompt: text})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, h.ollamaBaseURL+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var out ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out.Embedding, nil
}
