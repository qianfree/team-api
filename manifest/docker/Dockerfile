# Multi-stage build for Team-API

# Stage 1: Frontend build
FROM oven/bun:1-alpine AS frontend

WORKDIR /build/web

COPY web/admin/package.json web/admin/bun.lock* ./admin/
COPY web/tenant/package.json web/tenant/bun.lock* ./tenant/

RUN cd admin && bun install --frozen-lockfile && \
    cd ../tenant && bun install --frozen-lockfile

COPY VERSION /build/VERSION
COPY web/admin ./admin
COPY web/tenant ./tenant

RUN cd admin && bun run build && \
    cd ../tenant && bun run build

# Stage 2: Go build
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git make gcc musl-dev

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Copy frontend dist from previous stage
COPY --from=frontend /build/web/admin/dist ./web/admin/dist
COPY --from=frontend /build/web/tenant/dist ./web/tenant/dist

ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -tags embedweb -ldflags="-s -w -X github.com/qianfree/team-api/internal/consts.Version=${VERSION}" \
    -o /app/team-api main.go

# Stage 3: Runtime
FROM alpine:3.20

RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    && rm -rf /var/cache/apk/*

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
