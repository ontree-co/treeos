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
- Port 8080 available (or configure a different port)

### Running OnTree

1. Download the latest release for your platform from [GitHub Releases](https://github.com/yourusername/ontree-node/releases)

2. Make the binary executable:
   ```bash
   chmod +x ontree-server
   ```

3. Run OnTree:
   ```bash
   ./ontree-server
   ```

4. Open your browser to `http://localhost:8080`

### Configuration

OnTree can be configured using environment variables:

- `PORT`: HTTP server port (default: 8080)
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
   git clone https://github.com/yourusername/ontree-node.git
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

For issues, questions, or contributions, please visit our [GitHub repository](https://github.com/yourusername/ontree-node).