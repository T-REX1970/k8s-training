FROM golang:1.22-alpine AS builder
WORKDIR /src

# gen モジュールをコピー (replace ディレクティブで ../../gen を参照)
COPY gen/ ./gen/

# 依存解決を先にキャッシュ
COPY services/chat-service/go.mod services/chat-service/go.sum* ./services/chat-service/
RUN cd services/chat-service && go mod download

COPY services/chat-service/ ./services/chat-service/
RUN cd services/chat-service && CGO_ENABLED=0 GOOS=linux go build -o /out/chat-service ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/chat-service /chat-service
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/chat-service"]
