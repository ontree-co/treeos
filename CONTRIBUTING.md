# Contributing to TreeOS

Thanks for your interest in contributing! We welcome improvements, bug fixes, and ideas from the community.

## How to Contribute

### Reporting Issues

- Check if the issue already exists.
- Use issue templates when available.
- Include reproduction steps and expected vs. actual behavior.
- Share environment details (OS, Go version, browser for UI issues).

### Pull Requests

1. Fork the repository and create a feature branch.
2. Make your changes.
3. Run tests and linting: `make test && make lint`
4. Run end-to-end tests: `make test-e2e`
5. Commit with clear messages.
6. Push to your fork and open a pull request.

### Development Setup

1. Install Go 1.24 or later.
2. Clone the repository.
3. Install dependencies: `go mod download`
4. Build the app: `make build`
5. Run locally: `make run` (or `make dev` for hot reload if you have `wgo` installed).

### Code Style

- Use standard Go formatting (`gofmt`).
- Run `make lint` (golangci-lint) before submitting.
- Keep functions focused and small; add comments where clarity is needed.

### Testing

- Unit/integration tests: `make test`
- Race checks: `make test-race`
- Coverage: `make test-coverage`
- End-to-end tests (requires Node/Playwright): `make test-e2e`

### Commit Messages

- Start with a verb (Add, Fix, Update, Remove, etc.).
- Keep the first line under ~50 characters; add detail in the body if needed.

### Questions?

- Open an issue with your question or proposal.
