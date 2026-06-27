package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	llmv1 "github.com/user/llm-rag/gen/llm/v1"
	retrievalv1 "github.com/user/llm-rag/gen/retrieval/v1"
	"google.golang.org/grpc"
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

// ChatHandler は gRPC クライアントで llm-service と retrieval-service を呼ぶ
type ChatHandler struct {
	llmClient       llmv1.LLMServiceClient
	retrievalClient retrievalv1.RetrievalServiceClient
	store           *SessionStore
}

func NewChatHandler(llmConn, retrievalConn *grpc.ClientConn) *ChatHandler {
	return &ChatHandler{
		llmClient:       llmv1.NewLLMServiceClient(llmConn),
		retrievalClient: retrievalv1.NewRetrievalServiceClient(retrievalConn),
		store:           NewSessionStore(),
	}
}

// Handle は非ストリーミングチャット (POST /chat)
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
	chunks, sources, _ := h.searchContext(c.Request.Context(), req.Message)
	prompt := buildPrompt(history, req.Message, chunks)

	resp, err := h.llmClient.Generate(c.Request.Context(), &llmv1.GenerateRequest{Prompt: prompt})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("llm-service: %v", err)})
		return
	}

	h.store.append(sessionID, turn{Role: "user", Content: req.Message})
	h.store.append(sessionID, turn{Role: "assistant", Content: resp.Response})

	c.JSON(http.StatusOK, chatResponse{SessionID: sessionID, Response: resp.Response, Sources: sources})
}

// HandleStream は SSE ストリーミングチャット (POST /chat/stream)
// イベントプロトコル:
//
//	data: {"session_id":"..."}         — 最初のイベント
//	data: {"token":"..."}               — LLM トークン（逐次）
//	data: {"sources":[...]}            — RAG 使用時のみ、生成後に送出
//	data: [DONE]                       — 終端
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
	chunks, sources, _ := h.searchContext(c.Request.Context(), req.Message)
	prompt := buildPrompt(history, req.Message, chunks)

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")

	rc := http.NewResponseController(c.Writer)
	_ = rc.SetWriteDeadline(time.Time{})

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	// セッション ID を最初に送出（ブラウザがストアする前にトークンが届くのを防ぐ）
	sessionEvt, _ := json.Marshal(map[string]string{"session_id": sessionID})
	fmt.Fprintf(c.Writer, "data: %s\n\n", sessionEvt)
	flusher.Flush()

	var accumulated strings.Builder
	if err := h.streamTokens(c.Request.Context(), prompt, flusher, c.Writer, &accumulated); err != nil {
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

// streamTokens は gRPC サーバーストリームからトークンを受信して SSE に変換する
func (h *ChatHandler) streamTokens(ctx context.Context, prompt string, flusher http.Flusher, w http.ResponseWriter, acc *strings.Builder) error {
	stream, err := h.llmClient.GenerateStream(ctx, &llmv1.GenerateRequest{Prompt: prompt})
	if err != nil {
		return err
	}

	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if chunk.Token == "" {
			continue
		}
		acc.WriteString(chunk.Token)
		data, _ := json.Marshal(map[string]string{"token": chunk.Token})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
}

// searchContext は retrieval-service を gRPC で呼び出し RAG コンテキストを取得する
func (h *ChatHandler) searchContext(ctx context.Context, query string) ([]string, []Source, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	resp, err := h.retrievalClient.Search(ctx, &retrievalv1.SearchRequest{Text: query, TopK: 3})
	if err != nil {
		return nil, nil, err
	}

	texts := make([]string, 0, len(resp.Chunks))
	sources := make([]Source, 0, len(resp.Chunks))
	for _, ch := range resp.Chunks {
		texts = append(texts, ch.Text)
		sources = append(sources, Source{DocID: ch.DocId, Title: ch.Title, Score: float64(ch.Score)})
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
