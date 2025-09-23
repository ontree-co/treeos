# Woodpecker CI Integration for TreeOS

## Overview

This document describes the planned integration of Woodpecker CI with TreeOS for self-hosted continuous integration/continuous deployment (CI/CD) with Codeberg repositories.

## Current Status: Not Yet Implemented

**Important**: Woodpecker CI template has been temporarily removed from TreeOS apps. This integration requires additional infrastructure setup that will be addressed in a future release.

## Key Findings and Requirements

### Infrastructure Requirements

1. **Public IP Address Required**
   - Woodpecker CI relies on webhooks from Codeberg to trigger builds
   - When code is pushed to Codeberg, it sends HTTP POST requests to your Woodpecker instance
   - This means Woodpecker MUST be publicly accessible on the internet
   - The warning `"WOODPECKER_HOST should probably be publicly accessible (not localhost)"` confirms this

2. **No Built-in Polling Alternative**
   - Unlike some CI systems, Woodpecker does not offer polling as an alternative to webhooks
   - Webhooks are the primary and only automatic trigger mechanism

### Tested Configuration

The following setup was tested and confirmed working:

#### Docker Containers
- **Server**: `woodpeckerci/woodpecker-server:v3` (Port 8000)
- **Agent**: `woodpeckerci/woodpecker-agent:v3` (Healthy status)
- **Agent Secret**: Successfully configured for agent-server communication

#### Pipeline Files
Working CI pipeline files exist in `.woodpecker/`:
- `test.yml` - Comprehensive test pipeline (unit tests, linting, E2E, coverage)
- `pull_request.yml` - Quick PR checks
- `release.yml` - Release automation

## Solutions for Non-Public IP Environments

### Option 1: Tailscale Funnel (Recommended for Testing)

Tailscale Funnel can expose your local Woodpecker instance with a public HTTPS URL.

**Setup Steps**:
1. Install and configure Tailscale
2. Enable Funnel: `tailscale funnel 8000`
3. Your Woodpecker will be accessible at: `https://[your-machine].shorthair-neon.ts.net`
4. Use this URL for Codeberg OAuth redirect URI

**Pros**:
- Free for personal use
- Provides stable HTTPS URL
- No port forwarding needed

**Cons**:
- Service must be kept running
- Publicly exposes your service (OAuth still required for access)

### Option 2: Split Architecture

Run components separately:
- **Woodpecker Server**: On a VPS with public IP (lightweight, receives webhooks)
- **Woodpecker Agent**: On local machine (does the heavy CI work)

**Pros**:
- Only lightweight server needs public access
- Agents can be scaled locally

**Cons**:
- Requires a VPS subscription
- More complex setup

### Option 3: Other Tunneling Services

Alternative tunneling options:
- **ngrok**: `ngrok http 8000` (temporary URLs in free tier)
- **Cloudflare Tunnel**: More stable, requires Cloudflare account
- **localtunnel**: Open source alternative

## Woodpecker Template Configuration

### Docker Compose Template (`woodpecker_ci/docker-compose.yml`)

```yaml
version: '3.8'

services:
  woodpecker-server:
    image: woodpeckerci/woodpecker-server:v3
    container_name: ${COMPOSE_PROJECT_NAME}${COMPOSE_SEPARATOR}woodpecker-server
    ports:
      - "${HOST_PORT:-8000}:8000"
    environment:
      - WOODPECKER_HOST=${WOODPECKER_HOST}
      - WOODPECKER_ADMIN=${WOODPECKER_ADMIN}
      - WOODPECKER_OPEN=${WOODPECKER_OPEN:-false}
      - WOODPECKER_AGENT_SECRET=${WOODPECKER_AGENT_SECRET}
      # Codeberg/Forgejo Configuration
      - WOODPECKER_FORGEJO=${WOODPECKER_FORGEJO:-true}
      - WOODPECKER_FORGEJO_URL=${WOODPECKER_FORGEJO_URL:-https://codeberg.org}
      - WOODPECKER_FORGEJO_CLIENT=${WOODPECKER_FORGEJO_CLIENT}
      - WOODPECKER_FORGEJO_SECRET=${WOODPECKER_FORGEJO_SECRET}
      # Database
      - WOODPECKER_DATABASE_DRIVER=sqlite3
      - WOODPECKER_DATABASE_DATASOURCE=/data/woodpecker.db
    volumes:
      - woodpecker-server-data:/data
    networks:
      - woodpecker
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8000/healthz"]
      interval: 30s
      timeout: 10s
      retries: 3

  woodpecker-agent:
    image: woodpeckerci/woodpecker-agent:v3
    container_name: ${COMPOSE_PROJECT_NAME}${COMPOSE_SEPARATOR}woodpecker-agent
    depends_on:
      - woodpecker-server
    environment:
      - WOODPECKER_SERVER=woodpecker-server:9000
      - WOODPECKER_AGENT_SECRET=${WOODPECKER_AGENT_SECRET}
      - WOODPECKER_MAX_WORKFLOWS=${WOODPECKER_MAX_WORKFLOWS:-2}
      - WOODPECKER_BACKEND=docker
      - WOODPECKER_BACKEND_DOCKER_NETWORK=woodpecker
      - WOODPECKER_LOG_LEVEL=${WOODPECKER_LOG_LEVEL:-info}
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    networks:
      - woodpecker
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "woodpecker-agent", "ping"]
      interval: 30s
      timeout: 10s
      retries: 3

networks:
  woodpecker:
    name: ${COMPOSE_PROJECT_NAME}${COMPOSE_SEPARATOR}network

volumes:
  woodpecker-server-data:
    name: ${COMPOSE_PROJECT_NAME}${COMPOSE_SEPARATOR}server-data
```

### Environment Template (`woodpecker_ci/.env.example`)

```env
# REQUIRED: TreeOS compose configuration
COMPOSE_PROJECT_NAME=ontree-woodpecker
COMPOSE_SEPARATOR=-

# REQUIRED: Your Woodpecker instance URL (must be publicly accessible)
# Examples:
# - With public IP: http://your-domain.com:8000
# - With Tailscale Funnel: https://your-machine.ts.net
# - With ngrok: https://abc123.ngrok.io
WOODPECKER_HOST=

# REQUIRED: Your Codeberg username (will be admin)
WOODPECKER_ADMIN=

# User registration (set to true if you want others to register)
WOODPECKER_OPEN=false

# REQUIRED: Codeberg OAuth credentials
# Get these from: https://codeberg.org/user/settings/applications
# Redirect URI must be: ${WOODPECKER_HOST}/authorize
WOODPECKER_FORGEJO=true
WOODPECKER_FORGEJO_URL=https://codeberg.org
WOODPECKER_FORGEJO_CLIENT=
WOODPECKER_FORGEJO_SECRET=

# REQUIRED: Security token for agent-server communication
# Generate with: openssl rand -hex 32
WOODPECKER_AGENT_SECRET=

# Agent settings
WOODPECKER_MAX_WORKFLOWS=2
WOODPECKER_LOG_LEVEL=info

# Optional: Use PostgreSQL instead of SQLite
# WOODPECKER_DATABASE_DRIVER=postgres
# WOODPECKER_DATABASE_DATASOURCE=postgres://user:password@postgres:5432/woodpecker?sslmode=disable
```

## Implementation Checklist

When ready to implement:

### Prerequisites
- [ ] Ensure server has public IP OR tunneling solution configured
- [ ] Have Codeberg account with repository to test

### Setup Steps
1. [ ] **Configure Public Access**
   - Set up public IP/domain OR
   - Configure tunneling service (Tailscale Funnel, ngrok, etc.)

2. [ ] **Create Codeberg OAuth App**
   - Go to: https://codeberg.org/user/settings/applications
   - Application Name: `TreeOS Woodpecker CI`
   - Redirect URI: `[YOUR_PUBLIC_URL]/authorize`
   - Save Client ID and Secret

3. [ ] **Configure Woodpecker**
   - Copy `.env.example` to `.env`
   - Fill in all required values
   - Ensure `WOODPECKER_AGENT_SECRET` is generated

4. [ ] **Enable in TreeOS**
   - Add Woodpecker template back to embedded templates
   - Ensure Docker socket mount is allowed (security bypass required)

5. [ ] **Start Services**
   - Deploy via TreeOS interface
   - Verify both server and agent are healthy

6. [ ] **Connect Repository**
   - Access Woodpecker UI
   - Login via Codeberg OAuth
   - Add repository
   - Verify webhook created in Codeberg

7. [ ] **Test Pipeline**
   - Push code to trigger build
   - Verify agent executes jobs

## Security Considerations

1. **Docker Socket Access**: Woodpecker agent requires `/var/run/docker.sock` mount
   - This gives container full Docker control
   - Must enable "Bypass security validation" in TreeOS

2. **Public Exposure**: With tunneling, your CI is publicly accessible
   - OAuth provides authentication
   - Keep `WOODPECKER_OPEN=false` unless you want public registration

3. **Secrets Management**:
   - Never commit `.env` file
   - Use strong agent secret (32+ characters)
   - Rotate OAuth credentials regularly

## Troubleshooting

### Common Issues

1. **"agent could not auth: please provide a token"**
   - Ensure `WOODPECKER_AGENT_SECRET` is set and matches in both services

2. **Webhook delivery failures**
   - Verify Woodpecker is publicly accessible
   - Check firewall rules
   - Confirm OAuth redirect URI matches exactly

3. **Server unhealthy status**
   - Normal if health endpoint isn't fully configured
   - Check logs: `docker logs ontree-woodpecker-woodpecker-server-1`

4. **Builds not triggering**
   - Verify webhook exists in Codeberg repository settings
   - Check Woodpecker server logs for webhook receipt
   - Ensure repository is activated in Woodpecker UI

## Future Enhancements

1. **Automated Setup**: Script to automate OAuth app creation and configuration
2. **Built-in Tunneling**: Integrate Tailscale Funnel or similar into TreeOS
3. **Monitoring**: Add Woodpecker metrics to TreeOS monitoring dashboard
4. **Multi-Agent Support**: Allow scaling agents across multiple machines

## References

- [Woodpecker CI Documentation](https://woodpecker-ci.org/docs/intro)
- [Codeberg CI Documentation](https://docs.codeberg.org/ci/)
- [Tailscale Funnel Documentation](https://tailscale.com/kb/1223/tailscale-funnel/)
- [TreeOS Repository](https://codeberg.org/stefanmunz/treeos)

## Migration Notes

When re-enabling Woodpecker CI in TreeOS:
1. Move templates from `woodpecker_ci/` back to `templates/compose/`
2. Add Woodpecker to embedded templates list
3. Update template categories to include Woodpecker
4. Test full deployment flow with public access solution