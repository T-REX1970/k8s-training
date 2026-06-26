package vectorstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const (
	collectionName = "documents"
	// vectorSize must match the output dimension of the embedding model
	// configured on embedding-service (nomic-embed-text -> 768).
	vectorSize = 768
)

// Store wraps the subset of Qdrant's REST API that retrieval-service needs.
// It talks to Qdrant directly over net/http rather than pulling in a client
// library, matching the rest of this codebase's minimal-dependency style.
type Store struct {
	baseURL    string
	httpClient *http.Client
}

func New(baseURL string) *Store {
	return &Store{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type Point struct {
	ID      string
	Vector  []float64
	Payload map[string]any
}

type ScoredChunk struct {
	Text  string
	DocID string
	Title string
	Score float64
}

// EnsureCollection creates the collection used for document chunks if it
// doesn't exist yet. It's idempotent and cheap, so callers can use it both
// as a one-time setup step and as a readiness check (Qdrant may not be up
// yet when this pod starts, since Kubernetes doesn't order Deployments).
func (s *Store) EnsureCollection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.collectionURL(), nil)
	if err != nil {
		return err
	}

	if resp, err := s.httpClient.Do(req); err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return nil
		}
	}

	body, err := json.Marshal(map[string]any{
		"vectors": map[string]any{"size": vectorSize, "distance": "Cosine"},
	})
	if err != nil {
		return err
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodPut, s.collectionURL(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("create collection: unexpected status %d", resp.StatusCode)
	}
	return nil
}

func (s *Store) Upsert(ctx context.Context, points []Point) error {
	qdrantPoints := make([]map[string]any, 0, len(points))
	for _, p := range points {
		qdrantPoints = append(qdrantPoints, map[string]any{
			"id":      p.ID,
			"vector":  p.Vector,
			"payload": p.Payload,
		})
	}

	body, err := json.Marshal(map[string]any{"points": qdrantPoints})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, s.collectionURL()+"/points", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upsert points: unexpected status %d", resp.StatusCode)
	}
	return nil
}

func (s *Store) Search(ctx context.Context, vector []float64, topK int) ([]ScoredChunk, error) {
	body, err := json.Marshal(map[string]any{
		"vector":       vector,
		"limit":        topK,
		"with_payload": true,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.collectionURL()+"/points/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search: unexpected status %d", resp.StatusCode)
	}

	var out struct {
		Result []struct {
			Score   float64        `json:"score"`
			Payload map[string]any `json:"payload"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}

	chunks := make([]ScoredChunk, 0, len(out.Result))
	for _, r := range out.Result {
		text, _ := r.Payload["chunk_text"].(string)
		if text == "" {
			continue
		}
		docID, _ := r.Payload["doc_id"].(string)
		title, _ := r.Payload["title"].(string)
		chunks = append(chunks, ScoredChunk{Text: text, DocID: docID, Title: title, Score: r.Score})
	}
	return chunks, nil
}

func (s *Store) collectionURL() string {
	return s.baseURL + "/collections/" + collectionName
}
