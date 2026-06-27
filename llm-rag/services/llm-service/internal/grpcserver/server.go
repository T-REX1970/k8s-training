package grpcserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"

	llmv1 "github.com/user/llm-rag/gen/llm/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// server は LLMService の gRPC 実装
type server struct {
	llmv1.UnimplementedLLMServiceServer
	ollamaBaseURL string
	model         string
}

func newServer(ollamaBaseURL, model string) *server {
	return &server{ollamaBaseURL: ollamaBaseURL, model: model}
}

// Generate は一括生成（非ストリーミング）
func (s *server) Generate(ctx context.Context, req *llmv1.GenerateRequest) (*llmv1.GenerateResponse, error) {
	body, err := json.Marshal(map[string]any{
		"model": s.model, "prompt": req.Prompt, "stream": false,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal: %v", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		s.ollamaBaseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "ollama: %v", err)
	}
	defer resp.Body.Close()

	var out struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, status.Errorf(codes.Internal, "decode: %v", err)
	}
	return &llmv1.GenerateResponse{Response: out.Response}, nil
}

// GenerateStream は Ollama の stream:true をトークン単位で gRPC ストリームに流す
func (s *server) GenerateStream(req *llmv1.GenerateRequest, stream llmv1.LLMService_GenerateStreamServer) error {
	body, err := json.Marshal(map[string]any{
		"model": s.model, "prompt": req.Prompt, "stream": true,
	})
	if err != nil {
		return status.Errorf(codes.Internal, "marshal: %v", err)
	}

	httpReq, err := http.NewRequestWithContext(stream.Context(), http.MethodPost,
		s.ollamaBaseURL+"/api/generate", bytes.NewReader(body))
	if err != nil {
		return status.Errorf(codes.Internal, "request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// ストリーミング中はタイムアウトなし（コンテキストキャンセルで終了）
	resp, err := (&http.Client{}).Do(httpReq)
	if err != nil {
		return status.Errorf(codes.Unavailable, "ollama: %v", err)
	}
	defer resp.Body.Close()

	var ollamaChunk struct {
		Response string `json:"response"`
		Done     bool   `json:"done"`
	}
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		if err := json.Unmarshal([]byte(line), &ollamaChunk); err != nil {
			continue
		}
		if ollamaChunk.Response != "" {
			if err := stream.Send(&llmv1.TokenChunk{Token: ollamaChunk.Response}); err != nil {
				return err
			}
		}
		if ollamaChunk.Done {
			break
		}
	}
	return scanner.Err()
}

// New は gRPC サーバーを生成して LLMService を登録する
func New(ollamaBaseURL, model string) *grpc.Server {
	grpcSrv := grpc.NewServer()
	llmv1.RegisterLLMServiceServer(grpcSrv, newServer(ollamaBaseURL, model))
	return grpcSrv
}

// Run はポートで gRPC サーバーを起動し、ctx キャンセル時にグレースフル停止する
func Run(ctx context.Context, srv *grpc.Server, port string) error {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.Serve(lis); err != nil {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	srv.GracefulStop()
	return nil
}
