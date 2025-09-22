# Woodpecker CI Configuration for TreeOS

This directory contains Woodpecker CI pipeline configurations for the TreeOS project on Codeberg.

## Pipeline Files

- **test.yml** - Main test pipeline that runs on pushes to main, develop, and release branches
  - Unit tests with coverage
  - Linting with golangci-lint
  - Build verification
  - E2E tests with Playwright
  - Documentation build and typecheck

- **pull_request.yml** - Lightweight pipeline for pull requests
  - Quick syntax checks
  - Linting
  - Unit tests
  - Build verification
  - Documentation typecheck

- **release.yml** - Release automation pipeline (triggered on version tags)
  - Tests verification
  - Cross-platform builds with GoReleaser
  - GitHub release creation
  - Public repository publishing

## Setup Instructions

### 1. Enable Codeberg CI

1. Request access by filling out the [Codeberg CI form](https://docs.codeberg.org/ci/)
2. Once approved, add your repository at https://ci.codeberg.org/repos/add
3. Login with your Codeberg account

### 2. Configure Secrets

Add these secrets in the Woodpecker UI:

- `github_token` - GitHub Personal Access Token for releases
- `public_releases_token` - Token for publishing to public repository
- `codecov_token` - Codecov token for coverage reports

### 3. Set Up Self-Hosted Runner

Since Codeberg CI only provides linux/amd64 runners, you'll need to set up a self-hosted runner for:
- ARM64 builds
- Resource-intensive E2E tests
- Better caching and performance

#### Docker-based Runner Setup

```bash
# Create docker-compose.yml for Woodpecker agent
cat > docker-compose.woodpecker.yml << 'EOF'
version: '3'

services:
  woodpecker-agent:
    image: woodpeckerci/woodpecker-agent:latest
    restart: unless-stopped
    environment:
      - WOODPECKER_SERVER=https://ci.codeberg.org
      - WOODPECKER_AGENT_SECRET=<YOUR_AGENT_SECRET>
      - WOODPECKER_FILTER_LABELS=platform=linux/amd64,type=docker
      - WOODPECKER_MAX_WORKFLOWS=2
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /tmp/woodpecker:/tmp
    networks:
      - woodpecker

networks:
  woodpecker:
    driver: bridge
EOF

# Start the agent
docker-compose -f docker-compose.woodpecker.yml up -d
```

### 4. Pipeline Triggers

- **Push to branches**: Runs full test suite
- **Pull requests**: Runs lightweight checks
- **Version tags (v*.*.*)**: Triggers release pipeline

## Differences from GitHub Actions

### Syntax Changes
- No `uses: actions/*` - replaced with Docker images and shell commands
- Secrets accessed via `from_secret` syntax
- Dependencies between steps use `depends_on`
- Caching implemented via volumes

### Feature Adaptations
- Artifact uploads → External storage or Codeberg releases
- Matrix strategies → Multiple pipeline steps
- Node version from file → Direct commands
- GitHub-specific features → Woodpecker equivalents

## Troubleshooting

### Common Issues

1. **Pipeline not triggering**
   - Verify repository is enabled in Woodpecker UI
   - Check branch/event filters in pipeline configuration

2. **Cross-compilation failures**
   - Ensure gcc-aarch64-linux-gnu is installed
   - Check GOARCH and CGO settings

3. **Cache not working**
   - Verify volume mounts in agent configuration
   - Check GOCACHE and GOMODCACHE paths

## Resources

- [Woodpecker CI Documentation](https://woodpecker-ci.org/docs/intro)
- [Codeberg CI Examples](https://codeberg.org/Codeberg-CI/examples)
- [Woodpecker CLI](https://woodpecker-ci.org/docs/usage/cli) for local testing