package handler

import (
	"bytes"
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

type chatResponse struct {
	SessionID string `json:"session_id"`
	Response  string `json:"response"`
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
	llmServiceURL string
	httpClient    *http.Client
	store         *SessionStore
}

func NewChatHandler(llmServiceURL string) *ChatHandler {
	return &ChatHandler{
		llmServiceURL: llmServiceURL,
		httpClient:    &http.Client{Timeout: 60 * time.Second},
		store:         NewSessionStore(),
	}
}

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
	prompt := buildPrompt(history, req.Message)

	reply, err := h.callLLMService(c, prompt)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("llm-service call failed: %v", err)})
		return
	}

	h.store.append(sessionID, turn{Role: "user", Content: req.Message})
	h.store.append(sessionID, turn{Role: "assistant", Content: reply})

	c.JSON(http.StatusOK, chatResponse{SessionID: sessionID, Response: reply})
}

func newSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("sess-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func buildPrompt(history []turn, message string) string {
	var b strings.Builder
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

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, h.llmServiceURL+"/generate", bytes.NewReader(body))
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
