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

### Option A: Using Your Own Woodpecker Instance (Recommended)

You can host your own Woodpecker instance and connect it directly to Codeberg without any approval process.

#### 1. Set Up Self-Hosted Woodpecker Server and Agent

Create a `docker-compose.woodpecker.yml` file for running both server and agent:

```yaml
version: '3'

services:
  woodpecker-server:
    image: woodpeckerci/woodpecker-server:latest
    restart: unless-stopped
    ports:
      - "8000:8000"
    environment:
      - WOODPECKER_HOST=https://your-woodpecker-domain.com
      - WOODPECKER_ADMIN=your-codeberg-username
      - WOODPECKER_AGENT_SECRET=<GENERATE_RANDOM_SECRET>
      - WOODPECKER_FORGE=forgejo
      - WOODPECKER_FORGEJO_URL=https://codeberg.org
      - WOODPECKER_FORGEJO_CLIENT=<OAUTH_CLIENT_ID>
      - WOODPECKER_FORGEJO_SECRET=<OAUTH_CLIENT_SECRET>
      - WOODPECKER_DATABASE_DRIVER=sqlite3
      - WOODPECKER_DATABASE_DATASOURCE=/var/lib/woodpecker/woodpecker.db
      - WOODPECKER_LOG_LEVEL=info
    volumes:
      - woodpecker-server-data:/var/lib/woodpecker
    networks:
      - woodpecker

  woodpecker-agent:
    image: woodpeckerci/woodpecker-agent:latest
    restart: unless-stopped
    environment:
      - WOODPECKER_SERVER=woodpecker-server:9000
      - WOODPECKER_AGENT_SECRET=<SAME_SECRET_AS_ABOVE>
      - WOODPECKER_FILTER_LABELS=platform=linux/amd64,type=docker
      - WOODPECKER_MAX_WORKFLOWS=4
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /tmp/woodpecker:/tmp
    depends_on:
      - woodpecker-server
    networks:
      - woodpecker

volumes:
  woodpecker-server-data:

networks:
  woodpecker:
    driver: bridge
```

#### 2. Configure OAuth Application on Codeberg

1. Go to https://codeberg.org/user/settings/applications
2. Create a new OAuth2 Application with:
   - Application Name: `Woodpecker CI`
   - Homepage URL: `https://your-woodpecker-domain.com`
   - Redirect URI: `https://your-woodpecker-domain.com/authorize`
3. Save the Client ID and Client Secret for the docker-compose file

#### 3. Start Your Woodpecker Instance

```bash
# Generate a secure agent secret
export AGENT_SECRET=$(openssl rand -hex 32)

# Update docker-compose with your values
# Then start the services
docker-compose -f docker-compose.woodpecker.yml up -d
```

#### 4. Add Your Repository

1. Visit your Woodpecker instance at `https://your-woodpecker-domain.com`
2. Login with your Codeberg account
3. Enable your repository in the Woodpecker UI

#### 5. Configure Secrets

Add these secrets in your Woodpecker UI:

- `github_token` - GitHub Personal Access Token for releases
- `public_releases_token` - Token for publishing to public repository
- `codecov_token` - Codecov token for coverage reports

### Option B: Using Codeberg's Shared CI (Requires Approval)

If you prefer to use Codeberg's shared infrastructure:

1. Request access by filling out the [Codeberg CI form](https://docs.codeberg.org/ci/)
2. Once approved, add your repository at https://ci.codeberg.org/repos/add
3. Login with your Codeberg account
4. Configure secrets in the Woodpecker UI

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