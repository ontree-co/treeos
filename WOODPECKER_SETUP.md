# Woodpecker CI Setup Guide for TreeOS

This guide explains how to set up Woodpecker CI as a self-hosted CI/CD solution integrated with Codeberg using TreeOS.

## Overview

Woodpecker CI is a simple yet powerful CI/CD engine with great extensibility. This template provides a pre-configured setup for use with Codeberg (Forgejo) repositories.

## Template Components

The Woodpecker template includes:

1. **`templates/compose/woodpecker.yml`** - Docker Compose configuration with:
   - Woodpecker server (web UI and API)
   - Woodpecker agent (job runner)
   - SQLite database by default (PostgreSQL optional)
   - Health checks and service dependencies
   - Network isolation

2. **`templates/compose/woodpecker.env.example`** - Environment variable documentation
   - OAuth configuration placeholders
   - Security settings
   - Agent configuration options

## Setup Instructions

### Step 1: Create Woodpecker Instance in TreeOS

1. Navigate to **Templates** in the TreeOS web UI
2. Find **"Woodpecker CI"** under the Development category
3. Click to create a new instance
4. Configure:
   - **Application Name**: e.g., `woodpecker-ci`
   - **Port**: Default is 8000 (or choose custom)
   - **Emoji**: Select an icon for the dashboard
5. Click **Create Application**

### Step 2: Create OAuth Application on Codeberg

1. Go to https://codeberg.org/user/settings/applications
2. Click **"Create a new OAuth2 Application"**
3. Fill in the details:
   - **Application Name**: `Woodpecker CI` (or your preference)
   - **Redirect URI**: `http://YOUR_DOMAIN:8000/authorize` 
     - Replace `YOUR_DOMAIN` with your server's IP or domain
     - Replace `8000` with your chosen port if different
4. Click **Create Application**
5. **Save the Client ID and Client Secret** - you'll need these next

### Step 3: Configure Environment Variables

1. In TreeOS, go to your Woodpecker app detail page
2. Click **"Edit Configuration"**
3. Create or edit the `.env` file with the following:

```env
# Your Woodpecker instance URL (must be accessible from Codeberg)
WOODPECKER_HOST=http://YOUR_DOMAIN:8000

# Your Codeberg username (will be admin)
WOODPECKER_ADMIN=your-codeberg-username

# User registration (set to true if you want others to register)
WOODPECKER_OPEN=false

# Codeberg OAuth credentials (from Step 2)
WOODPECKER_FORGEJO=true
WOODPECKER_FORGEJO_URL=https://codeberg.org
WOODPECKER_FORGEJO_CLIENT=your-oauth-client-id-here
WOODPECKER_FORGEJO_SECRET=your-oauth-client-secret-here

# Security - MUST generate a unique secret
# Generate with: openssl rand -hex 32
WOODPECKER_AGENT_SECRET=paste-generated-32-hex-string-here

# Agent settings
WOODPECKER_MAX_WORKFLOWS=2
WOODPECKER_LOG_LEVEL=info
```

4. To generate the agent secret, run:
   ```bash
   openssl rand -hex 32
   ```
   Copy the output and paste it as `WOODPECKER_AGENT_SECRET`

5. Save the configuration

### Step 4: Start Woodpecker

1. From the app detail page, click **"Start"** to launch Woodpecker
2. Wait for the containers to start (check the logs if needed)
3. Once running, access Woodpecker at `http://YOUR_DOMAIN:8000`

### Step 5: First Login and Repository Setup

1. Visit your Woodpecker instance URL
2. Click **"Login"** - you'll be redirected to Codeberg
3. Authorize the OAuth application
4. You'll be redirected back to Woodpecker and logged in

To enable CI for a repository:
1. Click **"Add Repository"** in Woodpecker
2. Select the Codeberg repository you want to enable
3. Activate it in Woodpecker
4. Create a `.woodpecker.yml` file in your repository root

## Example Pipeline Configuration

Create `.woodpecker.yml` in your repository:

```yaml
steps:
  - name: build
    image: golang:1.21
    commands:
      - go build
      - go test ./...

  - name: docker
    image: plugins/docker
    settings:
      repo: your-dockerhub-user/your-app
      tags: latest
    when:
      branch: main
```

## Troubleshooting

### Common Issues

1. **"Host key verification failed"**
   - Woodpecker needs to trust Codeberg's SSH keys
   - This should be handled automatically

2. **OAuth redirect mismatch**
   - Ensure the redirect URL in Codeberg matches exactly: `http://YOUR_DOMAIN:8000/authorize`
   - The scheme (http/https) must match exactly

3. **Agent can't connect to server**
   - Check that both containers are on the same Docker network
   - Verify the agent secret matches in both services

4. **Can't access Woodpecker externally**
   - Ensure the port is open in your firewall
   - Check that WOODPECKER_HOST is set to a publicly accessible URL

### Logs and Debugging

View logs in TreeOS:
1. Go to the Woodpecker app detail page
2. Check the container logs section
3. Look for both `woodpecker-server` and `woodpecker-agent` logs

Or via command line:
```bash
docker logs ontree-woodpecker-ci-woodpecker-server-1
docker logs ontree-woodpecker-ci-woodpecker-agent-1
```

## Security Considerations

1. **Agent Secret**: Always use a strong, randomly generated secret
2. **OAuth Secrets**: Never commit these to version control
3. **Network Access**: Consider using a reverse proxy with SSL for production
4. **User Registration**: Keep `WOODPECKER_OPEN=false` unless you want public registration
5. **Docker Socket**: The agent needs Docker socket access - be aware of the security implications

## Advanced Configuration

### Using PostgreSQL Instead of SQLite

Uncomment the PostgreSQL sections in the docker-compose.yml and update your `.env`:

```env
WOODPECKER_DATABASE_DRIVER=postgres
WOODPECKER_DB_PASSWORD=secure-database-password
```

### Multiple Agents

You can scale agents by increasing the replica count or running agents on different machines. Each agent must use the same `WOODPECKER_AGENT_SECRET`.

### Custom Networks

If you need Woodpecker to access other TreeOS apps, ensure they're on the same Docker network or configure network access appropriately.

## Resources

- [Woodpecker CI Documentation](https://woodpecker-ci.org/docs/intro)
- [Codeberg Documentation](https://docs.codeberg.org)
- [TreeOS Documentation](https://github.com/stefanmunz/treeos)

## Support

For TreeOS-specific issues, check the TreeOS repository.
For Woodpecker CI issues, refer to the [Woodpecker documentation](https://woodpecker-ci.org/docs/intro).