# Multi-stage build for minimal image size
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.Version=docker" \
    -o go-anta cmd/go-anta/main.go

# Final stage - minimal runtime image
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    bash \
    curl \
    jq

# Create non-root user
RUN addgroup -g 1000 go-anta && \
    adduser -D -u 1000 -G go-anta go-anta

# Create necessary directories
RUN mkdir -p /app /data /config && \
    chown -R go-anta:go-anta /app /data /config

# Copy binary from builder
COPY --from=builder --chown=go-anta:go-anta /build/go-anta /app/go-anta

# Copy example files for reference
COPY --chown=go-anta:go-anta examples /app/examples

# Set working directory
WORKDIR /data

# Switch to non-root user
USER go-anta

# Add binary to PATH
ENV PATH="/app:${PATH}"

# Default environment variables
ENV GO_ANTA_LOG_LEVEL=info \
    GO_ANTA_DEVICE_TIMEOUT=30s \
    GO_ANTA_TEST_MAX_CONCURRENCY=10

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/go-anta", "--help"]

# Volume mounts for inventory, catalog, and output
VOLUME ["/data", "/config"]

# Default command shows help
ENTRYPOINT ["/app/go-anta"]
CMD ["--help"]