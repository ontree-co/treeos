1. The Claude.md files are not to be touched by the agents. Only when explicitly requested.
2. This repository has a pre-commit hook installed that prevents commits with linter or unit test errors. It is forbidden to circumvent this in any way. Instead the tests and linting are to be fixed.
3. This app does not need backwards compatibility to earlier versions. The goal is to have a simple solution rather.
4. Restrain from defensive fixes for issues. In this code base most of the time there is a right place for the fix. Take your time to locate it and suggest to fix it there.
5. Add a unit test first if applicable to really find out if your fix fixes the problem.
6. This project supports macOS on Apple Silicon and Linux on amd64.

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

When `TREEOS_ENV=development` or `DEBUG=true`:

- All logs (server and browser) are written to `./logs/treeos.log`
- Browser JavaScript errors are automatically captured and logged
- `console.log/error/warn` calls are forwarded to the server
- Logs are in chronological order for easy debugging

To enable development logging:

```bash
# Set in .env file
TREEOS_ENV=development

# Or via environment variable
export TREEOS_ENV=development
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

For testing and development purposes, the following credentials are available:

- Username: `ontree`
- Password: `_g8CYFF27yURWBo4`

These credentials are stored in the `.env` file and should be used when testing authenticated endpoints or simulating user interactions.
