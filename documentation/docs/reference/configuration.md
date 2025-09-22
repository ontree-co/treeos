---
sidebar_position: 1
---

# Configuration Reference

OnTree can be configured through environment variables, command-line flags, or a configuration file. This reference covers all available options.

## Configuration Priority

OnTree loads configuration in this order (highest priority first):

1. **Command-line flags** - Override everything
2. **Environment variables** - Override config file
3. **Configuration file** - `config.toml` in working directory
4. **Default values** - Built-in defaults

## Configuration File

Create `config.toml` in the same directory as the OnTree binary:

```toml
# Server Configuration
port = 8080
host = "0.0.0.0"
base_url = "http://localhost:8080"

# Database Configuration
database_path = "./ontree.db"
database_max_connections = 25
database_max_idle_connections = 5

# Docker Configuration
docker_socket = "/var/run/docker.sock"
apps_directory = "./apps"

# Monitoring
monitoring_enabled = true
monitoring_interval = 60
monitoring_retention_days = 7

# Domain Configuration
public_base_domain = "example.com"
tailscale_base_domain = "machine.tail-scale.ts.net"
caddy_admin_url = "http://localhost:2019"

# Security
session_secret = "change-me-to-random-string"
cors_allowed_origins = ["http://localhost:3000"]

# Features
enable_monitoring = true
enable_templates = true
enable_compose_editor = true

# Logging
log_level = "info"
log_format = "json"
```

## Environment Variables

All configuration options can be set via environment variables:

```bash
# Server
PORT=8080
HOST=0.0.0.0
BASE_URL=http://localhost:8080

# Database
DATABASE_PATH=./ontree.db
DATABASE_MAX_CONNECTIONS=25

# Docker
DOCKER_SOCKET=/var/run/docker.sock
APPS_DIRECTORY=./apps

# Monitoring
MONITORING_ENABLED=true
MONITORING_INTERVAL=60
MONITORING_RETENTION_DAYS=7

# Domains
PUBLIC_BASE_DOMAIN=example.com
TAILSCALE_BASE_DOMAIN=machine.tail-scale.ts.net
CADDY_ADMIN_URL=http://localhost:2019

# Logging
LOG_LEVEL=debug
LOG_FORMAT=text
```

## Command-Line Flags

Available flags when starting OnTree:

```bash
treeos [flags]

Flags:
  --port int                 Port to listen on (default 8080)
  --host string             Host to bind to (default "0.0.0.0")
  --config string           Config file path (default "./config.toml")
  --database string         Database path (default "./ontree.db")
  --apps-dir string         Apps directory (default "./apps")
  --log-level string        Log level: debug|info|warn|error (default "info")
  --debug                   Enable debug mode
  --version                 Show version and exit
  --help                    Show help
```

Example usage:
```bash
# Start on different port
treeos --port 3000

# Use different database
treeos --database /var/lib/ontree/data.db

# Enable debug logging
treeos --debug --log-level debug
```

## Configuration Options

### Server Settings

#### `port`
- **Type**: Integer
- **Default**: `8080`
- **Description**: HTTP server port
- **Environment**: `PORT`

#### `host`
- **Type**: String
- **Default**: `"0.0.0.0"`
- **Description**: Interface to bind to
- **Environment**: `HOST`
- **Options**:
  - `"0.0.0.0"` - All interfaces
  - `"127.0.0.1"` - Localhost only
  - Specific IP address

#### `base_url`
- **Type**: String
- **Default**: `"http://localhost:8080"`
- **Description**: Public URL for the application
- **Environment**: `BASE_URL`
- **Used for**: Generating links, redirects

### Database Settings

#### `database_path`
- **Type**: String
- **Default**: `"./ontree.db"`
- **Description**: SQLite database file location
- **Environment**: `DATABASE_PATH`

#### `database_max_connections`
- **Type**: Integer
- **Default**: `25`
- **Description**: Maximum database connections
- **Environment**: `DATABASE_MAX_CONNECTIONS`

#### `database_max_idle_connections`
- **Type**: Integer
- **Default**: `5`
- **Description**: Maximum idle connections
- **Environment**: `DATABASE_MAX_IDLE_CONNECTIONS`

### Docker Settings

#### `docker_socket`
- **Type**: String
- **Default**: `"/var/run/docker.sock"`
- **Description**: Docker daemon socket path
- **Environment**: `DOCKER_SOCKET`
- **Windows**: Use `"//./pipe/docker_engine"`

#### `apps_directory`
- **Type**: String
- **Default**: Platform-specific (see below)
- **Description**: Directory for application data
- **Environment**: `APPS_DIRECTORY`

##### Platform-Specific Defaults

OnTree uses different default paths based on the operating system:

**Linux:**
- Apps directory: `/opt/ontree/apps`
- Shared resources: `/opt/ontree/shared`
  - Ollama models: `/opt/ontree/shared/ollama`
- Database: `ontree.db` (current directory)

**macOS (Darwin):**
- Apps directory: `./apps` (relative to binary/repository)
- Shared resources: `./shared` (relative to binary/repository)
  - Ollama models: `./shared/ollama`
- Database: `ontree.db` (current directory)

**Note**: On macOS, paths are relative to allow development without root permissions. In production on Linux, absolute paths under `/opt/ontree` are used for system-wide installation.

## Docker Integration

OnTree integrates deeply with Docker Compose to manage containerized applications. Understanding how OnTree works with Docker is essential for proper configuration and operation.

### Application Structure

Each OnTree application requires three files in its directory:

1. **`docker-compose.yml`** - Standard Docker Compose configuration
2. **`.env`** - Docker Compose environment variables (auto-generated)
3. **`app.yaml`** - OnTree metadata and configuration (auto-generated)

### Container Naming Convention

OnTree enforces a strict naming convention for all Docker resources to ensure proper isolation and management:

```
ontree-<app-identifier>-<service>-<instance>
```

Key points:
- All app directory names are converted to **lowercase** for Docker operations
- The prefix `ontree-` identifies OnTree-managed containers
- The `.env` file contains `COMPOSE_PROJECT_NAME=ontree-<app-identifier>`
- Networks follow the pattern: `ontree-<app-identifier>_default`
- Volumes follow the pattern: `ontree-<app-identifier>_<volume-name>`

Example for an app in `/opt/ontree/apps/MyWebApp/`:
- Container: `ontree-mywebapp-web-1`
- Network: `ontree-mywebapp_default`
- Volume: `ontree-mywebapp_data`

### Docker Compose Integration

OnTree uses Docker Compose for all container operations:

| OnTree Action | Docker Compose Command | Uses From |
|--------------|------------------------|-----------|
| Start App | `docker-compose up -d` | COMPOSE_PROJECT_NAME in .env |
| Stop App | `docker-compose down` | COMPOSE_PROJECT_NAME in .env |
| View Logs | `docker-compose logs` | COMPOSE_PROJECT_NAME in .env |
| List Containers | `docker-compose ps` | COMPOSE_PROJECT_NAME in .env |

### Agent Monitoring

The OnTree agent monitors applications using the `app.yaml` configuration:

```yaml
id: mywebapp
name: My Web Application
primary_service: web
expected_services:
  - ontree-mywebapp-web-1
  - ontree-mywebapp-db-1
```

The agent:
- Checks if `expected_services` containers are running
- Monitors container health and restart counts
- Can restart containers that have failed
- Reports status through the chat interface

### Initial Setup for Templates

When creating apps from templates, OnTree sets `initial_setup_required: true` in `app.yaml`. This triggers the agent to:

1. Pull the latest Docker images
2. Lock specific version tags in `docker-compose.yml`
3. Remove the `initial_setup_required` flag
4. Report progress through the chat interface

### Docker API Access

OnTree requires access to the Docker daemon. Configure this based on your setup:

**Local Docker:**
```toml
docker_socket = "/var/run/docker.sock"
```

**Remote Docker:**
```toml
docker_host = "tcp://docker-host:2375"
docker_cert_path = "/path/to/certs"
docker_tls_verify = true
```

**Docker in Docker:**
```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock
```

### Container Resource Limits

You can set default resource limits for all OnTree-managed containers:

```toml
[docker.defaults]
memory_limit = "512m"
cpu_limit = "1.0"
restart_policy = "unless-stopped"
```

These defaults are applied when creating new applications but can be overridden in individual `docker-compose.yml` files.

### Monitoring Settings

#### `monitoring_enabled`
- **Type**: Boolean
- **Default**: `true`
- **Description**: Enable system monitoring
- **Environment**: `MONITORING_ENABLED`

#### `monitoring_interval`
- **Type**: Integer
- **Default**: `60`
- **Description**: Metrics collection interval (seconds)
- **Environment**: `MONITORING_INTERVAL`

#### `monitoring_retention_days`
- **Type**: Integer
- **Default**: `7`
- **Description**: Days to retain monitoring data
- **Environment**: `MONITORING_RETENTION_DAYS`

### Domain Settings

#### `public_base_domain`
- **Type**: String
- **Default**: `""`
- **Description**: Public domain for app exposure
- **Environment**: `PUBLIC_BASE_DOMAIN`
- **Example**: `"example.com"`

#### `tailscale_base_domain`
- **Type**: String
- **Default**: `""`
- **Description**: Tailscale domain for private access
- **Environment**: `TAILSCALE_BASE_DOMAIN`
- **Example**: `"machine.tail-scale.ts.net"`

#### `caddy_admin_url`
- **Type**: String
- **Default**: `"http://localhost:2019"`
- **Description**: Caddy admin API endpoint
- **Environment**: `CADDY_ADMIN_URL`

### Security Settings

#### `session_secret`
- **Type**: String
- **Default**: Auto-generated
- **Description**: Secret for session encryption
- **Environment**: `SESSION_SECRET`
- **Important**: Set to random string in production

#### `cors_allowed_origins`
- **Type**: String array
- **Default**: `[]`
- **Description**: Allowed CORS origins
- **Environment**: `CORS_ALLOWED_ORIGINS` (comma-separated)

### Feature Flags

#### `enable_monitoring`
- **Type**: Boolean
- **Default**: `true`
- **Description**: Enable monitoring dashboard
- **Environment**: `ENABLE_MONITORING`

#### `enable_templates`
- **Type**: Boolean
- **Default**: `true`
- **Description**: Enable template system
- **Environment**: `ENABLE_TEMPLATES`

#### `enable_compose_editor`
- **Type**: Boolean
- **Default**: `true`
- **Description**: Enable YAML editor
- **Environment**: `ENABLE_COMPOSE_EDITOR`

### Logging Settings

#### `log_level`
- **Type**: String
- **Default**: `"info"`
- **Description**: Minimum log level
- **Environment**: `LOG_LEVEL`
- **Options**: `"debug"`, `"info"`, `"warn"`, `"error"`

#### `log_format`
- **Type**: String
- **Default**: `"json"`
- **Description**: Log output format
- **Environment**: `LOG_FORMAT`
- **Options**: `"json"`, `"text"`

## Advanced Configuration

### Custom Templates Directory

```toml
templates_directory = "./custom-templates"
template_repos = [
    "https://github.com/org/templates.git",
    "/local/path/to/templates"
]
```

### Resource Limits

```toml
[limits]
max_apps = 100
max_container_log_size = "100MB"
max_operation_log_entries = 1000
```

### Background Workers

```toml
[workers]
pool_size = 10
queue_size = 100
operation_timeout = 300  # seconds
```

## Configuration Examples

### Production Setup

```toml
# Production configuration
port = 443
host = "0.0.0.0"
base_url = "https://ontree.company.com"

# Database
database_path = "/var/lib/ontree/production.db"
database_max_connections = 50

# Security
session_secret = "very-long-random-string-here"
cors_allowed_origins = ["https://app.company.com"]

# Domains
public_base_domain = "apps.company.com"
caddy_admin_url = "http://caddy:2019"

# Logging
log_level = "warn"
log_format = "json"

# Monitoring
monitoring_enabled = true
monitoring_retention_days = 30
```

### Development Setup

```toml
# Development configuration
port = 3000
host = "127.0.0.1"
base_url = "http://localhost:3000"

# Enable all features
enable_monitoring = true
enable_templates = true
enable_compose_editor = true

# Verbose logging
log_level = "debug"
log_format = "text"

# Faster monitoring
monitoring_interval = 10
```

### Docker Deployment

```yaml
# docker-compose.yml
version: '3.8'

services:
  ontree:
    image: ontree/ontree:latest
    environment:
      - PORT=8080
      - DATABASE_PATH=/data/ontree.db
      - APPS_DIRECTORY=/apps
      - PUBLIC_BASE_DOMAIN=example.com
      - LOG_LEVEL=info
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./data:/data
      - ./apps:/apps
    ports:
      - "8080:8080"
```

## Validation

OnTree validates configuration on startup:

- **Required fields** - Ensures critical settings exist
- **Type checking** - Verifies correct data types
- **Path validation** - Checks file/directory accessibility
- **Connection tests** - Verifies Docker and Caddy access

Invalid configuration prevents startup with clear error messages.