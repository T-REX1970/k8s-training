package handler

import (
	"bufio"
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

// HandleStream calls Ollama with stream:true and forwards each token as an
// SSE event ("data: {\"token\":\"...\"}\n\n"). Ends with "data: [DONE]\n\n".
// The write deadline is cleared so long-running generations don't get cut off.
func (h *GenerateHandler) HandleStream(c *gin.Context) {
	var req generateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	body, err := json.Marshal(ollamaRequest{Model: h.model, Prompt: req.Prompt, Stream: true})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	httpReq, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost,
		h.ollamaBaseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "ollama stream failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")

	// Disable write deadline so the stream doesn't get cut mid-generation.
	rc := http.NewResponseController(c.Writer)
	_ = rc.SetWriteDeadline(time.Time{})

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		fmt.Fprintf(c.Writer, "data: {\"error\":\"streaming unsupported\"}\n\ndata: [DONE]\n\n")
		return
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var chunk ollamaResponse
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			continue
		}
		if chunk.Response != "" {
			data, _ := json.Marshal(map[string]string{"token": chunk.Response})
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
			flusher.Flush()
		}
		if chunk.Done {
			break
		}
	}
	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	flusher.Flush()
}

// callOllama issues a non-streaming generate request.
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
