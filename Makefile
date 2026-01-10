# Goiler Makefile
# Production-grade Go backend boilerplate

.PHONY: help build run test lint clean docker migrate generate swagger

# Variables
APP_NAME := goiler
BINARY_API := bin/api
BINARY_WORKER := bin/worker
GO := go
GOFLAGS := -ldflags="-s -w"
DOCKER_COMPOSE := docker-compose

# Database
DB_URL ?= postgres://postgres:postgres@localhost:5432/goiler?sslmode=disable
MIGRATIONS_DIR := db/migrations

# Help
help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*##"; printf ""} /^[a-zA-Z_-]+:.*?##/ { printf "  %-20s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

# Development
.PHONY: setup
setup: ## Install development dependencies
	$(GO) install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	$(GO) install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	$(GO) install github.com/swaggo/swag/cmd/swag@latest
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	$(GO) install github.com/air-verse/air@latest
	$(GO) mod download

.PHONY: dev
dev: ## Run with hot reload (requires air)
	air -c .air.toml

.PHONY: run
run: ## Run the API server
	$(GO) run ./cmd/api

.PHONY: run-worker
run-worker: ## Run the worker
	$(GO) run ./cmd/worker

# Build
.PHONY: build
build: build-api build-worker ## Build all binaries

.PHONY: build-api
build-api: ## Build the API binary
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -o $(BINARY_API) ./cmd/api

.PHONY: build-worker
build-worker: ## Build the worker binary
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -o $(BINARY_WORKER) ./cmd/worker

# Testing
.PHONY: test
test: ## Run tests
	$(GO) test -v -race -cover ./...

.PHONY: test-short
test-short: ## Run short tests
	$(GO) test -v -short ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

.PHONY: test-bench
test-bench: ## Run benchmarks
	$(GO) test -bench=. -benchmem ./...

# Linting
.PHONY: lint
lint: ## Run linter
	golangci-lint run ./...

.PHONY: lint-fix
lint-fix: ## Run linter with auto-fix
	golangci-lint run --fix ./...

.PHONY: fmt
fmt: ## Format code
	$(GO) fmt ./...
	gofmt -s -w .

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

# Code generation
.PHONY: generate
generate: sqlc-generate swagger-generate ## Generate all code

.PHONY: sqlc-generate
sqlc-generate: ## Generate sqlc code
	@# Workaround: sqlc interprets brackets in paths as glob patterns
	@ln -sfn "$$(pwd)" /tmp/goiler-sqlc-link && cd /tmp/goiler-sqlc-link && sqlc generate

.PHONY: swagger-generate
swagger-generate: ## Generate Swagger documentation
	swag init -g cmd/api/main.go -o docs/swagger

# Database migrations
.PHONY: migrate-create
migrate-create: ## Create a new migration (usage: make migrate-create name=migration_name)
	@if [ -z "$(name)" ]; then echo "Error: name is required. Usage: make migrate-create name=migration_name"; exit 1; fi
	migrate create -ext sql -dir $(MIGRATIONS_DIR) -seq $(name)

.PHONY: migrate-up
migrate-up: ## Run all pending migrations
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" -verbose up

.PHONY: migrate-up-one
migrate-up-one: ## Run one migration up
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" -verbose up 1

.PHONY: migrate-down
migrate-down: ## Rollback all migrations
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" -verbose down

.PHONY: migrate-down-one
migrate-down-one: ## Rollback one migration
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" -verbose down 1

.PHONY: migrate-force
migrate-force: ## Force migration version (usage: make migrate-force version=1)
	@if [ -z "$(version)" ]; then echo "Error: version is required"; exit 1; fi
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" force $(version)

.PHONY: migrate-version
migrate-version: ## Show current migration version
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" version

.PHONY: migrate-drop
migrate-drop: ## Drop all tables (DANGER!)
	@echo "WARNING: This will drop all tables!"
	@read -p "Are you sure? [y/N] " confirm && [ "$$confirm" = "y" ] || exit 1
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" drop -f

# Docker
.PHONY: docker-build
docker-build: ## Build Docker images
	docker build -t $(APP_NAME):latest .

.PHONY: docker-up
docker-up: ## Start all services with Docker Compose
	$(DOCKER_COMPOSE) up -d

.PHONY: docker-down
docker-down: ## Stop all services
	$(DOCKER_COMPOSE) down

.PHONY: docker-logs
docker-logs: ## View logs
	$(DOCKER_COMPOSE) logs -f

.PHONY: docker-ps
docker-ps: ## List running containers
	$(DOCKER_COMPOSE) ps

.PHONY: docker-clean
docker-clean: ## Remove all containers and volumes
	$(DOCKER_COMPOSE) down -v --remove-orphans

# Database
.PHONY: db-shell
db-shell: ## Open PostgreSQL shell
	docker exec -it goiler-postgres psql -U postgres -d goiler

.PHONY: redis-shell
redis-shell: ## Open Redis shell
	docker exec -it goiler-redis redis-cli

# Cleaning
.PHONY: clean
clean: ## Clean build artifacts
	rm -rf bin/
	rm -rf coverage.out coverage.html
	rm -rf docs/swagger/docs.go docs/swagger/swagger.json docs/swagger/swagger.yaml

# All-in-one commands
.PHONY: all
all: lint test build ## Run lint, test, and build

.PHONY: ci
ci: lint test-coverage build ## Run CI pipeline locally

.PHONY: fresh
fresh: docker-clean docker-up migrate-up generate ## Fresh start: clean, start services, migrate, generate

# Environment
.PHONY: env
env: ## Copy .env.example to .env
	@if [ ! -f .env ]; then cp .env.example .env; echo ".env file created"; else echo ".env already exists"; fi

.PHONY: check-tools
check-tools: ## Check if required tools are installed
	@which sqlc > /dev/null || (echo "sqlc not found. Run 'make setup'" && exit 1)
	@which migrate > /dev/null || (echo "migrate not found. Run 'make setup'" && exit 1)
	@which swag > /dev/null || (echo "swag not found. Run 'make setup'" && exit 1)
	@which golangci-lint > /dev/null || (echo "golangci-lint not found. Run 'make setup'" && exit 1)
	@echo "All tools are installed!"
