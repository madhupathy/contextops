.PHONY: help up down build build-cli logs seed test clean

help: ## Show help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ── Services ──────────────────────────────────────────────

up: ## Start all services
	docker compose up -d

down: ## Stop all services
	docker compose down

build: ## Build all containers
	docker compose build

clean: ## Remove volumes and containers
	docker compose down -v --remove-orphans

logs: ## Tail all logs
	docker compose logs -f

logs-api: ## Tail API logs
	docker compose logs -f api

logs-eval: ## Tail evaluator logs
	docker compose logs -f evaluator

logs-web: ## Tail web logs
	docker compose logs -f web

# ── CLI ───────────────────────────────────────────────────

build-cli: ## Build the contextops CLI binary
	cd apps/cli && go build -o ../../bin/contextops .
	@echo "Built bin/contextops"

install-cli: ## Install the contextops CLI to GOPATH
	cd apps/cli && go install .

# ── Testing ───────────────────────────────────────────────

test: test-api test-eval ## Run all tests

test-api: ## Run Go API tests
	cd apps/api && go test ./...

test-eval: ## Run Python evaluator tests
	cd apps/evaluator && python -m pytest tests/ -v

# ── Data ──────────────────────────────────────────────────

seed: ## Seed sample run data
	@echo "Seeding sample data..."
	./scripts/seed.sh

ingest-examples: build-cli ## Ingest example traces via CLI
	./bin/contextops trace ingest ./examples/traces/ --recursive

# ── Dev Shells ────────────────────────────────────────────

db-shell: ## Open psql shell
	docker compose exec postgres psql -U contextops -d contextops

redis-shell: ## Open redis-cli
	docker compose exec redis redis-cli

# ── Health Checks ─────────────────────────────────────────

health: api-health eval-health ## Check all service health

api-health: ## Check API health
	@curl -sf http://localhost:8080/health | python3 -m json.tool

eval-health: ## Check evaluator health
	@curl -sf http://localhost:8081/health | python3 -m json.tool

eval-list: ## List available evaluators
	@curl -sf http://localhost:8081/evaluators | python3 -m json.tool

# ── Project Init ──────────────────────────────────────────

init: build-cli ## Initialize a ContextOps project in current dir
	./bin/contextops init
