# TreeOS

OnTree is a Docker container management application with a web interface for managing containerized applications. It provides an intuitive UI for creating, managing, and monitoring Docker containers on your local system or server.

## Features

- **Web-based Docker Management**: Manage Docker containers through a clean web interface
- **Real-time Operation Logging**: View detailed logs of all container operations in real-time
- **Container Templates**: Pre-configured templates for popular applications (PostgreSQL, Redis, etc.)
- **Dynamic UI Updates**: Real-time status updates using HTMX without page refreshes
- **Self-contained Binary**: Single executable with all assets embedded
- **Multi-architecture Support**: Runs on Linux (AMD64) and macOS (ARM64)
- **System Monitoring Dashboard**: Real-time system metrics with historical sparklines (CPU, Memory, Disk, Network)

## Quick Start

### Prerequisites

- Docker installed and running
- Port 3000 available (or configure a different port)

### Running OnTree

1. Download the latest release for your platform from [GitHub Releases](https://github.com/stefanmunz/treeos/releases)

2. Make the binary executable:

   ```bash
   chmod +x treeos
   ```

3. Run OnTree:

   ```bash
   ./treeos
   ```

4. Open your browser to `http://localhost:3000`

### Configuration

OnTree can be configured using environment variables:

- `LISTEN_ADDR`: HTTP server listen address (default: :3000)
- `DATABASE_PATH`: Path to SQLite database (default: `./ontree.db`)
- `AUTH_USERNAME`: Basic auth username (required)
- `AUTH_PASSWORD`: Basic auth password (required)
- `SESSION_KEY`: Session encryption key (auto-generated if not set)
- `MONITORING_ENABLED`: Enable/disable system monitoring dashboard (default: true)
- `PUBLIC_BASE_DOMAIN`: Public domain for app exposure via Caddy
- `TAILSCALE_BASE_DOMAIN`: Tailscale domain for app exposure via Caddy

Example:

```bash
export AUTH_USERNAME="admin"
export AUTH_PASSWORD="secure-password"
export PORT="3000"
./treeos
```

## System Monitoring

OnTree includes a comprehensive system monitoring dashboard that provides real-time insights into your server's performance.

### Features

- **Real-time Metrics**: View current CPU, Memory, Disk, and Network usage
- **Historical Sparklines**: 24-hour trend visualization for each metric
- **Detailed Charts**: Click any sparkline to view detailed charts with multiple time ranges
- **Auto-refresh**: Metrics update every 5 seconds automatically
- **Responsive Design**: Optimized for both desktop and mobile viewing

### Accessing the Dashboard

1. Navigate to `/monitoring` in your OnTree instance
2. The dashboard displays a 2x2 grid of metric cards
3. Click any sparkline to see detailed historical data

### Time Ranges

Detailed charts support multiple time ranges:

- 1 Hour - For immediate performance troubleshooting
- 6 Hours - For recent trend analysis
- 24 Hours - Default view showing daily patterns
- 7 Days - For weekly trend analysis

### Data Retention

- System metrics are collected every 60 seconds
- Historical data is retained for 7 days
- Older data is automatically cleaned up to save space

### Performance Optimization

The monitoring system includes several optimizations:

- 5-minute caching for sparkline generation
- Batch database queries for efficiency
- Optimized SVG generation for fast rendering
- Connection pooling for database access

### Configuration

To disable monitoring (if needed for performance reasons):

```bash
export MONITORING_ENABLED=false
./treeos
```

Or in `config.toml`:

```toml
monitoring_enabled = false
```

## Development

### Prerequisites

- Go 1.23 or later
- Docker
- Make

### Building from Source

1. Clone the repository:

   ```bash
   git clone https://github.com/stefanmunz/treeos.git
   cd treeos
   ```

2. Install dependencies:

   ```bash
   go mod download
   ```

3. Build the application:
   ```bash
   make build
   ```

### Running Tests

OnTree includes comprehensive test suites for unit, integration, and end-to-end testing.

```bash
# Run unit and integration tests
make test

# Run tests with race detector
make test-race

# Run tests with coverage report
make test-coverage

# Run Playwright E2E tests
make test-e2e

# Run all tests (unit, integration, and E2E)
make test-all

# Run linting
make lint
```

#### E2E Testing Details

The E2E test suite uses Playwright to test the application through real browser interactions:

- **Automatic Setup**: The `make test-e2e` command automatically:

  - Builds the application if needed
  - Starts the server on port 3001
  - Waits for the server to be ready
  - Runs the Playwright test suite
  - Generates HTML reports in `tests/e2e/playwright-report/`

- **Test Coverage**: E2E tests cover:

  - Authentication flows (login/logout)
  - Dashboard functionality and system vitals
  - Simple app lifecycle (create, start, stop, delete)
  - Complex multi-service app lifecycle
  - Docker container naming conventions

- **Environment Variables**: Configure E2E tests with:
  - `TEST_BASE_URL`: Base URL for tests (default: http://localhost:3001)
  - `TEST_PORT`: Port for test server (default: 3001)
  - `KEEP_SERVER_RUNNING`: Keep server running after tests (default: false)
  - `KEEP_TEST_DATA`: Preserve test data after cleanup (default: false)

### Development Mode

For development with hot-reloading:

```bash
go run cmd/treeos/main.go
```

### Template Syntax Checking

OnTree includes a template syntax checker to catch errors before runtime:

```bash
# Check all templates
make check-templates

# Template checking runs automatically during build
make build
```

The template checker validates:

- All HTML templates against the base template
- Template syntax (proper opening/closing of blocks)
- Component templates for standalone validity

Template checking is enforced in CI to prevent broken templates from being merged.

## Building Releases

OnTree uses [GoReleaser](https://goreleaser.com/) for automated release builds. The release process creates self-contained binaries with all assets embedded.

### Creating a Release

1. **Tag your release**:

   ```bash
   git tag -a v1.0.0 -m "Release version 1.0.0"
   git push origin v1.0.0
   ```

2. **Automated GitHub Release**:
   - GitHub Actions automatically builds releases when tags matching `v*.*.*` are pushed
   - The workflow runs tests first, then creates platform-specific binaries
   - Release notes are auto-generated from commit messages

### Local Release Build

To build a release locally (useful for testing):

1. **Install GoReleaser**:

   ```bash
   go install github.com/goreleaser/goreleaser/v2@latest
   ```

2. **Build a snapshot release** (without publishing):

   ```bash
   goreleaser build --snapshot --clean
   ```

3. **Build for all platforms**:
   ```bash
   make build-all
   ```

### Release Artifacts

Each release includes:

- `treeos_VERSION_linux_x86_64.tar.gz` - Linux AMD64 binary
- `treeos_VERSION_darwin_arm64.tar.gz` - macOS Apple Silicon binary
- `checksums.txt` - SHA256 checksums for verification

### Binary Features

- **Self-contained**: All templates and static assets are embedded
- **No dependencies**: Statically linked, no external libraries required
- **Version info**: Build version, commit, and date embedded in binary
- **Cross-platform**: Supports Linux (AMD64) and macOS (ARM64)

### Manual Build Process

For manual builds without GoReleaser:

```bash
# Ensure assets are embedded
make embed-assets

# Build for current platform
make build

# Build for specific platform
GOOS=linux GOARCH=amd64 make build
GOOS=darwin GOARCH=arm64 make build
```

The built binary will be in the `build/` directory.

## Architecture

- **Backend**: Go with embedded HTTP server
- **Frontend**: HTMX + Bootstrap for dynamic UI
- **Database**: SQLite for metadata storage
- **Container Management**: Direct Docker API integration
- **Asset Embedding**: Templates and static files embedded in binary

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For issues, questions, or contributions, please visit our [GitHub repository](https://github.com/stefanmunz/treeos).
