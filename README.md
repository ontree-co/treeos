# OnTree

OnTree is a Docker container management application with a web interface for managing containerized applications. It provides an intuitive UI for creating, managing, and monitoring Docker containers on your local system or server.

## Features

- **Web-based Docker Management**: Manage Docker containers through a clean web interface
- **Real-time Operation Logging**: View detailed logs of all container operations in real-time
- **Container Templates**: Pre-configured templates for popular applications (PostgreSQL, Redis, etc.)
- **Dynamic UI Updates**: Real-time status updates using HTMX without page refreshes
- **Self-contained Binary**: Single executable with all assets embedded
- **Multi-architecture Support**: Runs on Linux (AMD64) and macOS (ARM64)

## Quick Start

### Prerequisites

- Docker installed and running
- Port 3000 available (or configure a different port)

### Running OnTree

1. Download the latest release for your platform from [GitHub Releases](https://github.com/stefanmunz/ontree-node/releases)

2. Make the binary executable:
   ```bash
   chmod +x ontree-server
   ```

3. Run OnTree:
   ```bash
   ./ontree-server
   ```

4. Open your browser to `http://localhost:3000`

### Configuration

OnTree can be configured using environment variables:

- `LISTEN_ADDR`: HTTP server listen address (default: :3000)
- `DATABASE_PATH`: Path to SQLite database (default: `./ontree.db`)
- `AUTH_USERNAME`: Basic auth username (required)
- `AUTH_PASSWORD`: Basic auth password (required)
- `SESSION_KEY`: Session encryption key (auto-generated if not set)

Example:
```bash
export AUTH_USERNAME="admin"
export AUTH_PASSWORD="secure-password"
export PORT="3000"
./ontree-server
```

## Development

### Prerequisites

- Go 1.23 or later
- Docker
- Make

### Building from Source

1. Clone the repository:
   ```bash
   git clone https://github.com/stefanmunz/ontree-node.git
   cd ontree-node
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

```bash
# Run unit tests
make test

# Run E2E tests
make test-e2e

# Run linting
make lint
```

### Development Mode

For development with hot-reloading:
```bash
go run cmd/ontree-server/main.go
```

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
- `ontree-server_VERSION_linux_x86_64.tar.gz` - Linux AMD64 binary
- `ontree-server_VERSION_darwin_arm64.tar.gz` - macOS Apple Silicon binary
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

For issues, questions, or contributions, please visit our [GitHub repository](https://github.com/stefanmunz/ontree-node).