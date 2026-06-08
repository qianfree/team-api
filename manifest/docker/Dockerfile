# Multi-stage build for Team-API
#
# Mirror acceleration build args (for mainland China users):
#   GOPROXY             - Go module proxy (default: https://goproxy.cn,direct)
#   BUN_CONFIG_REGISTRY - Bun npm registry (default: https://registry.npmmirror.com)
#
# Override example: docker compose build --build-arg GOPROXY=https://proxy.golang.org,direct

# Stage 1: Frontend build
FROM --platform=$BUILDPLATFORM oven/bun:1-alpine AS frontend

ARG BUN_CONFIG_REGISTRY=https://registry.npmmirror.com
ARG VERSION=dev
ENV BUN_CONFIG_REGISTRY=${BUN_CONFIG_REGISTRY}

WORKDIR /build/web

COPY web/admin/package.json web/admin/bun.lock* ./admin/
COPY web/tenant/package.json web/tenant/bun.lock* ./tenant/

RUN cd admin && bun install --frozen-lockfile && \
    cd ../tenant && bun install --frozen-lockfile

# Copy source files after dependency install (keeps install layer cached when only source changes)
COPY web/admin/ ./admin/
COPY web/tenant/ ./tenant/

# Resolve version: build arg → VERSION file → dev
RUN if [ "$VERSION" = "dev" ]; then \
      VERSION=$(cat /build/VERSION 2>/dev/null || echo "dev"); \
    fi && \
    echo "Building frontend with version: ${VERSION}" && \
    cd admin && VITE_APP_VERSION=$VERSION bun run build && \
    cd ../tenant && VITE_APP_VERSION=$VERSION bun run build

# Stage 2: Go build
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

RUN apk add --no-cache git make gcc musl-dev

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Copy frontend dist from previous stage
COPY --from=frontend /build/web/admin/dist ./web/admin/dist
COPY --from=frontend /build/web/tenant/dist ./web/tenant/dist

ARG VERSION=dev
ARG TARGETARCH
RUN if [ "$VERSION" = "dev" ]; then \
      VERSION=$(cat /build/VERSION 2>/dev/null || echo "dev"); \
    fi && \
    echo "Building Go binary with version: ${VERSION}" && \
    CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} \
    go build -tags embedweb -ldflags="-s -w -X github.com/qianfree/team-api/internal/consts.Version=${VERSION}" \
    -o /app/team-api main.go

# Stage 3: Runtime
FROM alpine:3.20

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    && rm -rf /var/cache/apk/*

ENV TZ=Asia/Shanghai
RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

RUN addgroup -g 1000 team-api && \
    adduser -D -u 1000 -G team-api team-api

WORKDIR /app

COPY --from=builder /app/team-api /app/team-api

# Copy i18n and migrations for runtime use
COPY --from=builder /build/manifest/i18n /app/i18n
COPY --from=builder /build/migrations /app/migrations

RUN mkdir -p /app/config /app/logs && \
    chown -R team-api:team-api /app

USER team-api

EXPOSE 18888

HEALTHCHECK --interval=30s --timeout=10s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:18888/api/health || exit 1

CMD ["./team-api"]
