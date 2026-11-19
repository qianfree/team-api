ROOT_DIR    = $(CURDIR)
DEPLOY_NAME = "team-api"
DOCKER_NAME = "team-api"
VERSION     ?= $(shell git describe --tags --long 2>/dev/null | sed -E 's/-0-g[0-9a-f]+$$//; s/-([0-9]+)-g[0-9a-f]+$$/+\1/' || cat VERSION 2>/dev/null || echo "dev")
LDFLAGS     = -X github.com/qianfree/team-api/internal/consts.Version=$(VERSION)

# Mirror acceleration (override for non-China regions)
GOPROXY     ?= https://goproxy.cn,direct
BUN_REGISTRY ?= https://registry.npmmirror.com
BUN_CONFIG_REGISTRY = $(BUN_REGISTRY)
export GOPROXY BUN_CONFIG_REGISTRY

# Cross-compile: make build GOOS=windows GOARCH=amd64
GOOS    ?= $(shell go env GOOS)
GOARCH  ?= $(shell go env GOARCH)
export GOOS GOARCH

ifeq ($(GOOS),windows)
    BINARY = team-api.exe
else
    BINARY = team-api
endif

# DB_URL 设置方式（三选一）：
#   1. 创建 .env 文件（推荐）：  DB_URL = "host=... port=... user=... password=... dbname=... sslmode=disable"
#   2. 设置环境变量：            export DB_URL="host=..."
#   3. 直接写在下方：            DB_URL = "host=..."
ifneq (,$(wildcard .env))
    include .env
endif

.PHONY: run build build-web build-all tidy fmt lint test cov ctrl dao service migrate-up migrate-down migrate-status migrate-reset docker-build docker-up docker-down docker-logs docker-rebuild

# GoFrame hot-reload dev server
run:
	gf run main.go

# Build production binary (backend only, no frontend embedded)
build:
	go build -ldflags "$(LDFLAGS)" -o ./$(BINARY) main.go

# Build frontend assets
build-web:
	cd web/admin && bun install && bun run build
	cd web/tenant && bun install && bun run build

# Build all (frontend embedded into backend binary)
build-all: build-web
	go build -tags embedweb -ldflags "$(LDFLAGS)" -o ./$(BINARY) main.go

# Tidy go modules
tidy:
	go mod tidy

# 格式化两个 module 的全部 Go 代码（提交前三件套之一）
fmt:
	gofmt -l -w main.go internal relay relaykit plugins web

# golangci-lint 静态检查（主 module + relaykit 子 module，需 golangci-lint v2.x）
# 安装：go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
lint:
	golangci-lint run
	cd relaykit && golangci-lint run

# 单元测试（无需 DB/Redis，集成/e2e 测试由 build tag 隔离不参与）
test:
	go test ./internal/... ./relay/...
	cd relaykit && go test ./...

# 核心模块覆盖率（billing / dispatchadapter / relayconvert，规范要求 ≥80%）
cov:
	go test -coverprofile=coverage.out ./internal/logic/billing/... ./internal/dispatchadapter/...
	cd relaykit && go test -coverprofile=coverage.out ./relayconvert/...
	go tool cover -func=coverage.out | tail -1

# GoFrame code generation
ctrl:
	# 必须带 -m：本项目 controller 按 api 源文件合并生成，
	# 裸 gf gen ctrl 会按方法拆分并与现有合并文件产生重复方法定义
	gf gen ctrl -m

dao:
	gf gen dao

service:
	gf gen service

# Goose migration commands
migrate-up:
	goose -dir migrations postgres $(DB_URL) up

migrate-down:
	goose -dir migrations postgres $(DB_URL) down

migrate-status:
	goose -dir migrations postgres $(DB_URL) status

migrate-reset:
	goose -dir migrations postgres $(DB_URL) reset

# Docker commands
docker-build:
	cd manifest/docker && VERSION=$(VERSION) docker compose build --build-arg VERSION=$(VERSION)

docker-up:
	cd manifest/docker && docker compose up -d

docker-down:
	cd manifest/docker && docker compose down

docker-logs:
	cd manifest/docker && docker compose logs -f

docker-rebuild: docker-down
	cd manifest/docker && VERSION=$(VERSION) docker compose build --no-cache && VERSION=$(VERSION) docker compose up -d

docker-clean:
	cd manifest/docker && docker compose down -v
	-docker volume rm team-api-postgres_data team-api-redis_data team-api-app_logs
