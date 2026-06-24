# ============================================================
# BusinessSAAS Backend — Makefile
# ============================================================

.PHONY: help dev build run test lint clean docs docs-install \
        docker-build docker-dev \
        migrate-up migrate-down migrate-status migrate-create \
        db-shell redis-shell

.DEFAULT_GOAL := help

BINARY_NAME=businesssaas
BUILD_DIR=./bin
CMD_PATH=./cmd/server

GOFLAGS=-trimpath
LDFLAGS=-s -w

# swag CLI version — pin this so all developers use the same generator
SWAG_VERSION=v1.16.4

GREEN  := \033[0;32m
YELLOW := \033[0;33m
CYAN   := \033[0;36m
RESET  := \033[0m

## -------------------------------------------------------
## Help
## -------------------------------------------------------

help: ## Show this help message
	@echo ""
	@echo "$(CYAN)BusinessSAAS Backend$(RESET)"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  $(GREEN)%-22s$(RESET) %s\n", $$1, $$2}'
	@echo ""

## -------------------------------------------------------
## Development
## -------------------------------------------------------

dev: ## Run backend with Air live reload (requires Air installed)
	air -c .air.toml

run: ## Build and run backend without live reload
	go run $(CMD_PATH)

build: ## Build production binary
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		$(GOFLAGS) \
		-ldflags="$(LDFLAGS)" \
		-o $(BUILD_DIR)/$(BINARY_NAME) \
		$(CMD_PATH)
	@echo "$(GREEN)✓ Binary built: $(BUILD_DIR)/$(BINARY_NAME)$(RESET)"

## -------------------------------------------------------
## API Documentation
## -------------------------------------------------------

docs-install: ## Install swag CLI (run once per machine)
	@echo "$(CYAN)Installing swag $(SWAG_VERSION)...$(RESET)"
	go install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)
	@echo "$(GREEN)✓ swag installed: $$(swag --version)$(RESET)"

docs: ## Regenerate API docs from handler annotations (requires swag)
	@command -v swag >/dev/null 2>&1 || { \
		echo "$(YELLOW)swag not found. Run: make docs-install$(RESET)"; exit 1; \
	}
	swag init \
		-g cmd/server/main.go \
		--output docs/ \
		--oas3 \
		--parseDependency \
		--parseInternal \
		--exclude internal/tests/
	@echo "$(GREEN)✓ Swagger docs regenerated → docs/swagger.json$(RESET)"
	@echo "  Start the server (make dev) and visit: http://localhost:8080/api/v1/docs"

docs-open: ## Open API docs in browser (server must be running)
	@echo "$(CYAN)Opening API docs...$(RESET)"
	@open http://localhost:8080/api/v1/docs 2>/dev/null || \
		xdg-open http://localhost:8080/api/v1/docs 2>/dev/null || \
		echo "Visit: http://localhost:8080/api/v1/docs"

## -------------------------------------------------------
## Code Quality
## -------------------------------------------------------

test: ## Run all tests
	go test -v -race -cover ./...

test-unit: ## Run unit tests only
	go test -v -race -cover ./internal/... ./pkg/...

test-integration: ## Run integration tests only
	INTEGRATION=1 go test -v -race -cover ./internal/tests/integration/...

lint: ## Run linter (requires golangci-lint)
	golangci-lint run ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format code
	gofmt -s -w .

tidy: ## Tidy go modules
	go mod tidy

## -------------------------------------------------------
## Database Migrations (Goose)
## -------------------------------------------------------

GOOSE=goose
MIGRATIONS_DIR=./internal/migrations
GOOSE_DRIVER=postgres
GOOSE_DSN=postgres://saas_user:saas_password@localhost:5432/businesssaas?sslmode=disable

migrate-up: ## Run all pending migrations
	$(GOOSE) -dir $(MIGRATIONS_DIR) $(GOOSE_DRIVER) "$(GOOSE_DSN)" up

migrate-down: ## Roll back the last migration
	$(GOOSE) -dir $(MIGRATIONS_DIR) $(GOOSE_DRIVER) "$(GOOSE_DSN)" down

migrate-status: ## Show migration status
	$(GOOSE) -dir $(MIGRATIONS_DIR) $(GOOSE_DRIVER) "$(GOOSE_DSN)" status

migrate-reset: ## Roll back ALL migrations (destructive!)
	$(GOOSE) -dir $(MIGRATIONS_DIR) $(GOOSE_DRIVER) "$(GOOSE_DSN)" reset

migrate-create: ## Create a new migration — usage: make migrate-create NAME=create_users
	@if [ -z "$(NAME)" ]; then echo "$(YELLOW)Usage: make migrate-create NAME=migration_name$(RESET)"; exit 1; fi
	$(GOOSE) -dir $(MIGRATIONS_DIR) create $(NAME) sql

## -------------------------------------------------------
## Docker
## -------------------------------------------------------

docker-build: ## Build production Docker image
	docker build --target production -t businesssaas-backend:latest .

docker-dev: ## Build development Docker image
	docker build --target development -t businesssaas-backend:dev .

## -------------------------------------------------------
## Database Utilities
## -------------------------------------------------------

db-shell: ## Open psql shell (requires Docker Compose running)
	docker compose exec postgres psql -U $${POSTGRES_USER:-saas_user} -d $${POSTGRES_DB:-businesssaas}

redis-shell: ## Open redis-cli shell (requires Docker Compose running)
	docker compose exec redis redis-cli -a $${REDIS_PASSWORD:-saas_redis_password}

## -------------------------------------------------------
## Cleanup
## -------------------------------------------------------

clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR) ./tmp ./coverage
	@echo "$(GREEN)✓ Cleaned$(RESET)"
