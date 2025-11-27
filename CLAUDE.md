1. The Claude.md files are not to be touched by the agents. Only when explicitly requested.
2. This repository has a pre-commit hook installed that prevents commits with linter or unit test errors. It is forbidden to circumvent this in any way. Instead the tests and linting are to be fixed.
3. This app does not need backwards compatibility to earlier versions. The goal is to have a simple solution rather.
4. Restrain from defensive fixes for issues. In this code base most of the time there is a right place for the fix. Take your time to locate it and suggest to fix it there.
5. Add a unit test first if applicable to really find out if your fix fixes the problem.
6. This project supports macOS on Apple Silicon and Linux on amd64.

## Development with Hot Reload

This project supports hot reloading using `wgo` for improved development experience:

- **`make dev`** - Run with hot reload (recommended for development)
- **`make dev-debug`** - Run with Delve debugger (for breakpoint debugging)
- **`make dev-watch`** - Simple file watching mode

See [docs/WGO_HOT_RELOAD.md](docs/WGO_HOT_RELOAD.md) for detailed setup and usage instructions.

## Tool Version Management

This project uses `.tool-versions` file to maintain consistent tool versions across all development environments.

### Required Tools
- Go 1.24.4
- golangci-lint 2.5.0

### Setup Instructions

1. **Install asdf** (recommended version manager):
   ```bash
   # macOS
   brew install asdf

   # Linux
   git clone https://github.com/asdf-vm/asdf.git ~/.asdf --branch v0.14.0
   ```

2. **Install plugins**:
   ```bash
   asdf plugin add golang
   asdf plugin add golangci-lint
   ```

3. **Install specified versions**:
   ```bash
   asdf install
   ```

The `.tool-versions` file ensures all developers and CI use the same tool versions, preventing "works on my machine" issues.

### CI/CD Configuration
GitHub Actions is configured to use the same versions specified in `.tool-versions`. The workflow automatically uses golangci-lint v2.5.0 with the v2 configuration format.

### ⚠️ Version Mismatch Protocol

**If you encounter version mismatch errors** (e.g., "the Go language version used to build golangci-lint is lower than the targeted Go version"):

1. **STOP and REPORT to the user immediately** - This is YOUR responsibility, not outside your scope
2. Explain the mismatch clearly (required vs installed versions)
3. Warn that linting and pre-commit hooks will fail
4. Offer to fix by running `asdf install` (if asdf is available)

**DO NOT overlook or skip past version errors.** Pre-commit hooks require correct versions per Rule #2.

## Working with Graphite instead of Git Directly

Unless explicitly asked otherwise, use the gt CLI for interacting with PRs and create stacks. Stacks are easier to review because each PR is smaller and logically focused.
Instead of git commit, use gt create. This will create a commit and a branch with the current changes.
Instead of git push, use gt submit --no-interactive. This will submit the current branch & all downstack branches to Graphite.

## Docker Container Naming Scheme v1.0

### Core Pattern

All Docker containers managed by this system MUST follow this naming pattern:
`ontree-<app_identifier>-<service_name>-<instance_number>`

### Components

- **Prefix**: `ontree` (hardcoded, identifies system-managed containers)
- **App Identifier**: Lowercase directory name from `/opt/ontree/apps/<app_name>/`
- **Service Name**: From docker-compose.yml services section
- **Instance Number**: Docker Compose generated (typically 1)
- **Separator**: Dash (`-`) between all components

### Implementation

Every app directory MUST have a `.env` file containing:

```
COMPOSE_PROJECT_NAME=ontree-<app_identifier>
COMPOSE_SEPARATOR=-
```

### Examples

- App in `/opt/ontree/apps/nextcloud/` with service `app` → `ontree-nextcloud-app-1`
- App in `/opt/ontree/apps/code-server/` with service `db` → `ontree-code-server-db-1`

### App IDs

Internal app IDs (database, API) use just the lowercase app name without prefix:

- Directory: `/opt/ontree/apps/Uptime-Kuma/`
- App ID: `uptime-kuma`
- Container: `ontree-uptime-kuma-<service>-1`

## Logging System

### Overview

TreeOS uses a simple dual-mode logging system optimized for development and production environments.

### Development Mode

When `DEBUG=true`:

- All logs (server and browser) are written to `./logs/treeos.log`
- Browser JavaScript errors are automatically captured and logged
- `console.log/error/warn` calls are forwarded to the server
- Logs are in chronological order for easy debugging

To enable development logging:

```bash
# Set in .env file
DEBUG=true

# Or via environment variable
export DEBUG=true
```

### Production Mode

In production (default):

- Server logs to stdout/stderr only (no file logging)
- Browser log forwarding is disabled
- Logs are captured by systemd/Docker
- External monitoring (e.g., PostHog) handles error tracking

### Log Format

```
2025/09/15 09:28:16 [INFO] [SERVER] Starting application...
2025-09-15T09:28:18.652Z [INFO] [BROWSER] Page loaded: /apps/myapp
```

### API Endpoints

- `POST /api/log` - Receives browser logs (development only)
- `GET /api/logs?limit=100&source=browser` - Query recent logs (development only)

### Implementation Files

- `internal/server/handlers_logging.go` - Browser log handling
- `cmd/treeos/main.go` - Development mode detection and log initialization

## Local Development Credentials

Use the credentials defined in your local `.env` (`TREEOS_TEST_USER` / `TREEOS_TEST_PASSWORD`). They may differ between machines; do not rely on hard-coded values here.
