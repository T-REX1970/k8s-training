FROM golang:1.22-alpine AS builder
WORKDIR /src

# gen モジュールをコピー (replace ディレクティブで ../../gen を参照)
COPY gen/ ./gen/

# 依存解決を先にキャッシュ
COPY services/retrieval-service/go.mod services/retrieval-service/go.sum* ./services/retrieval-service/
RUN cd services/retrieval-service && go mod download

COPY services/retrieval-service/ ./services/retrieval-service/
RUN cd services/retrieval-service && CGO_ENABLED=0 GOOS=linux go build -o /out/retrieval-service ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/retrieval-service /retrieval-service
USER nonroot:nonroot
EXPOSE 8080 9090
ENTRYPOINT ["/retrieval-service"]
