.PHONY: build run lint test cover swagger clean help

# Variables
BINARY_NAME=insider-messaging
MAIN_PATH=./cmd/server
BUILD_DIR=./bin
COVERAGE_DIR=./coverage

# Default target
help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the application
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)

run: build ## Build and run the application
	@echo "Running $(BINARY_NAME)..."
	@$(BUILD_DIR)/$(BINARY_NAME)

lint: ## Run golangci-lint
	@echo "Running linter..."
	@golangci-lint run

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

test: ## Run tests
	@echo "Running tests..."
	@go test -race -v ./...

cover: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@mkdir -p $(COVERAGE_DIR)
	@go test -race -coverprofile=$(COVERAGE_DIR)/coverage.out ./...
	@go tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "Coverage report generated: $(COVERAGE_DIR)/coverage.html"

swagger: ## Generate swagger documentation
	@echo "Generating swagger docs..."
	@swag init -g $(MAIN_PATH)/main.go -o ./docs

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -rf $(COVERAGE_DIR)
	@rm -rf ./docs

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...

check: lint vet test ## Run all checks (lint, vet, test)

dev: ## Run in development mode with auto-reload (requires air)
	@echo "Starting development server..."
	@air

docker-build: ## Build docker image
	@echo "Building docker image..."
	@docker build -t $(BINARY_NAME) .

docker-run: docker-build ## Build and run docker container
	@echo "Running docker container..."
	@docker run -p 8080:8080 $(BINARY_NAME)

compose-up: ## Start services with docker-compose
	@echo "Starting services with docker-compose..."
	@docker-compose up -d

compose-down: ## Stop services with docker-compose
	@echo "Stopping services with docker-compose..."
	@docker-compose down

compose-logs: ## Show docker-compose logs
	@docker-compose logs -f