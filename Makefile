ROOT_DIR    = $(CURDIR)
DEPLOY_NAME = "team-api"
DOCKER_NAME = "team-api"
VERSION     ?= $(strip $(file < VERSION))
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

.PHONY: run build build-web build-all tidy ctrl dao service migrate-up migrate-down migrate-status migrate-reset docker-build docker-up docker-down docker-logs docker-rebuild

# GoFrame hot-reload dev server
run:
	gf run main.go

# Build production binary (backend only, no frontend embedded)
build:
	go build -ldflags "$(LDFLAGS)" -o ./$(BINARY) main.go

# Build frontend assets
build-web:
	cd web/admin && bun install && bun run build
	cd web/admin-mobile && bun install && bun run build
	cd web/tenant && bun install && bun run build

# Build all (frontend embedded into backend binary)
build-all: build-web
	go build -tags embedweb -ldflags "$(LDFLAGS)" -o ./$(BINARY) main.go

# Tidy go modules
tidy:
	go mod tidy

# GoFrame code generation
ctrl:
	gf gen ctrl

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
	cd manifest/docker && VERSION=$(VERSION) docker compose build

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
