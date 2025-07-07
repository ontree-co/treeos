# OnTree Release Specification (v2)

## Overview
This specification outlines the requirements and implementation plan for getting OnTree to a production-ready, releasable state. The goal is to create self-contained, installable releases that support both ARM64 (Apple Silicon Macs) and AMD64 (modern x86_64 CPUs) architectures, with a proper CI/CD pipeline and deployment automation.

## Release Goals

### 1. Testing & Quality Assurance
- **Unit Tests**: Achieve >70% code coverage across all packages.
- **Integration Tests**: Test Docker API interactions, database operations, and API endpoints.
- **E2E Tests**: Ensure existing Playwright tests pass consistently.
- **CI Pipeline**: All tests must pass in GitHub Actions before release.

### 2. Build & Distribution
- **Multi-Architecture Support**:
  - `darwin/arm64` (Apple Silicon Macs)
  - `linux/amd64` (Intel/AMD servers)
- **Release Artifacts**:
  - A single, self-contained binary executable for each platform.
  - Multi-arch Docker images published to a container registry.

### 3. Deployment Infrastructure
- **Ansible Playbooks**:
  - Update existing playbooks to deploy the self-contained Go binary.
  - Production deployment managed with a robust systemd service.
  - A playbook to switch a live server to a "development mode" for debugging.

## Implementation Plan

### Phase 1: Build Infrastructure & Application Hardening

#### 1.1 Create Makefile
A `Makefile` will be created to standardize common development tasks:
```makefile
# Key targets needed:
- build: Compile for current platform
- build-all: Cross-compile for all target platforms
- test: Run unit and integration tests
- test-e2e: Run Playwright E2E tests
- lint: Run golangci-lint
- clean: Remove build artifacts
- release: Create release binaries using GoReleaser
```

#### 1.2 Version Management
- **Strategy**: Versioning will be automated, derived directly from Git tags (e.g., pushing tag `v0.1.0` triggers a release).
- **Tool**: GoReleaser will manage the release process.
- **Initial Version**: The first release will be `v0.1.0`.

#### 1.3 Implement Asset Embedding
- **Goal**: To make the application fully self-contained, all static assets must be embedded into the Go binary.
- **Paths**: This includes all files within the `static/` and `templates/` directories.
- **Tool**: Use Go's `embed` package.

#### 1.4 Implement Database Migrations
- **Goal**: Establish a structured system for managing database schema changes.
- **Tool**: Implement `pressly/goose` for handling migrations.
- **Tasks**: Create an initial schema migration and add a `migrate` command to the application or Makefile.

### Phase 2: CI/CD Pipeline

#### 2.1 GitHub Actions Workflows
- **`test.yml`** (On push/PR):
  - Checks out code, sets up Go, runs linting, unit tests, and builds the binary.
  - Runs the full E2E test suite against the built binary using Docker Compose.
- **`release.yml`** (On version tag like `v*.*.*`):
  - Runs all tests to ensure quality.
  - Executes GoReleaser to build multi-arch binaries and Docker images.
  - Creates a GitHub Release, automatically generating a changelog from commits.
  - Uploads binaries as release artifacts and pushes images to a registry.

### Phase 3: Ansible Updates

#### 3.1 Production Playbook (`ontreenode-enable-production-playbook.yaml`)
- **Refactor**: The playbook will be updated to deploy the new Go binary, replacing the previous Python/Django deployment logic.
- **Process**:
  1. Download the correct binary from GitHub Releases.
  2. Place it in the application directory (`/opt/ontree/ontreenode`).
  3. Ensure the systemd service points to the new binary.
  4. Run database migrations.
  5. Restart the service.
- **Secrets**: Continue using 1Password for secret management as currently implemented.

#### 3.2 Development Mode Playbook (`ontreenode-allow-local-development-playbook.yaml`)
- **Purpose**: This playbook is for debugging on a live server. It stops the production `ontreenode` service, freeing up the port for a developer to run a temporary instance.

### Phase 4: Documentation

#### 4.1 Core Documentation
- Create `README.md`, `INSTALL.md`, and `DEPLOYMENT.md` to provide clear instructions for users and administrators.
- Maintain a `CHANGELOG.md`, which will be automatically updated by GoReleaser.

## Technical Requirements

### Binary Requirements
- **Self-Contained**: The binary must embed all necessary assets (`templates/`, `static/`) to run without external file dependencies.
- **Configuration**: Configuration will follow this precedence: **1. Environment Variables**, **2. Configuration File** (if implemented), **3. Default Values**.
- **Portability**: Statically link libraries where possible.

### Security Considerations
- No hardcoded credentials.
- Secure defaults and TLS support via Caddy.
- Authentication required for all operations.

## Success Criteria
1. **Tests**: All tests pass reliably in the CI pipeline.
2. **Builds**: GoReleaser successfully builds and releases binaries for `darwin/arm64` and `linux/amd64`.
3. **Installation**: The Ansible playbook provides a one-command deployment for the Go application.
4. **Release**: Pushing a Git tag automatically creates a complete GitHub Release.
