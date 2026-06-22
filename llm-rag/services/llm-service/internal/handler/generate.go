package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type generateRequest struct {
	Prompt string `json:"prompt" binding:"required"`
}

type generateResponse struct {
	Response string `json:"response"`
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

type GenerateHandler struct {
	ollamaBaseURL string
	model         string
	httpClient    *http.Client
}

func NewGenerateHandler(ollamaBaseURL, model string) *GenerateHandler {
	return &GenerateHandler{
		ollamaBaseURL: ollamaBaseURL,
		model:         model,
		// Local LLM inference on CPU can be slow, especially on first call
		// while the model is loaded into memory.
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

func (h *GenerateHandler) Handle(c *gin.Context) {
	var req generateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reply, err := h.callOllama(c, req.Prompt)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("ollama call failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, generateResponse{Response: reply})
}

// callOllama issues a non-streaming generate request. True token streaming
// through to the client is left for a later iteration once the rest of the
// chain (chat-service, gateway-api) also supports it end-to-end.
func (h *GenerateHandler) callOllama(c *gin.Context, prompt string) (string, error) {
	body, err := json.Marshal(ollamaRequest{Model: h.model, Prompt: prompt, Stream: false})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, h.ollamaBaseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var out ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Response, nil
}
