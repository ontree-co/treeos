# OnTree Node Makefile
# This file provides standard targets for building, testing, and releasing OnTree

# Variables
BINARY_NAME=treeos
MAIN_PATH=cmd/treeos/main.go
BUILD_DIR=build
GO=go
GOTEST=$(GO) test
GOBUILD=$(GO) build
GOCLEAN=$(GO) clean
GOLINT=$(shell go env GOPATH)/bin/golangci-lint

# Platform detection for local builds
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# Get the module path from go.mod
GOMODULE = $(shell go list -m)

# Build version information
GIT_TAG ?= $(shell git describe --tags --always --dirty)
GIT_COMMIT ?= $(shell git rev-parse HEAD)
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# ldflags to inject version info
LDFLAGS = -ldflags="-X '$(GOMODULE)/internal/version.Version=$(GIT_TAG)' -X '$(GOMODULE)/internal/version.Commit=$(GIT_COMMIT)' -X '$(GOMODULE)/internal/version.BuildDate=$(BUILD_DATE)'"

# Default target
.PHONY: all
all: build

# Build for current platform
.PHONY: build
build: embed-assets
	@echo "Building $(BINARY_NAME) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Check template syntax
.PHONY: check-templates
check-templates:
	@echo "Checking template syntax..."
	@$(GO) run cmd/template-check/main.go
	@echo "Template check complete"

# Prepare embedded assets
.PHONY: embed-assets
embed-assets: check-templates
	@echo "Preparing embedded assets..."
	@cp -r static internal/embeds/
	@cp -r templates internal/embeds/
	@echo "Assets prepared for embedding"

# Cross-compile for all target platforms
.PHONY: build-all
build-all: embed-assets
	@echo "Building for all target platforms..."
	@mkdir -p $(BUILD_DIR)

	@echo "Building for darwin/arm64 (Apple Silicon)..."
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)

	@echo "Building for linux/amd64..."
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)

	@echo "Cross-platform builds complete"

# Run unit and integration tests
.PHONY: test
test:
	@echo "Running unit and integration tests..."
	@$(GOTEST) -v -coverprofile=coverage.out ./internal/... ./cmd/...
	@echo "Tests complete"

# Run tests with race detector
.PHONY: test-race
test-race:
	@echo "Running tests with race detector..."
	@$(GOTEST) -v -race ./internal/... ./cmd/...
	@echo "Race tests complete"

# Run tests and generate coverage report
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	@$(GOTEST) -v -coverprofile=coverage.out -covermode=atomic ./internal/... ./cmd/...
	@echo "Generating coverage report..."
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run Playwright E2E tests
.PHONY: test-e2e
test-e2e: build
	@echo "Running E2E tests..."
	@echo "Checking if server is already running on port 3001..."
	@if lsof -Pi :3001 -sTCP:LISTEN -t >/dev/null 2>&1; then \
		echo "⚠️  Server already running on port 3001. Using existing server."; \
	else \
		echo "📦 Building application if needed..."; \
		$(MAKE) build; \
		echo "📂 Creating test directories..."; \
		mkdir -p ./test-apps ./logs; \
		echo "🚀 Starting server on port 3001..."; \
		ONTREE_APPS_DIR=./test-apps nohup ./$(BUILD_DIR)/$(BINARY_NAME) --demo -p 3001 > server.log 2>&1 & \
		SERVER_PID=$$!; \
		echo "Server started with PID $$SERVER_PID"; \
		echo "⏳ Waiting for server to be ready..."; \
		for i in $$(seq 1 30); do \
			if curl -s -f http://localhost:3001/ > /dev/null 2>&1; then \
				echo "✅ Server is ready!"; \
				break; \
			fi; \
			if [ $$i -eq 30 ]; then \
				echo "❌ Server failed to start after 30 seconds"; \
				if [ -f server.log ]; then \
					echo "📋 Server log output:"; \
					cat server.log; \
				fi; \
				kill $$SERVER_PID 2>/dev/null || true; \
				exit 1; \
			fi; \
			sleep 1; \
		done; \
	fi
	@echo "🧪 Running Playwright tests..."
	@cd tests/e2e && npm test || (echo "❌ E2E tests failed"; exit 1)
	@echo "📊 Test reports generated in tests/e2e/playwright-report/"
	@echo "✅ E2E tests complete"

# Run all tests (unit, integration, and E2E)
.PHONY: test-all
test-all:
	@echo "🧪 Running all tests..."
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "1️⃣  Running unit and integration tests..."
	@$(MAKE) test || (echo "❌ Unit/integration tests failed"; exit 1)
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "2️⃣  Running E2E tests..."
	@$(MAKE) test-e2e || (echo "❌ E2E tests failed"; exit 1)
	@echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
	@echo "✅ All tests passed successfully!"

# Run linter
.PHONY: lint
lint: embed-assets
	@echo "Running linter..."
	@if [ ! -f $(GOLINT) ]; then \
		echo "golangci-lint v1.62.2 not found. Please install it first."; \
		echo "Run: make install-tools"; \
		exit 1; \
	fi
	$(GOLINT) run ./...
	@echo "Linting complete"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out
	@rm -rf internal/embeds/static internal/embeds/templates
	$(GOCLEAN)
	@echo "Clean complete"

# Run the application locally
.PHONY: run
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME)

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...
	@echo "Formatting complete"

# Run go vet
.PHONY: vet
vet:
	@echo "Running go vet..."
	$(GO) vet ./...
	@echo "Vet complete"

# Install development tools
.PHONY: install-tools
install-tools:
	@echo "Installing development tools..."
	@echo "Installing golangci-lint v1.62.2..."
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2
	@echo "Installing goose for migrations..."
	@go install github.com/pressly/goose/v3/cmd/goose@latest
	@echo "Development tools installed"

# Check for module updates
.PHONY: mod-check
mod-check:
	@echo "Checking for module updates..."
	$(GO) list -u -m all

# Download dependencies
.PHONY: deps
deps:
	@echo "Downloading dependencies..."
	$(GO) mod download
	$(GO) mod tidy
	@echo "Dependencies downloaded"

# Generate test coverage report
.PHONY: coverage
coverage: test
	@echo "Generating coverage report..."
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Package release with setup files
.PHONY: package
package: build-all
	@echo "Packaging releases with setup files..."
	@mkdir -p $(BUILD_DIR)/releases

	@echo "Packaging darwin/arm64 release..."
	@mkdir -p $(BUILD_DIR)/treeos-darwin-arm64
	@cp $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(BUILD_DIR)/treeos-darwin-arm64/$(BINARY_NAME)
	@cp setup-production.sh $(BUILD_DIR)/treeos-darwin-arm64/
	@cp treeos.service $(BUILD_DIR)/treeos-darwin-arm64/
	@cp com.ontree.treeos.plist $(BUILD_DIR)/treeos-darwin-arm64/
	@cp README.setup.md $(BUILD_DIR)/treeos-darwin-arm64/README.md 2>/dev/null || echo "README.setup.md not found, skipping"
	@cd $(BUILD_DIR) && tar -czf releases/treeos-darwin-arm64.tar.gz treeos-darwin-arm64
	@rm -rf $(BUILD_DIR)/treeos-darwin-arm64

	@echo "Packaging linux/amd64 release..."
	@mkdir -p $(BUILD_DIR)/treeos-linux-amd64
	@cp $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(BUILD_DIR)/treeos-linux-amd64/$(BINARY_NAME)
	@cp setup-production.sh $(BUILD_DIR)/treeos-linux-amd64/
	@cp treeos.service $(BUILD_DIR)/treeos-linux-amd64/
	@cp com.ontree.treeos.plist $(BUILD_DIR)/treeos-linux-amd64/
	@cp README.setup.md $(BUILD_DIR)/treeos-linux-amd64/README.md 2>/dev/null || echo "README.setup.md not found, skipping"
	@cd $(BUILD_DIR) && tar -czf releases/treeos-linux-amd64.tar.gz treeos-linux-amd64
	@rm -rf $(BUILD_DIR)/treeos-linux-amd64

	@echo "Packages created in $(BUILD_DIR)/releases/"

# Install the binary locally
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME)..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/
	@echo "Installation complete"

# Database migrations
.PHONY: migrate
migrate:
	@echo "Running database migrations..."
	@$(GO) run -mod=mod github.com/pressly/goose/v3/cmd/goose@latest -dir migrations sqlite3 ./ontree.db up
	@echo "Migrations complete"

# Check migration status
.PHONY: migrate-status
migrate-status:
	@echo "Checking migration status..."
	@$(GO) run -mod=mod github.com/pressly/goose/v3/cmd/goose@latest -dir migrations sqlite3 ./ontree.db status

# Rollback last migration
.PHONY: migrate-down
migrate-down:
	@echo "Rolling back last migration..."
	@$(GO) run -mod=mod github.com/pressly/goose/v3/cmd/goose@latest -dir migrations sqlite3 ./ontree.db down
	@echo "Rollback complete"

# Create a new migration
.PHONY: migrate-create
migrate-create:
	@if [ -z "$(name)" ]; then \
		echo "Error: Please provide a migration name using 'make migrate-create name=your_migration_name'"; \
		exit 1; \
	fi
	@echo "Creating new migration: $(name)..."
	@$(GO) run -mod=mod github.com/pressly/goose/v3/cmd/goose@latest -dir migrations create $(name) sql
	@echo "Migration created"

# Print version
.PHONY: version
version:
	@echo "Version: $(VERSION)"

# Help target
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build           - Build the application for current platform"
	@echo "  build-all       - Cross-compile for darwin/arm64 and linux/amd64"
	@echo "  package         - Build and package releases with setup files"
	@echo "  test            - Run unit and integration tests"
	@echo "  test-race       - Run tests with race detector"
	@echo "  test-coverage   - Run tests and generate coverage report"
	@echo "  test-e2e        - Run Playwright E2E tests"
	@echo "  test-all        - Run all tests (unit, integration, and E2E)"
	@echo "  lint            - Run golangci-lint"
	@echo "  clean           - Remove build artifacts"
	@echo "  run             - Build and run the application"
	@echo "  fmt             - Format Go code"
	@echo "  vet             - Run go vet"
	@echo "  check-templates - Check HTML template syntax"
	@echo "  install-tools   - Install development tools"
	@echo "  deps            - Download and tidy dependencies"
	@echo "  coverage        - Generate test coverage report"
	@echo "  install         - Install binary to GOPATH/bin"
	@echo "  migrate         - Run database migrations"
	@echo "  migrate-status  - Check migration status"
	@echo "  migrate-down    - Rollback last migration"
	@echo "  migrate-create  - Create new migration (use: make migrate-create name=NAME)"
	@echo "  version         - Print version information"
	@echo "  help            - Show this help message"