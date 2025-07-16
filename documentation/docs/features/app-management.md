---
sidebar_position: 1
---

# App Management

OnTree provides comprehensive tools for managing the complete lifecycle of your containerized applications. From creation to deletion, every operation is designed to be intuitive and powerful.

## Creating Apps

### From Templates

The easiest way to create apps is using pre-configured templates:

1. **Navigate to Templates** via "Create New App" button
2. **Choose a template** that fits your needs
3. **Customize settings** (name, emoji, ports)
4. **Create and start** with one click

### Custom Apps

For applications not in our template library:

1. **Click "Create from Scratch"** on the templates page
2. **Enter app details**:
   - Name (lowercase, hyphens allowed)
   - Docker image
   - Port mappings
   - Environment variables
   - Volumes
3. **OnTree generates** the docker-compose.yml automatically

## Container Operations

### Starting and Stopping

Control your containers with single-click operations:

- **Start**: Launches a stopped container
- **Stop**: Gracefully stops a running container
- **Restart**: Stops and starts in one operation

All operations show real-time progress in the operation logs.

### Viewing Status

The app detail page shows comprehensive status information:

- **Container State**: Running, stopped, or creating
- **Uptime**: How long the container has been running
- **Resource Usage**: CPU and memory consumption
- **Port Mappings**: External → Internal port mappings
- **Health Status**: If health checks are configured

### Operation Logs

Every container operation is logged in detail:

1. **Automatic display** during operations
2. **Timestamp** for each action
3. **Command equivalents** shown for learning
4. **Error details** if operations fail

## Configuration Management

### Editing docker-compose.yml

OnTree provides an in-browser YAML editor:

1. **Click "Edit"** in the Configuration card
2. **Modify the YAML** with syntax highlighting
3. **Validation** occurs before saving
4. **Automatic recreation** if container is running

### Common Modifications

#### Adding Environment Variables
```yaml
services:
  app:
    environment:
      - NEW_VAR=value
      - ANOTHER_VAR=another_value
```

#### Adding Volumes
```yaml
services:
  app:
    volumes:
      - ./data:/app/data
      - /host/path:/container/path
```

#### Changing Resource Limits
```yaml
services:
  app:
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 4G
```

## Data Management

### Persistent Storage

OnTree automatically creates persistent storage:

- **App Directory**: `/apps/{app-name}/`
- **Data Volumes**: Preserved between container recreations
- **Configuration**: Stored in docker-compose.yml

### Backup Considerations

Your data is stored in:
```
/apps/
├── myapp/
│   ├── docker-compose.yml
│   └── data/
│       └── (persistent volumes)
```

Regular backups of the `/apps` directory ensure data safety.

## Advanced Features

### Container Logs

View real-time logs from your containers:

1. **Click "View Logs"** on the app detail page
2. **Stream live output** from the container
3. **Search and filter** logs (keyboard shortcuts available)
4. **Download logs** for offline analysis

### Shell Access

Access container shell (coming soon):
```bash
# Equivalent to:
docker exec -it {container-name} /bin/bash
```

### Health Checks

Configure health checks in docker-compose.yml:

```yaml
services:
  app:
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

## Deleting Apps

OnTree provides two deletion options:

### Delete Container Only
- Removes the Docker container
- Preserves app configuration and data
- Useful for troubleshooting

### Delete App Permanently
- Removes container
- Deletes entire app directory
- Removes from Caddy (if exposed)
- **Cannot be undone**

Both options require confirmation to prevent accidents.

## Multi-Container Apps

OnTree supports docker-compose files with multiple services:

```yaml
version: '3.8'
services:
  web:
    image: nginx
    ports:
      - "8080:80"
  
  db:
    image: postgres
    environment:
      POSTGRES_PASSWORD: secret
    volumes:
      - ./pgdata:/var/lib/postgresql/data
```

All containers are managed as a single unit.

## Best Practices

### Naming Conventions
- Use descriptive names: `nextcloud`, not `app1`
- Include purpose: `postgres-dev`, `redis-cache`
- Avoid special characters

### Resource Management
- Set memory limits for stability
- Monitor usage via the monitoring dashboard
- Scale vertically before horizontally

### Security
- Use specific image tags, not `latest`
- Keep sensitive data in environment variables
- Regular updates for security patches

### Organization
- Group related apps with similar names
- Use emojis for visual organization
- Document custom configurations

## Troubleshooting

### Container Fails to Start

1. **Check operation logs** for error details
2. **Verify image name** is correct
3. **Check port conflicts** with other apps
4. **Ensure sufficient resources** (disk, memory)

### Configuration Changes Not Applied

1. **Stop the container** before major changes
2. **Use "Recreate"** for running containers
3. **Check YAML syntax** if save fails

### Performance Issues

1. **Monitor resource usage** in the dashboard
2. **Check container logs** for errors
3. **Adjust resource limits** if needed
4. **Consider host system capacity**

## Integration with Other Features

### Monitoring
- View real-time metrics
- Historical usage graphs
- Resource consumption trends

### Domain Management
- Expose apps to custom domains
- Automatic HTTPS with Caddy
- Simple subdomain configuration

### Templates
- Save custom apps as templates
- Share configurations with team
- Standardize deployments