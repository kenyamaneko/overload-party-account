.PHONY: build test test-integration vet fmt run tidy db-up db-down db-reset help

APP := overload-party-account

build: ## Build Docker image
	docker build -t $(APP) .

test: ## Run unit tests (Testcontainers; requires Docker running)
	go test ./... -count=1 -race

test-integration: ## Run unit + integration tests (Pub/Sub emulator container; slower)
	go test -tags=integration ./... -count=1 -race

vet: ## Run go vet
	go vet ./...

tidy: ## Tidy dependencies
	go mod tidy

fmt: ## Format code
	gofmt -s -w .

db-up: ## Start local Postgres (docker compose)
	docker compose up -d postgres

db-down: ## Stop local Postgres
	docker compose down

db-reset: ## Drop volume and recreate DB
	docker compose down -v
	docker compose up -d postgres

run: db-up ## Run account server locally against compose Postgres (local env 込み)
	PORT=9005 \
	DATABASE_URL="postgres://account:account@localhost:5432/account?sslmode=disable" \
	PUBSUB_PROJECT_ID=account-local \
	FIRESTORE_PROJECT_ID=account-local \
	FACTION_PURCHASED_SUBSCRIPTION=faction-purchased-account-sub \
	PREMIUM_UPDATED_SUBSCRIPTION=premium-updated-account-sub \
	PLAYER_ONBOARDED_SUBSCRIPTION=player-onboarded-account-sub \
	ONBOARDING_NAME_SET_SUBSCRIPTION=onboarding-name-set-account-sub \
	ONBOARDING_FACTION_SET_SUBSCRIPTION=onboarding-faction-set-account-sub \
	LOG_MODE=local \
	PUBSUB_EMULATOR_HOST=localhost:8085 \
	FIRESTORE_EMULATOR_HOST=localhost:9041 \
	go run ./cmd/server

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
