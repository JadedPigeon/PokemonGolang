# --- Builder ---
FROM golang:1.24.3-alpine AS builder
WORKDIR /app
RUN apk add --no-cache build-base git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Build static binary from your current root main.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server .

# --- Runner ---
# Alpine runner so we can use curl in healthchecks
FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates curl
COPY --from=builder /app/server /app/server
ENV PORT=8080
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=2s --start-period=10s --retries=5 \
  CMD curl -fsS http://localhost:8080/health || exit 1
ENTRYPOINT ["/app/server"]
