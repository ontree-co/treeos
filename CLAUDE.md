1. The Claude.md files are not to be touched by the agents. Only when explicitly requested.
2. This repository has a pre-commit hook installed that prevents commits with linter or unit test errors. It is forbidden to circumvent this in any way. Instead the tests and linting are to be fixed.
3. This app does not need backwards compatibility to earlier versions. The goal is to have a simple solution rather.
4. Restrain from defensive fixes for issues. In this code base most of the time there is a right place for the fix. Take your time to locate it and suggest to fix it there.
5. Add a unit test first if applicable to really find out if your fix fixes the problem.
6. This project supports macOS on Apple Silicon and Linux on amd64 and arm64.

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
