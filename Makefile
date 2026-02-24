.PHONY: build test test-integration test-all lint fmt clean install help test-fixture

# Variables
BINARY_NAME=grove
MAIN_PATH=./cmd/grove
BUILD_DIR=./bin

# Default target
.DEFAULT_GOAL := help

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Binary built at $(BUILD_DIR)/$(BINARY_NAME)"

test: ## Run all tests
	@echo "Running tests..."
	@go test -race -cover ./...

test-verbose: ## Run tests with verbose output
	@echo "Running tests (verbose)..."
	@go test -v -race -cover ./...

test-coverage: ## Generate coverage report
	@echo "Generating coverage report..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"

lint: ## Run linters
	@echo "Running linters..."
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed, running go vet instead..."; \
		go vet ./...; \
	fi

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Code formatted"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@echo "Clean complete"

install: build ## Install the binary to $GOPATH/bin
	@echo "Installing $(BINARY_NAME) to $$(go env GOPATH)/bin..."
	@mkdir -p "$$(go env GOPATH)/bin"
	@cp $(BUILD_DIR)/$(BINARY_NAME) "$$(go env GOPATH)/bin/$(BINARY_NAME)"
	@codesign -s - "$$(go env GOPATH)/bin/$(BINARY_NAME)" 2>/dev/null || true
	@echo "$(BINARY_NAME) installed to $$(go env GOPATH)/bin/$(BINARY_NAME)"

test-integration: ## Run integration tests (requires git, slower)
	@echo "Running integration tests..."
	@go test -v -race -tags=integration -timeout 60s ./internal/tui/

test-all: test test-integration ## Run unit + integration tests

test-fixture: ## Create persistent test fixture for TUI testing
	@./scripts/create-fixture.sh

tidy: ## Tidy go.mod
	@echo "Tidying go.mod..."
	@go mod tidy
	@echo "go.mod tidied"
