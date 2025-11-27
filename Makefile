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

# Quiet mode: default to true when LOG_LEVEL=error (can override with QUIET=false)
QUIET ?= $(if $(filter error,$(LOG_LEVEL)),true,false)

# Conditional echo that respects QUIET
vecho = @if [ "$(QUIET)" != "true" ]; then echo $(1); fi

# Default target
.PHONY: all
all: build

# Build for current platform
.PHONY: build
build: embed-assets
	$(call vecho,"Building $(BINARY_NAME) for $(GOOS)/$(GOARCH)...")
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	$(call vecho,"Build complete: $(BUILD_DIR)/$(BINARY_NAME)")

# Check template syntax
.PHONY: check-templates
check-templates:
	$(call vecho,"Checking template syntax...")
	@$(GO) run cmd/template-check/main.go
	$(call vecho,"Template check complete")

# Prepare embedded assets
.PHONY: embed-assets
embed-assets: check-templates
	$(call vecho,"Preparing embedded assets...")
	@cp -r static internal/embeds/
	@cp -r templates internal/embeds/
	$(call vecho,"Assets prepared for embedding")

# Cross-compile for all target platforms
.PHONY: build-all
build-all: embed-assets
	$(call vecho,"Building for all target platforms...")
	@mkdir -p $(BUILD_DIR)

	$(call vecho,"Building for darwin/arm64 (Apple Silicon)...")
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)

	$(call vecho,"Building for linux/amd64...")
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)

	$(call vecho,"Cross-platform builds complete")

# Run unit and integration tests
.PHONY: test
test:
	$(call vecho,"Running unit and integration tests...")
	@$(GOTEST) -v -coverprofile=coverage.out ./internal/... ./cmd/...
	$(call vecho,"Tests complete")

# Run tests with race detector
.PHONY: test-race
test-race:
	$(call vecho,"Running tests with race detector...")
	@$(GOTEST) -v -race ./internal/... ./cmd/...
	$(call vecho,"Race tests complete")

# Run tests and generate coverage report
.PHONY: test-coverage
test-coverage:
	$(call vecho,"Running tests with coverage...")
	@$(GOTEST) -v -coverprofile=coverage.out -covermode=atomic ./internal/... ./cmd/...
	$(call vecho,"Generating coverage report...")
	@$(GO) tool cover -html=coverage.out -o coverage.html
	$(call vecho,"Coverage report generated: coverage.html")

# Run Playwright E2E tests
.PHONY: test-e2e
test-e2e: build
	$(call vecho,"Running E2E tests...")
	$(call vecho,"Checking if server is already running on port 3001...")
	@if lsof -Pi :3001 -sTCP:LISTEN -t >/dev/null 2>&1; then \
		if [ "$(QUIET)" != "true" ]; then echo "⚠️  Server already running on port 3001. Using existing server."; fi; \
	else \
		if [ "$(QUIET)" != "true" ]; then echo "📦 Building application if needed..."; fi; \
		$(MAKE) build; \
		if [ "$(QUIET)" != "true" ]; then echo "📂 Creating test directories..."; fi; \
		mkdir -p ./test-apps ./logs; \
		if [ "$(QUIET)" != "true" ]; then echo "🚀 Starting server on port 3001..."; fi; \
		ONTREE_APPS_DIR=./test-apps nohup ./$(BUILD_DIR)/$(BINARY_NAME) --demo -p 3001 > server.log 2>&1 & \
		SERVER_PID=$$!; \
		if [ "$(QUIET)" != "true" ]; then echo "Server started with PID $$SERVER_PID"; fi; \
		if [ "$(QUIET)" != "true" ]; then echo "⏳ Waiting for server to be ready..."; fi; \
		for i in $$(seq 1 30); do \
			if curl -s -f http://localhost:3001/ > /dev/null 2>&1; then \
				if [ "$(QUIET)" != "true" ]; then echo "✅ Server is ready!"; fi; \
				break; \
			fi; \
			if [ $$i -eq 30 ]; then \
				if [ "$(QUIET)" != "true" ]; then echo "❌ Server failed to start after 30 seconds"; fi; \
				if [ -f server.log ] && [ "$(QUIET)" != "true" ]; then \
					echo "📋 Server log output:"; \
					cat server.log; \
				fi; \
				kill $$SERVER_PID 2>/dev/null || true; \
				exit 1; \
			fi; \
			sleep 1; \
		done; \
	fi
	$(call vecho,"🧪 Running Playwright tests...")
	@cd tests/e2e && npm test || (if [ "$(QUIET)" != "true" ]; then echo "❌ E2E tests failed"; fi; exit 1)
	$(call vecho,"📊 Test reports generated in tests/e2e/playwright-report/")
	$(call vecho,"✅ E2E tests complete")

# Run all tests (unit, integration, and E2E)
.PHONY: test-all
test-all:
	$(call vecho,"🧪 Running all tests...")
	$(call vecho,"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	$(call vecho,"1️⃣  Running unit and integration tests...")
	@$(MAKE) test || (echo "❌ Unit/integration tests failed"; exit 1)
	$(call vecho,"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	$(call vecho,"2️⃣  Running E2E tests...")
	@$(MAKE) test-e2e || (echo "❌ E2E tests failed"; exit 1)
	$(call vecho,"━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	$(call vecho,"✅ All tests passed successfully!")

# Run linter
.PHONY: lint
lint: embed-assets
	$(call vecho,"Running linter...")
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "golangci-lint not found. Please install it first."; \
		echo "Run: make install-tools or use asdf install"; \
		exit 1; \
	fi
	golangci-lint run ./...
	$(call vecho,"Linting complete")

# Clean build artifacts
.PHONY: clean
clean:
	$(call vecho,"Cleaning build artifacts...")
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out
	@rm -rf internal/embeds/static internal/embeds/templates
	$(GOCLEAN)
	$(call vecho,"Clean complete")

# Run the application locally
.PHONY: run
run: build
	$(call vecho,"Running $(BINARY_NAME)...")
	./$(BUILD_DIR)/$(BINARY_NAME)

# Development mode with hot reload using wgo
.PHONY: dev
dev:
	$(call vecho,"🚀 Starting development server with hot reload...")
	@which wgo > /dev/null 2>&1 || (echo "❌ wgo not found. Install with: go install github.com/bokwoon95/wgo@latest" && exit 1)
	@./scripts/dev-wgo.sh

# Development mode with debugging support (no hot reload)
.PHONY: dev-debug
dev-debug: embed-assets
	$(call vecho,"🐛 Starting development server in debug mode (no hot reload)...")
	$(call vecho,"Use VSCode debugger or dlv to attach to the process")
	@mkdir -p $(BUILD_DIR)
	@# Source .env file if it exists to get LISTEN_ADDR
	@if [ -f .env ]; then \
		export $$(cat .env | grep -v '^#' | xargs); \
	fi; \
	DEBUG=true TREEOS_RUN_MODE=demo \
		dlv debug --headless --listen=:2345 --api-version=2 \
		--accept-multiclient --continue \
		$(MAIN_PATH)

# Development mode with simple file watching (alternative to wgo)
.PHONY: dev-watch
dev-watch: embed-assets
	$(call vecho,"👁️ Starting development with simple file watching...")
	@mkdir -p $(BUILD_DIR)
	$(call vecho,"Watching Go files for changes...")
	@# Source .env file if it exists to get LISTEN_ADDR
	@if [ -f .env ]; then \
		export $$(cat .env | grep -v '^#' | xargs); \
	fi; \
	DEBUG=true TREEOS_RUN_MODE=demo \
		wgo -file=".go" -xdir="vendor" -xdir="documentation" -xdir="tests" \
		go run $(MAIN_PATH)

# Format code
.PHONY: fmt
fmt:
	$(call vecho,"Formatting code...")
	$(GO) fmt ./...
	$(call vecho,"Formatting complete")

# Run go vet
.PHONY: vet
vet:
	$(call vecho,"Running go vet...")
	$(GO) vet ./...
	$(call vecho,"Vet complete")

# Install development tools
.PHONY: install-tools
install-tools:
	$(call vecho,"Installing development tools...")
	$(call vecho,"Installing golangci-lint v2.5.0...")
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@v2.5.0
	$(call vecho,"Installing goose for migrations...")
	@go install github.com/pressly/goose/v3/cmd/goose@latest
	$(call vecho,"Installing wgo for hot reload...")
	@go install github.com/bokwoon95/wgo@latest
	$(call vecho,"Installing delve debugger...")
	@go install github.com/go-delve/delve/cmd/dlv@latest
	$(call vecho,"Development tools installed")

# Check for module updates
.PHONY: mod-check
mod-check:
	$(call vecho,"Checking for module updates...")
	$(GO) list -u -m all

# Download dependencies
.PHONY: deps
deps:
	$(call vecho,"Downloading dependencies...")
	$(GO) mod download
	$(GO) mod tidy
	$(call vecho,"Dependencies downloaded")

# Generate test coverage report
.PHONY: coverage
coverage: test
	$(call vecho,"Generating coverage report...")
	$(GO) tool cover -html=coverage.out -o coverage.html
	$(call vecho,"Coverage report generated: coverage.html")

# Package release with setup files
.PHONY: package
package: build-all
	$(call vecho,"Packaging releases with setup files...")
	@mkdir -p $(BUILD_DIR)/releases

	$(call vecho,"Packaging darwin/arm64 release...")
	@mkdir -p $(BUILD_DIR)/treeos-darwin-arm64
	@cp $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(BUILD_DIR)/treeos-darwin-arm64/$(BINARY_NAME)
	@cp setup-production.sh $(BUILD_DIR)/treeos-darwin-arm64/
	@cp treeos.service $(BUILD_DIR)/treeos-darwin-arm64/
	@cp com.ontree.treeos.plist $(BUILD_DIR)/treeos-darwin-arm64/
	@cp README.setup.md $(BUILD_DIR)/treeos-darwin-arm64/README.md 2>/dev/null || echo "README.setup.md not found, skipping"
	@cd $(BUILD_DIR) && tar -czf releases/treeos-darwin-arm64.tar.gz treeos-darwin-arm64
	@rm -rf $(BUILD_DIR)/treeos-darwin-arm64

	$(call vecho,"Packaging linux/amd64 release...")
	@mkdir -p $(BUILD_DIR)/treeos-linux-amd64
	@cp $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(BUILD_DIR)/treeos-linux-amd64/$(BINARY_NAME)
	@cp setup-production.sh $(BUILD_DIR)/treeos-linux-amd64/
	@cp treeos.service $(BUILD_DIR)/treeos-linux-amd64/
	@cp com.ontree.treeos.plist $(BUILD_DIR)/treeos-linux-amd64/
	@cp README.setup.md $(BUILD_DIR)/treeos-linux-amd64/README.md 2>/dev/null || echo "README.setup.md not found, skipping"
	@cd $(BUILD_DIR) && tar -czf releases/treeos-linux-amd64.tar.gz treeos-linux-amd64
	@rm -rf $(BUILD_DIR)/treeos-linux-amd64

	$(call vecho,"Packages created in $(BUILD_DIR)/releases/")

# Install the binary locally
.PHONY: install
install: build
	$(call vecho,"Installing $(BINARY_NAME)...")
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/
	$(call vecho,"Installation complete")

# Database migrations
.PHONY: migrate
migrate:
	$(call vecho,"Running database migrations...")
	@$(GO) run -mod=mod github.com/pressly/goose/v3/cmd/goose@latest -dir migrations sqlite3 ./ontree.db up
	$(call vecho,"Migrations complete")

# Check migration status
.PHONY: migrate-status
migrate-status:
	$(call vecho,"Checking migration status...")
	@$(GO) run -mod=mod github.com/pressly/goose/v3/cmd/goose@latest -dir migrations sqlite3 ./ontree.db status

# Rollback last migration
.PHONY: migrate-down
migrate-down:
	$(call vecho,"Rolling back last migration...")
	@$(GO) run -mod=mod github.com/pressly/goose/v3/cmd/goose@latest -dir migrations sqlite3 ./ontree.db down
	$(call vecho,"Rollback complete")

# Create a new migration
.PHONY: migrate-create
migrate-create:
	@if [ -z "$(name)" ]; then \
		echo "Error: Please provide a migration name using 'make migrate-create name=your_migration_name'"; \
		exit 1; \
	fi
	$(call vecho,"Creating new migration: $(name)...")
	@$(GO) run -mod=mod github.com/pressly/goose/v3/cmd/goose@latest -dir migrations create $(name) sql
	$(call vecho,"Migration created")

# Print version
.PHONY: version
version:
	$(call vecho,"Version: $(VERSION)")

# Help target
.PHONY: help
help:
	$(call vecho,"Available targets:")
	$(call vecho,"  build           - Build the application for current platform")
	$(call vecho,"  build-all       - Cross-compile for darwin/arm64 and linux/amd64")
	$(call vecho,"  package         - Build and package releases with setup files")
	$(call vecho,"  test            - Run unit and integration tests")
	$(call vecho,"  test-race       - Run tests with race detector")
	$(call vecho,"  test-coverage   - Run tests and generate coverage report")
	$(call vecho,"  test-e2e        - Run Playwright E2E tests")
	$(call vecho,"  test-all        - Run all tests (unit, integration, and E2E)")
	$(call vecho,"  lint            - Run golangci-lint")
	$(call vecho,"  clean           - Remove build artifacts")
	$(call vecho,"  run             - Build and run the application")
	$(call vecho,"  dev             - Run with hot reload using wgo")
	$(call vecho,"  dev-debug       - Run with debugging support (Delve)")
	$(call vecho,"  dev-watch       - Run with simple file watching")
	$(call vecho,"  fmt             - Format Go code")
	$(call vecho,"  vet             - Run go vet")
	$(call vecho,"  check-templates - Check HTML template syntax")
	$(call vecho,"  install-tools   - Install development tools")
	$(call vecho,"  deps            - Download and tidy dependencies")
	$(call vecho,"  coverage        - Generate test coverage report")
	$(call vecho,"  install         - Install binary to GOPATH/bin")
	$(call vecho,"  migrate         - Run database migrations")
	$(call vecho,"  migrate-status  - Check migration status")
	$(call vecho,"  migrate-down    - Rollback last migration")
	$(call vecho,"  migrate-create  - Create new migration (use: make migrate-create name=NAME)")
	$(call vecho,"  version         - Print version information")
	$(call vecho,"  help            - Show this help message")
