# Go client Makefile for ttime
# Usage: make [target]

# Binary name
BINARY_NAME := ttime
MAIN_PACKAGE := ./cmd/ttime

# Build directories
BUILD_DIR := ./build
DIST_DIR := ./dist

# Go build flags
LDFLAGS := -ldflags "-s -w -X main.version=dev -X main.commit=$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown) -X main.date=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)"
CGO_ENABLED := 0

# Default target
.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

.PHONY: all
all: clean build ## Clean and build everything

.PHONY: build
build: ## Build the binary for current platform
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"

.PHONY: build-all
build-all: ## Build binaries for all platforms (darwin/linux/windows, amd64/arm64)
	@echo "Building for all platforms..."
	@mkdir -p $(DIST_DIR)
	# macOS AMD64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)_darwin_amd64 $(MAIN_PACKAGE)
	# macOS ARM64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)_darwin_arm64 $(MAIN_PACKAGE)
	# Linux AMD64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)_linux_amd64 $(MAIN_PACKAGE)
	# Linux ARM64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)_linux_arm64 $(MAIN_PACKAGE)
	# Windows AMD64
	CGO_ENABLED=$(CGO_ENABLED) GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)_windows_amd64.exe $(MAIN_PACKAGE)
	@echo "All builds complete in $(DIST_DIR)/"

.PHONY: run
run: build ## Build and run the binary (pass args with ARGS="...")
	$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

.PHONY: run-daemon
run-daemon: build ## Run the daemon once (equivalent to 'ttime daemon --once')
	$(BUILD_DIR)/$(BINARY_NAME) daemon --once

.PHONY: run-setup
run-setup: build ## Run the setup TUI (equivalent to 'ttime setup')
	$(BUILD_DIR)/$(BINARY_NAME) setup

.PHONY: install
install: build ## Install binary to $GOPATH/bin or $HOME/go/bin
	@echo "Installing $(BINARY_NAME)..."
	go install $(LDFLAGS) $(MAIN_PACKAGE)
	@echo "Installed to $$(go env GOPATH)/bin/$(BINARY_NAME)"

.PHONY: uninstall
uninstall: ## Remove installed binary from $GOPATH/bin
	@echo "Removing $$(go env GOPATH)/bin/$(BINARY_NAME)..."
	@rm -f "$$(go env GOPATH)/bin/$(BINARY_NAME)"
	@echo "Uninstalled"

.PHONY: test
test: ## Run all tests
	@echo "Running tests..."
	go test -v ./...

.PHONY: test-short
test-short: ## Run tests (short mode)
	@echo "Running short tests..."
	go test -short ./...

.PHONY: test-race
test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	go test -race ./...

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: fmt
fmt: ## Format Go code
	@echo "Formatting code..."
	go fmt ./...

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint (install if not present)
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "Running golangci-lint..."; \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Install with:"; \
		echo "  go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		exit 1; \
	fi

.PHONY: tidy
tidy: ## Run go mod tidy
	@echo "Tidying modules..."
	go mod tidy

.PHONY: deps
deps: ## Download and verify dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod verify

.PHONY: clean
clean: ## Remove build artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR) $(DIST_DIR) coverage.out coverage.html
	@echo "Cleaned"

.PHONY: version
version: build ## Show binary version
	$(BUILD_DIR)/$(BINARY_NAME) version 2>/dev/null || echo "Binary built - version command not implemented"

.PHONY: check
check: fmt vet test ## Run all checks (format, vet, test)
	@echo "All checks passed!"

.PHONY: release-check
release-check: ## Check if goreleaser config is valid
	@if command -v goreleaser >/dev/null 2>&1; then \
		echo "Checking goreleaser config..."; \
		goreleaser check; \
	else \
		echo "goreleaser not installed. Install from https://goreleaser.com/install/"; \
		exit 1; \
	fi

.PHONY: snapshot
snapshot: ## Build release snapshot locally (requires goreleaser)
	@if command -v goreleaser >/dev/null 2>&1; then \
		echo "Building snapshot release..."; \
		goreleaser release --snapshot --clean; \
	else \
		echo "goreleaser not installed. Install from https://goreleaser.com/install/"; \
		exit 1; \
	fi