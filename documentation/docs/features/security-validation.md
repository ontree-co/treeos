---
sidebar_position: 5
---

# Security Validation

TreeOS implements comprehensive security validation for all Docker Compose configurations to ensure containers run safely and don't compromise the host system.

## Overview

Before any container is started, TreeOS validates the docker-compose.yml file against strict security rules. This validation happens automatically when:
- Creating apps from templates
- Creating custom apps
- Updating compose configurations
- Starting containers

## Security Rules

### 1. Privileged Mode

**Rule**: Containers cannot run with `privileged: true`

**Why**: Privileged containers have full access to the host system, including all devices and capabilities. This poses significant security risks.

**Example of blocked configuration**:
```yaml
services:
  app:
    image: nginx
    privileged: true  # ❌ Not allowed
```

### 2. Dangerous Capabilities

**Rule**: Certain Docker capabilities are not allowed

**Blocked capabilities**:
- `SYS_ADMIN` - Mount filesystems, change hostname, etc.
- `NET_ADMIN` - Configure network interfaces
- `SYS_MODULE` - Load/unload kernel modules
- `SYS_RAWIO` - Raw I/O port operations
- `SYS_PTRACE` - Trace processes
- `SYS_BOOT` - Reboot the system
- `MAC_ADMIN` - MAC configuration
- `MAC_OVERRIDE` - Override MAC policies
- `DAC_READ_SEARCH` - Bypass file read permission checks
- `SETFCAP` - Set file capabilities

**Example of blocked configuration**:
```yaml
services:
  app:
    image: nginx
    cap_add:
      - SYS_ADMIN  # ❌ Not allowed
```

### 3. Bind Mount Restrictions

**Rule**: Host path bind mounts must follow strict patterns

**Allowed bind mounts**:
1. **Named volumes** (recommended):
   ```yaml
   volumes:
     - mydata:/data  # ✅ Named volume - allowed
   ```

2. **Absolute paths within app's mount directory**:
   ```yaml
   volumes:
     - /opt/ontree/apps/mount/myapp/service/data:/data  # ✅ Allowed
   ```

3. **Special exception - Shared models directory** (for AI apps):
   ```yaml
   volumes:
     - /opt/ontree/sharedmodels:/models  # ✅ Allowed for AI model sharing
   ```

**Blocked bind mounts**:
```yaml
services:
  app:
    image: nginx
    volumes:
      - /etc:/host-etc  # ❌ Not allowed - outside mount directory
      - ./data:/data    # ❌ Not allowed - relative paths
      - ~/data:/data    # ❌ Not allowed - home directory access
```

## Bind Mount Directory Structure

When you need to use bind mounts, they must follow this pattern:
```
/opt/ontree/apps/mount/{app-name}/{service-name}/
```

For example, if your app is named `nextcloud` with a service named `app`:
```yaml
services:
  app:
    volumes:
      - /opt/ontree/apps/mount/nextcloud/app/data:/var/www/html/data
```

## Best Practices

### 1. Prefer Named Volumes

Named volumes are the recommended approach for persistent storage:

```yaml
services:
  database:
    image: postgres
    volumes:
      - postgres_data:/var/lib/postgresql/data  # ✅ Best practice

volumes:
  postgres_data:  # Declare the named volume
```

### 2. Use Docker Configs for Configuration Files

Instead of bind-mounting configuration files, use Docker's config feature:

```yaml
services:
  app:
    image: myapp
    configs:
      - source: app_config
        target: /etc/app/config.yml

configs:
  app_config:
    content: |
      # Your configuration here
      setting: value
```

### 3. Environment Variables for Configuration

When possible, use environment variables instead of configuration files:

```yaml
services:
  app:
    image: myapp
    environment:
      - DATABASE_URL=postgres://db/myapp
      - REDIS_URL=redis://cache:6379
```

## Validation Errors

When validation fails, you'll see detailed error messages:

```
Security validation failed for service 'app': bind mount path -
bind mount path './config' is not allowed. Use named volumes instead
(e.g., 'mydata:/path') or absolute paths within
'/opt/ontree/apps/mount/myapp/'
```

## Working with Templates

TreeOS templates are pre-validated to ensure they meet security requirements. If you modify a template or create custom configurations, ensure they follow these security rules.

## Exceptions and Special Cases

### AI Model Sharing

The `/opt/ontree/sharedmodels` directory is specifically allowed for AI applications that need to share large model files between containers. This prevents duplicating multi-gigabyte model files.

### Development Mode

Security validation is always enforced, even in development mode. This ensures your applications are production-ready from the start.

## Troubleshooting

### "Bind mount path not allowed" Error

**Problem**: Your compose file uses relative paths or paths outside the allowed directory.

**Solution**:
1. Convert to named volumes (recommended)
2. Or use absolute paths within `/opt/ontree/apps/mount/{app-name}/`

### "Dangerous capability" Error

**Problem**: Your container requests privileged capabilities.

**Solution**:
1. Remove the capability from `cap_add`
2. Find alternative approaches that don't require privileged access
3. Use specific, less dangerous capabilities if needed

### "Privileged mode not allowed" Error

**Problem**: Your container has `privileged: true`.

**Solution**:
1. Remove the privileged flag
2. Use specific capabilities instead (if allowed)
3. Redesign your application to work without privileged access

## Security Philosophy

TreeOS follows the principle of least privilege. Containers should only have access to what they absolutely need. This approach:
- Prevents container escapes
- Limits damage from compromised containers
- Ensures multi-tenant safety
- Maintains system stability

By enforcing these rules, TreeOS ensures your applications run securely without compromising the host system or other applications.