FROM golang:1.22-alpine AS builder
WORKDIR /src

# gen モジュールをコピー (replace ディレクティブで ../../gen を参照)
COPY gen/ ./gen/

# 依存解決を先にキャッシュ
COPY services/llm-service/go.mod services/llm-service/go.sum* ./services/llm-service/
RUN cd services/llm-service && go mod download

COPY services/llm-service/ ./services/llm-service/
RUN cd services/llm-service && CGO_ENABLED=0 GOOS=linux go build -o /out/llm-service ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
ARG OLLAMA_MODEL_DEFAULT=llama3.2:1b
ENV OLLAMA_MODEL=${OLLAMA_MODEL_DEFAULT}
COPY --from=builder /out/llm-service /llm-service
USER nonroot:nonroot
EXPOSE 8080 9090
ENTRYPOINT ["/llm-service"]
