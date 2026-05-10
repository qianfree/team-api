ROOT_DIR    = $(shell pwd)
DEPLOY_NAME = "team-api"
DOCKER_NAME = "team-api"

# DB_URL 设置方式（三选一）：
#   1. 创建 .env 文件（推荐）：  DB_URL = "host=... port=... user=... password=... dbname=... sslmode=disable"
#   2. 设置环境变量：            export DB_URL="host=..."
#   3. 直接写在下方：            DB_URL = "host=..."
ifneq (,$(wildcard .env))
    include .env
endif

.PHONY: run build tidy migrate-up migrate-down migrate-status migrate-reset

include ./hack/hack-cli.mk
include ./hack/hack.mk

# GoFrame hot-reload dev server
run:
	gf run main.go

# Build production binary
build:
	gf build

# Tidy go modules
tidy:
	go mod tidy

# Goose migration commands
migrate-up:
	goose -dir migrations postgres $(DB_URL) up

migrate-down:
	goose -dir migrations postgres $(DB_URL) down

migrate-status:
	goose -dir migrations postgres $(DB_URL) status

migrate-reset:
	goose -dir migrations postgres $(DB_URL) reset
