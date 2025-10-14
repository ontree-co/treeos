# TreeOS

[![Build Status](https://ci.codeberg.org/api/badges/ontree/treeos/status.svg)](https://ci.codeberg.org/ontree/treeos)
[![GitHub Actions](https://github.com/stefanmunz/treeos/actions/workflows/test.yml/badge.svg)](https://github.com/stefanmunz/treeos/actions)

## What is TreeOS?

TreeOS is server software for your home lab or office—it puts AI on your site, both literally on your infrastructure and figuratively at your side. Builders get a single place to run their services and the models that power them, all from the browser without touching the command line.

TreeOS is a platform for shipping containers alongside local LLMs:

- **🚀 One-Click Deployments** – Launch production-ready stacks from polished templates or remix them into your own projects.
- **🎛️ Polished Interface** – A cohesive web dashboard for editing Compose files, streaming logs, and managing environments directly in the browser.
- **🧠 Model-Native Platform** – Treat models as first-class citizens. Pull curated chat, code, and vision models, watch download progress live, and plug them straight into your apps.
- **🛠️ Built for Builders** – Tailored to people who want to bring their own code, keep data on their hardware, and iterate quickly.
- **⚙️ Optimized for AMD AI 300 Series** – Designed to leverage modern APUs with up to 128 GB of RAM so local inference and application hosting share the same machine gracefully.

## Architecture

- **Backend**: Go with embedded HTTP server
- **Frontend**: HTMX + Bootstrap for dynamic UI
- **Database**: SQLite for metadata storage
- **Container Management**: Docker and Docker Compose
- **Asset Embedding**: Templates and static files embedded in binary

## Features

- **Web-based Container Management**: Manage Docker containers through a clean web interface
- **Real-time Operation Logging**: View detailed logs of all container operations in real-time
- **Application Templates**: Pre-configured templates for popular applications (PostgreSQL, Redis, Ollama, etc.)
- **Model Management**: Install and manage AI models (Ollama integration)
- **Dynamic UI Updates**: Real-time status updates using HTMX without page refreshes
- **Self-contained Binary**: Single executable with all assets embedded
- **Multi-architecture Support**: Runs on Linux (AMD64) and macOS (ARM64)
- **System Monitoring Dashboard**: Real-time system metrics with historical sparklines (CPU, Memory, Disk, Network)
- **Security Validation**: Built-in checks for container security (mounts, capabilities, privilege levels)
- **Caddy Integration**: Automatic domain configuration for exposed applications

## Quick Start

### Production Setup (with Binaries)

The fastest way to set up TreeOS in production mode (as a system service with automatic startup):

**Using Claude Code:**

```
/treeos-setup-production
```

This command will:

- Download the latest TreeOS binary for your platform
- Create the `ontree` system user
- Set up the directory structure in `/opt/ontree/`
- Install and configure the system service (systemd on Linux, launchd on macOS)
- Optionally install AMD ROCm drivers if an AMD GPU is detected

**Note**: This requires sudo privileges. Claude Code will guide you through running the setup script manually if needed.

**Manual Installation:**

If you prefer to run the setup script directly:

```bash
cd /path/to/treeos
sudo ./.claude/commands/treeos-setup-production-noconfirm.sh
```

Once installed, access TreeOS at `http://localhost:3000`

### Development Setup

For development or trying out TreeOS without installing as a system service:

**Using Claude Code:**

```
/treeos-setup-development-environment
```

This command will:

- Install asdf version manager (if not already installed)
- Install Go 1.24.4, golangci-lint 2.5.0, and Node.js 22.11.0
- Configure your shell for automatic version switching

**Development Mode** (Local Testing):

TreeOS has a development mode where everything runs locally without requiring system installation:

```bash
# Clone repository
git clone https://github.com/stefanmunz/treeos.git
cd treeos

# Copy environment template
cp .env.example .env

# Build and run in development mode
make build
./build/treeos --demo
```

Development mode creates local directories (`./apps`, `./shared`, `./logs`) and uses a local database (`./ontree.db`). This is ideal for:

- Developers working on TreeOS
- Tech-savvy users wanting to try it out
- Testing before production deployment

**Prerequisites:**

- Docker and Docker Compose installed
- Go 1.24.4 (managed via asdf)
- Port 3000 available (or configure a different port)

## CI and Development

### CI Process

TreeOS uses GitHub Actions for continuous integration. The CI workflow runs:

1. **Build** - Compiles the application with embedded assets
2. **Test** - Runs unit and integration tests with race detection
3. **Lint** - Runs golangci-lint for code quality
4. **E2E Tests** - Runs Playwright browser tests
5. **Documentation** - Builds and validates documentation

### Running Tests Locally

Before pushing changes, run these commands to ensure your code will pass CI:

```bash
# Build the application
make build

# Run unit and integration tests
make test

# Run linter
make lint

# Run end-to-end tests
make test-e2e

# Run all tests at once
make test-all
```

**Important**: TreeOS has a pre-commit hook that automatically runs tests and linting. Never bypass this hook with `--no-verify` unless absolutely necessary.

### Development Commands

```bash
# Format code
make fmt

# Run go vet
make vet

# Check template syntax
make check-templates

# Generate coverage report
make test-coverage

# Run with hot reload (development)
go run cmd/treeos/main.go
```

## Release Management

TreeOS uses semantic versioning with support for both beta and stable releases.

### Creating a Release

Use the `/treeos-release-version` command in Claude Code:

```
/treeos-release-version <type>
```

## Contributing

We welcome contributions to TreeOS!

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes and ensure tests pass (`make test-all`)
4. Commit your changes (`git commit -m 'Add amazing feature'`)
5. Push to the branch (`git push origin feature/amazing-feature`)
6. Open a Pull Request

**Development Guidelines:**

- Follow the project's coding standards (enforced by golangci-lint)
- Add tests for new features
- Update documentation as needed
- Ensure all CI checks pass before requesting review

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For issues, questions, or contributions, please visit our [GitHub repository](https://github.com/stefanmunz/treeos).

**Documentation**: For detailed documentation, configuration reference, and guides, visit the `documentation/` directory or view the docs at https://docs.treeos.com
