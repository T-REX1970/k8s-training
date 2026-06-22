FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/llm-service ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
# ARG lets phase 5's canary build produce two image variants
# (e.g. --build-arg OLLAMA_MODEL_DEFAULT=llama3.2:1b for v1,
# qwen2.5:1.5b for v2) without touching the source code.
ARG OLLAMA_MODEL_DEFAULT=llama3.2:1b
ENV OLLAMA_MODEL=${OLLAMA_MODEL_DEFAULT}
COPY --from=builder /out/llm-service /llm-service
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/llm-service"]
