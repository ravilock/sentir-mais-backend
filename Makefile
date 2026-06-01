ifeq (, $(shell command -v docker-compose))
DOCKER_COMPOSE=docker compose
else
DOCKER_COMPOSE=docker-compose
endif

SVC_DB := mongodb
SVC_DB_UI := mongo-express
SVC_CLASSIFIER := classifier
LOGS_CMD := $(DOCKER_COMPOSE) logs --follow --tail=20
GOCACHE ?= /tmp/sentir-mais-go-cache
GO_ENV := env GOCACHE=$(GOCACHE)
UNIT_PACKAGES := $(shell $(GO_ENV) go list ./... 2>/dev/null | grep -v integrationTests)

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {printf "  \033[36m%-26s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: setup
setup: install-golangci-lint ## Install development tools

run: run-all ## Run all local dependencies

.PHONY: run-all
run-all: run-db ## Start local dependencies and print API run command
	@echo 'Run the API locally with: make run-api'

.PHONY: run-api
run-api: ## Start the API locally
	@$(GO_ENV) go run ./cmd/sentir-mais-api

.PHONY: run-db
run-db: ## Start MongoDB, Mongo Express, and the classifier
	@$(DOCKER_COMPOSE) up -d $(SVC_DB) $(SVC_DB_UI) $(SVC_CLASSIFIER)

.PHONY: run-db-gpu
run-db-gpu: ## Start local dependencies with the published NVIDIA GPU classifier image
	@CLASSIFIER_IMAGE=ghcr.io/ravilock/sentir-mais-classifier:latest-gpu $(DOCKER_COMPOSE) up -d $(SVC_DB) $(SVC_DB_UI) $(SVC_CLASSIFIER)

.PHONY: run-db-cpu
run-db-cpu: ## Start local dependencies with the published CPU classifier image
	@CLASSIFIER_IMAGE=ghcr.io/ravilock/sentir-mais-classifier:latest $(DOCKER_COMPOSE) up -d $(SVC_DB) $(SVC_DB_UI) $(SVC_CLASSIFIER)

stop: stop-all ## Stop local dependencies

.PHONY: stop-all
stop-all: ## Stop all docker-compose services
	@$(DOCKER_COMPOSE) stop

.PHONY: stop-db
stop-db: ## Stop MongoDB
	@$(DOCKER_COMPOSE) stop $(SVC_DB)

.PHONY: stop-db-ui
stop-db-ui: ## Stop Mongo Express
	@$(DOCKER_COMPOSE) stop $(SVC_DB_UI)

.PHONY: stop-classifier
stop-classifier: ## Stop the classifier service
	@$(DOCKER_COMPOSE) stop $(SVC_CLASSIFIER)

.PHONY: logs-db
logs-db: ## Show MongoDB logs
	@$(LOGS_CMD) $(SVC_DB)

.PHONY: logs-db-ui
logs-db-ui: ## Show Mongo Express logs
	@$(LOGS_CMD) $(SVC_DB_UI)

.PHONY: logs-classifier
logs-classifier: ## Show classifier logs
	@$(LOGS_CMD) $(SVC_CLASSIFIER)

.PHONY: logs-all
logs-all: ## Show logs for all docker-compose services
	@$(LOGS_CMD)

.PHONY: test
test: ## Run unit tests
	@$(GO_ENV) go test -count=1 $(UNIT_PACKAGES)

.PHONY: test-verbose
test-verbose: ## Run unit tests with verbose output
	@$(GO_ENV) go test -v -count=1 $(UNIT_PACKAGES)

.PHONY: test-integration
test-integration: ## Run integration tests
	@$(GO_ENV) go test ./integrationTests/... -count=1 -p 1

.PHONY: test-integration-verbose
test-integration-verbose: ## Run integration tests with verbose output
	@$(GO_ENV) go test ./integrationTests/... -v -count=1 -p 1

.PHONY: bash
bash: ## Open a shell in the MongoDB container
	@$(DOCKER_COMPOSE) exec $(SVC_DB) sh

.PHONY: lint-check
lint-check: ## Run linter checks
	./bin/golangci-lint run --timeout 5m

.PHONY: install-golangci-lint
install-golangci-lint: ## Install golangci-lint tool
	@echo 'Installing Golang CI Lint'
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s v2.1.6

.PHONY: connect-db
connect-db: ## Connect to MongoDB with mongosh inside the container
	@$(DOCKER_COMPOSE) exec $(SVC_DB) mongosh mongodb://localhost:27017/$(or $(MONGO_DATABASE),sentir-mais)

.PHONY: generate-mocks
generate-mocks: ## Generate mock files for testing
	mockery --all

.PHONY: docs
docs: ## Show the OpenAPI spec location
	@echo 'OpenAPI spec available at: ./openapi.yaml'

.PHONY: serve-docs
serve-docs: ## Show a local docs workflow hint
	@echo 'Serve the OpenAPI spec with your preferred viewer using ./openapi.yaml'
