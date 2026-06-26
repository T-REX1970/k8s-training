FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/embedding-service ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/embedding-service /embedding-service
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/embedding-service"]
