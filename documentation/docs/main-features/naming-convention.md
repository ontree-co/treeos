# Naming Convention

This document describes the Docker container naming convention and application structure used in OnTree Node.

## Application Structure

Each application in OnTree Node consists of three essential files:

### 1. docker-compose.yml
The standard Docker Compose configuration file that defines services, networks, and volumes for your application.

### 2. .env file
Contains environment variables that configure Docker Compose behavior:

```bash
COMPOSE_PROJECT_NAME=ontree-<app-identifier>
COMPOSE_SEPARATOR=-
```

- `COMPOSE_PROJECT_NAME`: Defines the project name used by Docker Compose for all resources
- `COMPOSE_SEPARATOR`: Sets the separator character (always `-` in OnTree)

### 3. app.yaml
The OnTree-specific configuration file that defines application metadata and expected state:

```yaml
id: <app-identifier>
name: <display-name>
primary_service: <main-service-name>
expected_services:
  - <service-1>
  - <service-2>
initial_setup_required: true  # Optional, only for newly created apps from templates
```

Fields:
- `id`: The lowercase identifier used internally (e.g., `uptime-kuma`, `openwebui-0902`)
- `name`: Human-readable display name shown in the UI
- `primary_service`: The main service that represents the application
- `expected_services`: List of container names that should be running (used by the monitoring agent)
- `initial_setup_required`: Optional boolean flag. When `true`, the agent performs initial setup tasks like pulling and locking Docker image versions

## Container Naming Convention

OnTree uses a strict naming convention for all Docker containers to ensure isolation and prevent conflicts:

```
ontree-<app-identifier>-<service-name>-<instance>
```

### Components

- **`ontree`**: Static prefix identifying OnTree-managed containers
- **`<app-identifier>`**: Lowercase version of the application directory name
- **`<service-name>`**: Service name as defined in docker-compose.yml
- **`<instance>`**: Instance number (typically 1, managed by Docker Compose)

### Important Rules

1. **Directory names are converted to lowercase** for container naming
2. **The app identifier in all three files must match** (after lowercase conversion)
3. **All OnTree containers start with `ontree-`** prefix

## Examples

### Example 1: Uptime Kuma

Directory: `/opt/ontree/apps/uptime-kuma/`

**.env file:**
```bash
COMPOSE_PROJECT_NAME=ontree-uptime-kuma
COMPOSE_SEPARATOR=-
```

**app.yaml:**
```yaml
id: uptime-kuma
name: Uptime Kuma
primary_service: uptime-kuma
expected_services:
  - ontree-uptime-kuma-uptime-kuma-1
```

**Resulting container:** `ontree-uptime-kuma-uptime-kuma-1`

### Example 2: OpenWebUI with Mixed Case

Directory: `/opt/ontree/apps/OpenWebUI-0902/`

**.env file:**
```bash
COMPOSE_PROJECT_NAME=ontree-openwebui-0902
COMPOSE_SEPARATOR=-
```

**app.yaml:**
```yaml
id: openwebui-0902
name: OpenWebUI 0902
primary_service: openwebui
expected_services:
  - ontree-openwebui-0902-openwebui-1
  - ontree-openwebui-0902-ollama-1
initial_setup_required: true
```

**Resulting containers:** 
- `ontree-openwebui-0902-openwebui-1`
- `ontree-openwebui-0902-ollama-1`

## How It Works

1. **App Creation**: When you create an app (manually or from template), OnTree automatically:
   - Creates the app directory
   - Generates the `.env` file with the correct `COMPOSE_PROJECT_NAME`
   - Creates the `app.yaml` with expected services
   - Sets `initial_setup_required: true` for template-based apps

2. **Container Operations**: When starting/stopping containers:
   - Docker Compose reads the `COMPOSE_PROJECT_NAME` from `.env`
   - All containers are created with the `ontree-<app>-<service>-<number>` pattern
   - OnTree's monitoring agent uses `expected_services` to verify container health

3. **Initial Setup**: For new apps with `initial_setup_required: true`:
   - The agent automatically pulls the latest Docker images
   - Locks the image versions in docker-compose.yml to specific tags
   - Removes the `initial_setup_required` flag after completion
   - Reports progress through the chat interface

## Network and Volume Naming

Following the same pattern:

- **Networks**: `ontree-<app-identifier>_default`
- **Volumes**: `ontree-<app-identifier>_<volume-name>`

This ensures complete isolation between applications while maintaining a clear, consistent structure that's easy to identify and manage.