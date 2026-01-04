.PHONY: help build run test clean docker-build docker-up docker-down docker-logs deploy-dev deploy-staging deploy-prod

# Load environment variables
ifneq (,$(wildcard .env.local))
    include .env.local
    export
endif

# Colors for output
GREEN  := \033[0;32m
YELLOW := \033[0;33m
RED    := \033[0;31m
NC     := \033[0m # No Color

# Default target
.DEFAULT_GOAL := help

help: ## Show this help message
	@echo '$(GREEN)FinPull - Makefile Commands$(NC)'
	@echo ''
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "$(YELLOW)%-20s$(NC) %s\n", $$1, $$2}'

# ============================================================================
# Development
# ============================================================================

build: ## Build the Go application
	@echo "$(GREEN)Building FinPull...$(NC)"
	go build -o bin/finpull ./cmd/app

run: ## Run the application locally
	@echo "$(GREEN)Running FinPull...$(NC)"
	@if [ ! -f .env.local ]; then echo "$(RED)Error: .env.local not found. Copy from .env.example$(NC)"; exit 1; fi
	./bin/finpull -config config/local.yaml

test: ## Run tests
	@echo "$(GREEN)Running tests...$(NC)"
	go test -v -race -coverprofile=coverage.out ./...

test-coverage: test ## Run tests with coverage report
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report: coverage.html$(NC)"

lint: ## Run linter
	@echo "$(GREEN)Running linter...$(NC)"
	golangci-lint run

fmt: ## Format code
	@echo "$(GREEN)Formatting code...$(NC)"
	go fmt ./...
	goimports -w .

clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning...$(NC)"
	rm -f finpull coverage.out coverage.html
	go clean

# ============================================================================
# Docker - Development
# ============================================================================

docker-build: ## Build Docker images
	@echo "$(GREEN)Building Docker images...$(NC)"
	cd docker && docker compose build --no-cache

docker-up: ## Start all Docker services (with fresh build)
	@echo "$(GREEN)Starting Docker services...$(NC)"
	cd docker && docker compose up -d --build
	@echo "$(GREEN)Services started. Waiting for health checks...$(NC)"
	@sleep 10
	@$(MAKE) docker-status

docker-down: ## Stop all Docker services
	@echo "$(YELLOW)Stopping Docker services...$(NC)"
	cd docker && docker compose down

docker-restart: ## Restart Docker services
	@$(MAKE) docker-down
	@$(MAKE) docker-up

docker-logs: ## Tail Docker logs
	cd docker && docker compose logs -f

docker-status: ## Show status of Docker services
	@echo "$(GREEN)Docker Services Status:$(NC)"
	@cd docker && docker compose ps

docker-clean: ## Remove all Docker containers, volumes, and images
	@echo "$(RED)Warning: This will remove all data. Continue? [y/N]$(NC)" && read ans && [ $${ans:-N} = y ]
	cd docker && docker compose down -v
	docker system prune -af

# ============================================================================
# Database Operations
# ============================================================================

kafka-topics: ## List Kafka topics
	docker exec finpull-kafka kafka-topics --list --bootstrap-server localhost:9092

kafka-create-topic: ## Create Kafka topic (usage: make kafka-create-topic TOPIC=finnhub)
	docker exec finpull-kafka kafka-topics \
		--create --if-not-exists \
		--topic $(or $(TOPIC),finnhub) \
		--bootstrap-server localhost:9092 \
		--partitions 3 \
		--replication-factor 1

kafka-consume: ## Consume from Kafka topic
	docker exec -it finpull-kafka kafka-console-consumer \
		--bootstrap-server localhost:9092 \
		--topic $(or $(TOPIC),finnhub) \
		--from-beginning

clickhouse-client: ## Connect to ClickHouse CLI
	docker exec -it finpull-clickhouse clickhouse-client \
		--user $(CLICKHOUSE_USER) \
		--password $(CLICKHOUSE_PASSWORD)

clickhouse-query: ## Run ClickHouse query (usage: make clickhouse-query QUERY="SELECT COUNT(*) FROM finpull.binance_btc")
	docker exec finpull-clickhouse clickhouse-client \
		--user $(CLICKHOUSE_USER) \
		--password $(CLICKHOUSE_PASSWORD) \
		--query "$(QUERY)"

postgres-client: ## Connect to Postgres CLI
	docker exec -it finpull-postgres psql -U $(POSTGRES_USER) -d $(POSTGRES_DB)

# ============================================================================
# Monitoring
# ============================================================================

metrics: ## Show Prometheus metrics
	@curl -s http://localhost:$(METRICS_PORT)/metrics

health-check: ## Check health of all services
	@echo "$(GREEN)Checking service health...$(NC)"
	@echo "ClickHouse:" && curl -s http://localhost:8123/ping || echo "$(RED)DOWN$(NC)"
	@echo "Kafka:" && docker exec finpull-kafka kafka-broker-api-versions --bootstrap-server localhost:9092 > /dev/null 2>&1 && echo "$(GREEN)UP$(NC)" || echo "$(RED)DOWN$(NC)"
	@echo "Superset:" && curl -s http://localhost:8088/health || echo "$(RED)DOWN$(NC)"
	@echo "Metrics:" && curl -s http://localhost:$(METRICS_PORT)/metrics > /dev/null && echo "$(GREEN)UP$(NC)" || echo "$(RED)DOWN$(NC)"

# ============================================================================
# Deployment
# ============================================================================

deploy-dev: ## Deploy to development environment
	@echo "$(GREEN)Deploying to development...$(NC)"
	@if [ ! -f .env.local ]; then echo "$(RED)Error: .env.local not found$(NC)"; exit 1; fi
	@$(MAKE) docker-build
	#@$(MAKE) docker-up
	@echo "$(GREEN)Development deployment complete!$(NC)"

deploy-staging: ## Deploy to staging environment
	@echo "$(GREEN)Deploying to staging...$(NC)"
	@if [ ! -f .env.staging ]; then echo "$(RED)Error: .env.staging not found$(NC)"; exit 1; fi
	@echo "$(YELLOW)TODO: Implement staging deployment$(NC)"

deploy-prod: ## Deploy to production environment
	@echo "$(RED)Deploying to PRODUCTION...$(NC)"
	@if [ ! -f .env.production ]; then echo "$(RED)Error: .env.production not found$(NC)"; exit 1; fi
	@echo "$(YELLOW)TODO: Implement production deployment$(NC)"

# ============================================================================
# Backup & Restore
# ============================================================================

backup-clickhouse: ## Backup ClickHouse data
	@echo "$(GREEN)Backing up ClickHouse...$(NC)"
	@mkdir -p backups
	docker exec finpull-clickhouse clickhouse-client \
		--user $(CLICKHOUSE_USER) \
		--password $(CLICKHOUSE_PASSWORD) \
		--query "BACKUP DATABASE finpull TO Disk('backups', 'backup-$(shell date +%Y%m%d-%H%M%S).zip')"

backup-postgres: ## Backup Postgres data
	@echo "$(GREEN)Backing up Postgres...$(NC)"
	@mkdir -p backups
	docker exec finpull-postgres pg_dump -U $(POSTGRES_USER) $(POSTGRES_DB) | gzip > backups/postgres-$(shell date +%Y%m%d-%H%M%S).sql.gz

restore-postgres: ## Restore Postgres (usage: make restore-postgres BACKUP=backups/postgres-20250127-120000.sql.gz)
	@if [ -z "$(BACKUP)" ]; then echo "$(RED)Error: BACKUP not specified$(NC)"; exit 1; fi
	@echo "$(YELLOW)Restoring Postgres from $(BACKUP)...$(NC)"
	gunzip -c $(BACKUP) | docker exec -i finpull-postgres psql -U $(POSTGRES_USER) -d $(POSTGRES_DB)

# ============================================================================
# CI/CD
# ============================================================================

ci-test: ## Run CI tests
	@echo "$(GREEN)Running CI tests...$(NC)"
	@$(MAKE) lint
	@$(MAKE) test

ci-build: ## Build for CI
	@echo "$(GREEN)Building for CI...$(NC)"
	@$(MAKE) build
	@$(MAKE) docker-build

# ============================================================================
# Documentation
# ============================================================================

docs: ## Generate documentation
	@echo "$(GREEN)Generating documentation...$(NC)"
	godoc -http=:6060

# ============================================================================
# Wire & Tools
# ============================================================================

install-tools: ## Install development tools
	@echo "$(GREEN)Installing development tools...$(NC)"
	@go install github.com/google/wire/cmd/wire@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "$(GREEN)Tools installed successfully$(NC)"

wire: ## Generate Wire dependency injection code
	@echo "$(GREEN)Generating Wire code...$(NC)"
	@cd internal/di && wire
	@echo "$(GREEN)Wire code generated$(NC)"

wire-clean: ## Clean Wire generated files
	@echo "$(YELLOW)Cleaning Wire generated files...$(NC)"
	@rm -f internal/di/wire_gen.go
	@echo "$(GREEN)Wire files cleaned$(NC)"



