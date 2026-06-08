ifeq (, $(shell command -v docker-compose))
DOCKER_COMPOSE=docker compose
else
DOCKER_COMPOSE=docker-compose
endif

SVC_BACKEND := backend
SVC_FRONTEND := frontend
SVC_DB := mongodb
SVC_DB_UI := mongo-express
SVC_REDIS := redis
SVC_CLASSIFIER := classifier
SVC_PROMPTER := prompter
FRONTEND_API_URL ?= http://localhost:8001/api/v1
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
run-all: run-db run-prompter run-classifier run-api run-frontend ## Start local dependencies and print API run command
	@echo 'Running all applications'

.PHONY: run-api
run-api: ## Start the API
	@$(DOCKER_COMPOSE) up -d $(SVC_BACKEND)

.PHONY: rebuild-api
rebuild-api: ## Rebuild the API image
	@$(DOCKER_COMPOSE) build $(SVC_BACKEND)

.PHONY: run-frontend
run-frontend: ## Build and start the frontend using FRONTEND_API_URL
	@FRONTEND_API_URL=$(FRONTEND_API_URL) $(DOCKER_COMPOSE) up -d --build $(SVC_FRONTEND)

.PHONY: rebuild-frontend
rebuild-frontend: ## Rebuild the frontend image using FRONTEND_API_URL
	@FRONTEND_API_URL=$(FRONTEND_API_URL) $(DOCKER_COMPOSE) build $(SVC_FRONTEND)

.PHONY: run-db
run-db: ## Start MongoDB, Redis, Mongo Express, classifier, and prompter
	@$(DOCKER_COMPOSE) up -d $(SVC_DB) $(SVC_REDIS) $(SVC_DB_UI) $(SVC_CLASSIFIER) $(SVC_PROMPTER)

.PHONY: run-classifier
run-classifier: ## Start the classifier service
	@$(DOCKER_COMPOSE) up -d $(SVC_CLASSIFIER) $(SVC_PROMPTER)

.PHONY: run-prompter
run-prompter: ## Start the prompter service
	@$(DOCKER_COMPOSE) up -d $(SVC_PROMPTER)

.PHONY: run-ollama-host
run-ollama-host: ## Pull and preload the local Ollama model on the running host daemon
	@./scripts/run-ollama-host.sh

.PHONY: stop-ollama-host
stop-ollama-host: ## Unload the local Ollama model from the running host daemon
	@./scripts/stop-ollama-host.sh

.PHONY: run-db-gpu
run-db-gpu: ## Start local dependencies with the published NVIDIA GPU classifier image
	@CLASSIFIER_IMAGE=ghcr.io/ravilock/sentir-mais-classifier:latest-gpu $(DOCKER_COMPOSE) up -d $(SVC_DB) $(SVC_REDIS) $(SVC_DB_UI) $(SVC_CLASSIFIER) $(SVC_PROMPTER)

.PHONY: run-db-cpu
run-db-cpu: ## Start local dependencies with the published CPU classifier image
	@CLASSIFIER_IMAGE=ghcr.io/ravilock/sentir-mais-classifier:latest $(DOCKER_COMPOSE) up -d $(SVC_DB) $(SVC_REDIS) $(SVC_DB_UI) $(SVC_CLASSIFIER) $(SVC_PROMPTER)

stop: stop-all ## Stop local dependencies

.PHONY: stop-all
stop-all: ## Stop all docker-compose services
	@$(DOCKER_COMPOSE) stop

.PHONY: stop-db
stop-db: ## Stop MongoDB
	@$(DOCKER_COMPOSE) stop $(SVC_DB) $(SVC_REDIS)

.PHONY: stop-redis
stop-redis: ## Stop Redis
	@$(DOCKER_COMPOSE) stop $(SVC_REDIS)

.PHONY: stop-db-ui
stop-db-ui: ## Stop Mongo Express
	@$(DOCKER_COMPOSE) stop $(SVC_DB_UI)

.PHONY: stop-classifier
stop-classifier: ## Stop the classifier service
	@$(DOCKER_COMPOSE) stop $(SVC_CLASSIFIER)

.PHONY: stop-prompter
stop-prompter: ## Stop the prompter service
	@$(DOCKER_COMPOSE) stop $(SVC_PROMPTER)

.PHONY: stop-frontend
stop-frontend: ## Stop the frontend service
	@$(DOCKER_COMPOSE) stop $(SVC_FRONTEND)

logs-api: ## Show API logs
	@$(LOGS_CMD) $(SVC_BACKEND) $(SVC_CLASSIFIER) $(SVC_PROMPTER)

.PHONY: logs-db
logs-db: ## Show MongoDB logs
	@$(LOGS_CMD) $(SVC_DB) $(SVC_REDIS)

.PHONY: logs-redis
logs-redis: ## Show Redis logs
	@$(LOGS_CMD) $(SVC_REDIS)

.PHONY: logs-db-ui
logs-db-ui: ## Show Mongo Express logs
	@$(LOGS_CMD) $(SVC_DB_UI)

.PHONY: logs-classifier
logs-classifier: ## Show classifier logs
	@$(LOGS_CMD) $(SVC_CLASSIFIER)

.PHONY: logs-prompter
logs-prompter: ## Show prompter logs
	@$(LOGS_CMD) $(SVC_PROMPTER)

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

.PHONY: bash-frontend
bash-frontend: ## Open a shell in the frontend container
	@$(DOCKER_COMPOSE) exec $(SVC_FRONTEND) sh

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
