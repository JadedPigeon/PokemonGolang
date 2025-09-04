# Build a static goose binary
FROM golang:1.22-alpine AS builder
WORKDIR /build
RUN apk add --no-cache git
ENV CGO_ENABLED=0 GOOS=linux
# cache modules/builds so repeat builds are fast
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install github.com/pressly/goose/v3/cmd/goose@v3.20.0

# Minimal runtime image
FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /go/bin/goose /usr/local/bin/goose
# Ensure it's executable and *actually runs* in this final image
RUN chmod +x /usr/local/bin/goose && /usr/local/bin/goose -version
ENTRYPOINT ["/usr/local/bin/goose"]
