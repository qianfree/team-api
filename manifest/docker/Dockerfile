# Multi-stage build for Team-API
# Stage 1: Build
FROM golang:1.26-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

WORKDIR /build

# Copy go mod files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build arguments
ARG VERSION=dev
ARG GOOS=linux
ARG GOARCH=amd64

# Build the application with embedded frontend
RUN apk add --no-cache bash curl && \
    curl -fsSL https://bun.sh/install | bash && \
    export BUN_INSTALL="$HOME/.bun" && \
    export PATH="$BUN_INSTALL/bin:$PATH" && \
    (cd web/admin && bun install && bun run build) && \
    (cd web/tenant && bun install && bun run build) && \
    CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} \
    go build -tags embedweb -ldflags="-s -w -X github.com/qianfree/team-api/internal/consts.Version=${VERSION}" \
    -o /app/team-api main.go

# Stage 2: Runtime
FROM alpine:3.20

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    && rm -rf /var/cache/apk/*

# Create non-root user
RUN addgroup -g 1000 team-api && \
    adduser -D -u 1000 -G team-api team-api

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/team-api /app/team-api

# Copy config template
COPY manifest/config/config.example.yaml /app/config/config.yaml

# Change ownership
RUN chown -R team-api:team-api /app

# Switch to non-root user
USER team-api

# Expose ports
# 18888: Main application port
EXPOSE 18888

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:18888/api/docs || exit 1

# Run application
CMD ["./team-api"]
