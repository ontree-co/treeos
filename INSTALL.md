# OnTree Installation Guide

This guide provides detailed instructions for installing OnTree on various platforms.

## System Requirements

### Minimum Requirements
- **CPU**: 1 core (2+ cores recommended)
- **RAM**: 512MB (1GB+ recommended)
- **Storage**: 100MB for OnTree + space for container data
- **OS**: Linux (AMD64) or macOS (ARM64)
- **Docker**: Version 20.10 or later

### Supported Platforms
- Linux AMD64 (Ubuntu 20.04+, Debian 10+, RHEL 8+, etc.)
- macOS ARM64 (Apple Silicon, macOS 11+)

## Installation Methods

### Method 1: Download Pre-built Binary (Recommended)

1. Visit the [OnTree Releases page](https://github.com/yourusername/ontree-node/releases)

2. Download the appropriate binary for your platform:
   - `ontree-server-linux-amd64` for Linux systems
   - `ontree-server-darwin-arm64` for Apple Silicon Macs

3. Rename and make executable:
   ```bash
   # Linux example
   mv ontree-server-linux-amd64 ontree-server
   chmod +x ontree-server
   
   # macOS example  
   mv ontree-server-darwin-arm64 ontree-server
   chmod +x ontree-server
   ```

4. (Optional) Move to system path:
   ```bash
   sudo mv ontree-server /usr/local/bin/
   ```

### Method 2: Install via Package Manager

#### macOS (Homebrew)
```bash
# Coming soon
brew tap yourusername/ontree
brew install ontree
```

#### Linux (apt/yum)
Package manager support is planned for future releases.

### Method 3: Build from Source

#### Prerequisites
- Go 1.23 or later
- Git
- Make

#### Steps
1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/ontree-node.git
   cd ontree-node
   ```

2. Build the application:
   ```bash
   make build
   ```

3. The binary will be created at `./ontree-server`

### Method 4: Docker Installation

Run OnTree in a container:
```bash
docker run -d \
  --name ontree \
  -p 8080:8080 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v ontree-data:/data \
  -e AUTH_USERNAME=admin \
  -e AUTH_PASSWORD=your-secure-password \
  yourusername/ontree:latest
```

## Post-Installation Setup

### 1. Verify Docker Access

OnTree requires access to the Docker daemon. Verify Docker is running:
```bash
docker version
```

If using a non-root user, ensure they're in the docker group:
```bash
sudo usermod -aG docker $USER
# Log out and back in for changes to take effect
```

### 2. Create Configuration

OnTree uses environment variables for configuration. Create a `.env` file:
```bash
cat > ontree.env << EOF
# Required authentication
AUTH_USERNAME=admin
AUTH_PASSWORD=change-this-password

# Optional settings
PORT=8080
DATABASE_PATH=./ontree.db
SESSION_KEY=$(openssl rand -base64 32)
EOF
```

### 3. Run OnTree

With environment file:
```bash
source ontree.env && ./ontree-server
```

Or with systemd (see DEPLOYMENT.md for details):
```bash
sudo systemctl start ontree
sudo systemctl enable ontree
```

### 4. Access the Web Interface

Open your browser to:
- Local installation: `http://localhost:8080`
- Remote installation: `http://your-server-ip:8080`

Log in with the credentials you configured.

## Troubleshooting

### Permission Denied Errors

If you see "permission denied" when accessing Docker:
```bash
# Add user to docker group
sudo usermod -aG docker $USER

# Or run with sudo (not recommended)
sudo ./ontree-server
```

### Port Already in Use

If port 8080 is already in use:
```bash
# Use a different port
PORT=3000 ./ontree-server

# Or find what's using port 8080
sudo lsof -i :8080
```

### Database Errors

If you encounter database errors:
```bash
# Remove old database and let OnTree recreate it
rm ontree.db
./ontree-server
```

### macOS Security Warning

On macOS, you may see "cannot be opened because the developer cannot be verified":
1. Go to System Preferences → Security & Privacy
2. Click "Open Anyway" for ontree-server
3. Or run: `xattr -d com.apple.quarantine ./ontree-server`

## Uninstallation

### Binary Installation
```bash
# Remove binary
sudo rm /usr/local/bin/ontree-server

# Remove data
rm -rf ~/.ontree
rm ontree.db
```

### Docker Installation
```bash
# Stop and remove container
docker stop ontree
docker rm ontree

# Remove data volume (optional)
docker volume rm ontree-data

# Remove image
docker rmi yourusername/ontree:latest
```

## Next Steps

- See [DEPLOYMENT.md](DEPLOYMENT.md) for production deployment guidelines
- Check the [README.md](README.md) for usage instructions
- Report issues at [GitHub Issues](https://github.com/yourusername/ontree-node/issues)