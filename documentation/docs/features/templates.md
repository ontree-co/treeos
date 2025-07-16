---
sidebar_position: 2
---

# Templates

Templates are pre-configured application definitions that make deploying complex applications as simple as clicking a button. OnTree comes with a growing library of templates and supports creating custom templates for your specific needs.

## Using Templates

### Browse Available Templates

1. **Click "Create New App"** from the dashboard
2. **Browse the template library** organized by category
3. **Each template shows**:
   - Application name and description
   - Required resources
   - Default configuration
   - Quick start instructions

### Deploy from Template

1. **Click "Use Template"** on your chosen application
2. **Customize the configuration**:
   - App name (must be unique)
   - Emoji for visual identification
   - Port mappings (if needed)
   - Environment variables
3. **Click "Create App"** to deploy

## Built-in Templates

OnTree includes templates for popular applications:

### AI & Machine Learning
- **Open WebUI** - ChatGPT-like interface for local LLMs
- **Stable Diffusion** - AI image generation
- **LocalAI** - OpenAI compatible API

### Media & Entertainment
- **Jellyfin** - Media server (alternative to Plex)
- **Plex** - Popular media server
- **Sonarr/Radarr** - Media management

### Productivity
- **Nextcloud** - Self-hosted cloud storage
- **Paperless-ngx** - Document management
- **Bookstack** - Wiki and documentation

### Development
- **Code-server** - VS Code in the browser
- **Gitea** - Lightweight Git service
- **PostgreSQL** - Database server
- **Redis** - In-memory data store

### Infrastructure
- **Nginx** - Web server and reverse proxy
- **Traefik** - Modern reverse proxy
- **Portainer** - Docker management UI

## Template Structure

Templates are YAML files with OnTree metadata:

```yaml
# template.yaml
name: "Open WebUI"
description: "ChatGPT-like interface for running LLMs locally"
category: "AI"
icon: "🤖"
website: "https://openwebui.com"
documentation: "https://docs.openwebui.com"

compose:
  version: '3.8'
  services:
    openwebui:
      image: ghcr.io/open-webui/open-webui:main
      ports:
        - "${PORT}:8080"
      volumes:
        - ./data:/app/backend/data
      environment:
        - WEBUI_SECRET_KEY=${SECRET_KEY}
      restart: unless-stopped

variables:
  PORT:
    default: "3000"
    description: "Web interface port"
  SECRET_KEY:
    generate: "random"
    length: 32
```

## Creating Custom Templates

### From Existing App

Convert a running app into a reusable template:

1. **Perfect your app configuration**
2. **Test thoroughly**
3. **Click "Save as Template"** (coming soon)
4. **Add metadata**:
   - Description
   - Category
   - Documentation links

### Manual Creation

Create templates manually for complex setups:

1. **Create template directory**:
   ```
   /templates/custom/my-app/
   ├── template.yaml
   ├── docker-compose.yml
   └── README.md
   ```

2. **Define metadata** in template.yaml
3. **Add to OnTree** via settings

## Template Variables

Templates support dynamic variables for flexibility:

### Port Assignment
```yaml
ports:
  - "${PORT:-3000}:80"
```

OnTree automatically assigns available ports.

### Random Values
```yaml
environment:
  - SECRET_KEY=${RANDOM_STRING}
  - DB_PASSWORD=${RANDOM_PASSWORD}
```

Generates secure random values on deployment.

### User Input
```yaml
environment:
  - ADMIN_EMAIL=${USER_EMAIL}
  - SITE_NAME=${SITE_NAME}
```

Prompts user during creation.

## Advanced Templates

### Multi-Service Applications

Templates can define complete stacks:

```yaml
services:
  app:
    image: wordpress
    environment:
      WORDPRESS_DB_HOST: db
      WORDPRESS_DB_PASSWORD: ${DB_PASSWORD}
    depends_on:
      - db
  
  db:
    image: mariadb
    environment:
      MYSQL_ROOT_PASSWORD: ${DB_PASSWORD}
      MYSQL_DATABASE: wordpress
    volumes:
      - ./db:/var/lib/mysql
```

### Conditional Sections

Use template logic for optional features:

```yaml
services:
  app:
    image: myapp
    # Basic configuration

  # Optional Redis cache
  {{if .EnableCache}}
  cache:
    image: redis:alpine
    command: redis-server --maxmemory 256mb
  {{end}}
```

### Resource Presets

Define resource limits in templates:

```yaml
services:
  app:
    deploy:
      resources:
        limits:
          cpus: '${CPU_LIMIT:-2.0}'
          memory: ${MEMORY_LIMIT:-4G}
        reservations:
          cpus: '0.5'
          memory: 512M
```

## Template Best Practices

### Documentation

Include comprehensive documentation:

```markdown
# My App Template

## Quick Start
1. Deploy from template
2. Access at http://localhost:PORT
3. Default login: admin/changeme

## Configuration
- `ADMIN_EMAIL`: Administrator email
- `ENABLE_SSL`: Enable HTTPS (requires domain)

## Persistent Data
- `/data`: Application data
- `/config`: Configuration files
```

### Security

- **Never hardcode secrets** - Use variables
- **Specify image versions** - Avoid `:latest`
- **Set appropriate permissions** - Use `user:` directive
- **Enable security features** - App-specific hardening

### Compatibility

- **Test on multiple platforms** - Linux, macOS, Windows
- **Document requirements** - Minimum resources, dependencies
- **Handle edge cases** - Missing directories, permissions

## Managing Templates

### Update Templates

Keep templates current:

1. **Monitor upstream changes**
2. **Test updates thoroughly**
3. **Version your templates**
4. **Document breaking changes**

### Share Templates

Contribute to the community:

1. **Create pull request** on GitHub
2. **Include documentation**
3. **Add tests if applicable**
4. **Follow contribution guidelines**

## Template Gallery

### Featured Templates

#### Immich
Modern photo management:
```yaml
category: "Media"
description: "High-performance photo and video backup"
resources:
  min_memory: "4GB"
  recommended_memory: "8GB"
  storage: "Scales with photos"
```

#### Vaultwarden
Password management:
```yaml
category: "Security"
description: "Bitwarden compatible password manager"
resources:
  min_memory: "256MB"
  recommended_memory: "512MB"
```

#### Home Assistant
Home automation:
```yaml
category: "Smart Home"
description: "Open source home automation platform"
resources:
  min_memory: "2GB"
  recommended_memory: "4GB"
```

## Troubleshooting Templates

### Template Not Loading

- Check YAML syntax
- Verify file permissions
- Review OnTree logs

### Variable Substitution Failed

- Ensure variable names match
- Check for typos in placeholders
- Verify variable definitions

### Multi-Service Issues

- Check service dependencies
- Verify network configuration
- Ensure volume paths exist

## Future Features

Planned template enhancements:

- **Template marketplace** - Community sharing
- **Auto-updates** - Keep templates current
- **Template builder UI** - Visual creation
- **Import from Docker Hub** - Automatic conversion
- **Template versioning** - Track changes

Templates are the heart of OnTree's simplicity. They transform complex deployments into single-click operations while maintaining full flexibility for power users.