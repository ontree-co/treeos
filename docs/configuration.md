# OnTree Configuration Guide

This document provides a comprehensive overview of all configuration options available in OnTree.

## Configuration Methods

OnTree supports two methods of configuration:

1. **Environment Variables** - Takes precedence over config file
2. **Configuration File** (`config.toml`) - Default configuration method

## Core Configuration

### Server Settings

| Environment Variable | Config File Key | Default | Description |
|---------------------|-----------------|---------|-------------|
| `LISTEN_ADDR` | `listen_addr` | `:3000` | HTTP server listen address |
| `PORT` | - | - | Alternative way to set port (overrides LISTEN_ADDR) |
| `DATABASE_PATH` | `database_path` | `./ontree.db` | Path to SQLite database file |
| `ONTREE_APPS_DIR` | `apps_dir` | `./apps` | Directory where applications are stored |

### Authentication

| Environment Variable | Config File Key | Default | Description |
|---------------------|-----------------|---------|-------------|
| `AUTH_USERNAME` | - | - | **Required** - Basic auth username |
| `AUTH_PASSWORD` | - | - | **Required** - Basic auth password |
| `SESSION_KEY` | - | Auto-generated | Session encryption key (32 bytes) |

## Feature Configuration

### System Monitoring

| Environment Variable | Config File Key | Default | Description |
|---------------------|-----------------|---------|-------------|
| `MONITORING_ENABLED` | `monitoring_enabled` | `true` | Enable/disable monitoring dashboard |

**Details**:
- When enabled, provides real-time system metrics dashboard at `/monitoring`
- Collects CPU, Memory, Disk, and Network metrics every 60 seconds
- Stores 7 days of historical data
- Includes sparkline visualizations and detailed charts

### Caddy Integration

| Environment Variable | Config File Key | Default | Description |
|---------------------|-----------------|---------|-------------|
| `PUBLIC_BASE_DOMAIN` | `public_base_domain` | - | Public domain for app exposure (e.g., `example.com`) |
| `TAILSCALE_BASE_DOMAIN` | `tailscale_base_domain` | - | Tailscale domain for app exposure |

**Details**:
- Enables exposing apps via Caddy reverse proxy
- Requires Caddy to be installed with admin API enabled
- Apps can be accessed at `https://subdomain.yourdomain.com`

### Analytics

| Environment Variable | Config File Key | Default | Description |
|---------------------|-----------------|---------|-------------|
| `POSTHOG_API_KEY` | `posthog_api_key` | - | PostHog analytics API key |
| `POSTHOG_HOST` | `posthog_host` | `https://app.posthog.com` | PostHog host URL |

## Example Configurations

### Minimal Configuration (Environment Variables)

```bash
export AUTH_USERNAME="admin"
export AUTH_PASSWORD="secure-password-here"
./ontree-server
```

### Full Configuration File (config.toml)

```toml
# Server configuration
listen_addr = ":8080"
database_path = "/var/lib/ontree/ontree.db"
apps_dir = "/var/lib/ontree/apps"

# Monitoring
monitoring_enabled = true

# Caddy integration
public_base_domain = "apps.example.com"
tailscale_base_domain = "myserver.tailnet.ts.net"

# Analytics (optional)
posthog_api_key = "phc_xxxxxxxxxxxxx"
posthog_host = "https://app.posthog.com"
```

### Docker Configuration Example

```yaml
version: '3.8'
services:
  ontree:
    image: ontree/ontree-server:latest
    environment:
      - AUTH_USERNAME=admin
      - AUTH_PASSWORD=secure-password
      - MONITORING_ENABLED=true
      - PUBLIC_BASE_DOMAIN=apps.myserver.com
    volumes:
      - ./ontree.db:/app/ontree.db
      - ./apps:/app/apps
      - /var/run/docker.sock:/var/run/docker.sock
    ports:
      - "3000:3000"
```

## Configuration Precedence

1. Environment variables (highest priority)
2. Configuration file (`config.toml`)
3. Default values (lowest priority)

## Security Considerations

1. **Authentication**: Always set strong passwords for `AUTH_USERNAME` and `AUTH_PASSWORD`
2. **Session Key**: Let OnTree auto-generate this unless you need session persistence across restarts
3. **File Permissions**: Ensure config files are readable only by the OnTree process user
4. **Docker Socket**: When mounting Docker socket, ensure proper permissions

## Monitoring Configuration Details

### Performance Impact

The monitoring feature has minimal performance impact:
- Data collection: ~10MB memory overhead
- CPU usage: <1% for metric collection
- Database growth: ~5MB per week
- Network overhead: Negligible

### Disabling Monitoring

To disable monitoring for resource-constrained environments:

```bash
export MONITORING_ENABLED=false
```

Or in `config.toml`:
```toml
monitoring_enabled = false
```

When disabled:
- No background metric collection
- `/monitoring` route returns 404
- No "Monitoring" menu item
- Reduces memory usage by ~10MB

## Troubleshooting

### Common Issues

1. **Port Already in Use**
   ```bash
   export LISTEN_ADDR=":8080"  # Use different port
   ```

2. **Database Permissions**
   ```bash
   chmod 600 ontree.db  # Secure database file
   chown ontree:ontree ontree.db  # Set proper ownership
   ```

3. **Monitoring Not Working**
   - Check if `MONITORING_ENABLED=true`
   - Verify database is writable
   - Check system has required permissions for metric collection

### Debug Mode

Enable debug logging (future enhancement):
```bash
export LOG_LEVEL=debug
```

## Migration from Previous Versions

When upgrading OnTree:

1. Backup your database and apps directory
2. New configuration options have sensible defaults
3. Existing configurations remain compatible
4. Run any migration scripts if provided

## Best Practices

1. **Use Config File**: For production, use `config.toml` instead of environment variables
2. **Secure Storage**: Store sensitive configuration in a secure location
3. **Regular Backups**: Backup both database and apps directory
4. **Monitor Resources**: Use the monitoring dashboard to track OnTree's resource usage
5. **Update Regularly**: Keep OnTree updated for security and feature improvements