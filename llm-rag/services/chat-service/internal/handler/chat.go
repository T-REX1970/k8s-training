package handler

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/user/llm-rag/services/chat-service/internal/middleware"
)

type chatRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message" binding:"required"`
}

type Source struct {
	DocID string  `json:"doc_id"`
	Title string  `json:"title"`
	Score float64 `json:"score"`
}

type chatResponse struct {
	SessionID string   `json:"session_id"`
	Response  string   `json:"response"`
	Sources   []Source `json:"sources,omitempty"`
}

type turn struct {
	Role    string
	Content string
}

// SessionStore keeps per-session conversation history in memory.
// Phase 0 has no persistence backend yet (Redis arrives in a later phase),
// so history is lost on restart - that's acceptable for local dev.
type SessionStore struct {
	mu       sync.Mutex
	sessions map[string][]turn
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: make(map[string][]turn)}
}

func (s *SessionStore) history(sessionID string) []turn {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]turn(nil), s.sessions[sessionID]...)
}

func (s *SessionStore) append(sessionID string, t turn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionID] = append(s.sessions[sessionID], t)
}

type generateRequest struct {
	Prompt string `json:"prompt"`
}

type generateResponse struct {
	Response string `json:"response"`
}

type ChatHandler struct {
	llmServiceURL       string
	retrievalServiceURL string
	httpClient          *http.Client
	store               *SessionStore
}

func NewChatHandler(llmServiceURL, retrievalServiceURL string) *ChatHandler {
	return &ChatHandler{
		llmServiceURL:       llmServiceURL,
		retrievalServiceURL: retrievalServiceURL,
		httpClient:          &http.Client{Timeout: 60 * time.Second},
		store:               NewSessionStore(),
	}
}

// Handle is the non-streaming chat endpoint (POST /chat).
func (h *ChatHandler) Handle(c *gin.Context) {
	var req chatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = newSessionID()
	}

	history := h.store.history(sessionID)

	var sources []Source
	var contextChunks []string
	if h.retrievalServiceURL != "" {
		if chunks, srcs, err := h.searchContext(c.Request.Context(), req.Message); err == nil {
			contextChunks = chunks
			sources = srcs
		}
	}

	prompt := buildPrompt(history, req.Message, contextChunks)

	reply, err := h.callLLMService(c, prompt)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("llm-service call failed: %v", err)})
		return
	}

	h.store.append(sessionID, turn{Role: "user", Content: req.Message})
	h.store.append(sessionID, turn{Role: "assistant", Content: reply})

	c.JSON(http.StatusOK, chatResponse{SessionID: sessionID, Response: reply, Sources: sources})
}

// HandleStream is the SSE streaming chat endpoint (POST /chat/stream).
// Event protocol:
//
//	data: {"session_id":"..."}          — first event, carries the session id
//	data: {"token":"Hello"}              — one event per LLM token
//	data: {"sources":[...]}             — sent after generation ends (RAG only)
//	data: [DONE]                        — stream terminator
func (h *ChatHandler) HandleStream(c *gin.Context) {
	var req chatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = newSessionID()
	}

	history := h.store.history(sessionID)

	var sources []Source
	var contextChunks []string
	if h.retrievalServiceURL != "" {
		if chunks, srcs, err := h.searchContext(c.Request.Context(), req.Message); err == nil {
			contextChunks = chunks
			sources = srcs
		}
	}

	prompt := buildPrompt(history, req.Message, contextChunks)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")

	// Disable write deadline so long generations aren't cut off.
	rc := http.NewResponseController(c.Writer)
	_ = rc.SetWriteDeadline(time.Time{})

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	// First event: session id so the browser can store it before tokens arrive.
	sessionEvt, _ := json.Marshal(map[string]string{"session_id": sessionID})
	fmt.Fprintf(c.Writer, "data: %s\n\n", sessionEvt)
	flusher.Flush()

	var accumulated strings.Builder
	if err := h.streamFromLLM(c, prompt, flusher, &accumulated); err != nil {
		errEvt, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintf(c.Writer, "data: %s\n\n", errEvt)
		fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
		flusher.Flush()
		return
	}

	reply := accumulated.String()
	h.store.append(sessionID, turn{Role: "user", Content: req.Message})
	h.store.append(sessionID, turn{Role: "assistant", Content: reply})

	if len(sources) > 0 {
		srcEvt, _ := json.Marshal(map[string]any{"sources": sources})
		fmt.Fprintf(c.Writer, "data: %s\n\n", srcEvt)
		flusher.Flush()
	}

	fmt.Fprintf(c.Writer, "data: [DONE]\n\n")
	flusher.Flush()
}

// streamFromLLM calls llm-service /generate/stream, pipes token events to
// the client, and accumulates the full reply in acc.
func (h *ChatHandler) streamFromLLM(c *gin.Context, prompt string, flusher http.Flusher, acc *strings.Builder) error {
	body, err := json.Marshal(generateRequest{Prompt: prompt})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost,
		h.llmServiceURL+"/generate/stream", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.RequestIDHeader, c.GetString("request_id"))

	// Use a client without a read timeout since we're streaming.
	streamClient := &http.Client{}
	resp, err := streamClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("llm-service stream returned %d", resp.StatusCode)
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			break
		}

		var chunk map[string]string
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if token, ok := chunk["token"]; ok {
			acc.WriteString(token)
		}

		fmt.Fprintf(c.Writer, "data: %s\n\n", payload)
		flusher.Flush()
	}
	return scanner.Err()
}

type retrievalSearchRequest struct {
	Text string `json:"text"`
	TopK int    `json:"top_k"`
}

type retrievalChunk struct {
	Text  string  `json:"text"`
	DocID string  `json:"doc_id"`
	Title string  `json:"title"`
	Score float64 `json:"score"`
}

type retrievalSearchResponse struct {
	Chunks []retrievalChunk `json:"chunks"`
}

// searchContext calls retrieval-service for relevant document chunks and
// their source metadata. A short deadline keeps LLM response latency bounded.
func (h *ChatHandler) searchContext(ctx context.Context, query string) ([]string, []Source, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	body, err := json.Marshal(retrievalSearchRequest{Text: query, TopK: 3})
	if err != nil {
		return nil, nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		h.retrievalServiceURL+"/search", bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("retrieval-service returned %d", resp.StatusCode)
	}

	var out retrievalSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, nil, err
	}

	texts := make([]string, 0, len(out.Chunks))
	sources := make([]Source, 0, len(out.Chunks))
	for _, ch := range out.Chunks {
		if ch.Text == "" {
			continue
		}
		texts = append(texts, ch.Text)
		sources = append(sources, Source{DocID: ch.DocID, Title: ch.Title, Score: ch.Score})
	}
	return texts, sources, nil
}

func newSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("sess-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// buildPrompt assembles the LLM prompt from history, optional RAG context, and the new message.
func buildPrompt(history []turn, message string, contextChunks []string) string {
	var b strings.Builder

	if len(contextChunks) > 0 {
		b.WriteString("以下のコンテキスト情報を参考に、ユーザーの質問に答えてください。\n\nコンテキスト:\n")
		for _, chunk := range contextChunks {
			fmt.Fprintf(&b, "- %s\n", chunk)
		}
		b.WriteString("\n")
	}

	for _, t := range history {
		fmt.Fprintf(&b, "%s: %s\n", t.Role, t.Content)
	}
	fmt.Fprintf(&b, "user: %s\nassistant:", message)
	return b.String()
}

func (h *ChatHandler) callLLMService(c *gin.Context, prompt string) (string, error) {
	body, err := json.Marshal(generateRequest{Prompt: prompt})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost,
		h.llmServiceURL+"/generate", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(middleware.RequestIDHeader, c.GetString("request_id"))

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	var out generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Response, nil
}
