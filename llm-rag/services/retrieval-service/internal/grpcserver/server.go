package grpcserver

import (
	"context"
	"net"

	retrievalv1 "github.com/user/llm-rag/gen/retrieval/v1"
	"github.com/user/llm-rag/services/retrieval-service/internal/embedclient"
	"github.com/user/llm-rag/services/retrieval-service/internal/vectorstore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// server は RetrievalService の gRPC 実装
type server struct {
	retrievalv1.UnimplementedRetrievalServiceServer
	embedder *embedclient.Client
	store    *vectorstore.Store
}

// Search はクエリをベクトル化し Qdrant から類似チャンクを返す
func (s *server) Search(ctx context.Context, req *retrievalv1.SearchRequest) (*retrievalv1.SearchResponse, error) {
	if req.Text == "" {
		return nil, status.Errorf(codes.InvalidArgument, "text is required")
	}
	topK := int(req.TopK)
	if topK <= 0 {
		topK = 3
	}

	vector, err := s.embedder.Embed(ctx, req.Text)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "embedding: %v", err)
	}

	chunks, err := s.store.Search(ctx, vector, topK)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "search: %v", err)
	}

	out := make([]*retrievalv1.Chunk, 0, len(chunks))
	for _, ch := range chunks {
		out = append(out, &retrievalv1.Chunk{
			Text:  ch.Text,
			DocId: ch.DocID,
			Title: ch.Title,
			Score: float32(ch.Score),
		})
	}
	return &retrievalv1.SearchResponse{Chunks: out}, nil
}

// New は gRPC サーバーを生成して RetrievalService を登録する
func New(embedder *embedclient.Client, store *vectorstore.Store) *grpc.Server {
	grpcSrv := grpc.NewServer()
	retrievalv1.RegisterRetrievalServiceServer(grpcSrv, &server{embedder: embedder, store: store})
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
