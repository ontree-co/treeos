---
sidebar_position: 1
---

# Installation

Getting OnTree up and running takes just a few minutes. This guide will walk you through the installation process step by step.

## Prerequisites

Before installing OnTree, ensure you have:

- **Docker** installed and running (version 20.10 or later)
- **Docker Compose** plugin (usually included with Docker Desktop)
- A **Linux**, **macOS**, or **Windows** system with WSL2
- At least **2GB of RAM** available
- Port **8080** available (or configure a different port)

### Verify Docker Installation

```bash
docker --version
# Should output: Docker version 20.10.x or higher

docker compose version
# Should output: Docker Compose version v2.x.x or higher
```

## Download OnTree

OnTree is distributed as a single binary file. Download the appropriate version for your platform:

### Linux
```bash
# Download the latest release
wget https://github.com/ontree/ontree-node/releases/latest/download/ontree-server-linux-amd64
chmod +x ontree-server-linux-amd64
sudo mv ontree-server-linux-amd64 /usr/local/bin/ontree-server
```

### macOS
```bash
# Intel Mac
wget https://github.com/ontree/ontree-node/releases/latest/download/ontree-server-darwin-amd64
chmod +x ontree-server-darwin-amd64
sudo mv ontree-server-darwin-amd64 /usr/local/bin/ontree-server

# Apple Silicon (M1/M2)
wget https://github.com/ontree/ontree-node/releases/latest/download/ontree-server-darwin-arm64
chmod +x ontree-server-darwin-arm64
sudo mv ontree-server-darwin-arm64 /usr/local/bin/ontree-server
```

### Windows (WSL2)
Follow the Linux instructions above within your WSL2 environment.

## Start OnTree

Once downloaded, starting OnTree is simple:

```bash
# Start OnTree
ontree-server

# Or start with a custom port
ontree-server --port 3000
```

OnTree will:
1. Create necessary directories (`apps/` for your applications)
2. Initialize the SQLite database
3. Start the web server on port 8080 (default)

## Access the Web Interface

Open your web browser and navigate to:

```
http://localhost:8080
```

You should see the OnTree dashboard. Congratulations! 🎉

## Configuration Options

OnTree can be configured using environment variables or a configuration file.

### Environment Variables

```bash
# Change the port
PORT=3000 ontree-server

# Set database path
DATABASE_PATH=/var/lib/ontree/ontree.db ontree-server

# Enable debug logging
DEBUG=true ontree-server
```

### Configuration File

Create a `config.toml` file in the same directory as the binary:

```toml
# Server configuration
port = 8080
host = "0.0.0.0"

# Database configuration
database_path = "./ontree.db"

# Monitoring
monitoring_enabled = true

# Domain configuration (optional)
public_base_domain = "example.com"
tailscale_base_domain = "tail-scale.ts.net"
```

## Running as a Service

### Linux (systemd)

Create `/etc/systemd/system/ontree.service`:

```ini
[Unit]
Description=OnTree Container Manager
After=docker.service
Requires=docker.service

[Service]
Type=simple
ExecStart=/usr/local/bin/ontree-server
Restart=always
User=ontree
Group=docker
Environment="PATH=/usr/bin:/usr/local/bin"
WorkingDirectory=/var/lib/ontree

[Install]
WantedBy=multi-user.target
```

Enable and start the service:

```bash
# Create user and directories
sudo useradd -r -s /bin/false ontree
sudo usermod -aG docker ontree
sudo mkdir -p /var/lib/ontree
sudo chown ontree:ontree /var/lib/ontree

# Enable and start service
sudo systemctl enable ontree
sudo systemctl start ontree
sudo systemctl status ontree
```

### Docker

You can also run OnTree itself in a Docker container:

```yaml
# docker-compose.yml
version: '3.8'

services:
  ontree:
    image: ghcr.io/ontree/ontree:latest
    container_name: ontree
    ports:
      - "8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - ./apps:/apps
      - ./data:/data
    restart: unless-stopped
```

## Next Steps

Now that OnTree is installed:

1. **[Set up domains](/docs/getting-started/domain-setup)** - Configure Caddy integration for HTTPS
2. **[Create your first app](/docs/getting-started/first-app)** - Deploy Open WebUI
3. **[Explore features](/docs/features/app-management)** - Learn about all OnTree capabilities

## Troubleshooting

### Port Already in Use

If port 8080 is already in use:

```bash
# Use a different port
ontree-server --port 3000

# Or find what's using port 8080
sudo lsof -i :8080
```

### Permission Denied

If you get Docker permission errors:

```bash
# Add your user to the docker group
sudo usermod -aG docker $USER

# Log out and back in, or run
newgrp docker
```

### Can't Connect to Docker

Ensure Docker is running:

```bash
# Check Docker status
systemctl status docker

# Start Docker if needed
sudo systemctl start docker
```

## Getting Help

If you encounter issues:

- Check the [GitHub Issues](https://github.com/ontree/ontree-node/issues)
- Review the logs: OnTree outputs detailed logs to help diagnose problems
- Join the community discussions on GitHub