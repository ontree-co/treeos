# TreeOS Setup Instructions

TreeOS can run in two modes: **Demo Mode** for quick testing and development, or **Production Mode** for permanent installations with automatic startup.

## Requirements

- Docker or Docker Desktop installed and running
- macOS (Apple Silicon or Intel) or Linux (amd64 or arm64)
- For production mode: sudo/root access

## Demo Mode

Demo mode runs TreeOS with local directories and doesn't require special permissions. This is perfect for testing or development.

### Starting Demo Mode

```bash
# From the extracted package directory
./treeos --demo
```

This will:
- Create local directories: `./apps`, `./shared`, `./logs`
- Use local database: `./ontree.db`
- Run on default port 3000

### Stopping Demo Mode

Simply press `Ctrl+C` to stop the server.

## Production Mode

Production mode installs TreeOS as a system service that starts automatically on boot.

### Installation

Run the included setup script with sudo:

```bash
sudo ./setup-production.sh
```

The setup script will:

1. **Check Prerequisites**
   - Verify root/sudo access
   - Check if `/opt/ontree` already exists (fails if it does)

2. **Create User**
   - Create `ontree` system user (or use existing)
   - This user will run the TreeOS service

3. **Set Up Directories**
   - Create `/opt/ontree` directory structure
   - Set proper ownership and permissions

4. **Install Binary**
   - Copy TreeOS binary to `/opt/ontree/treeos`

5. **Configure Service**
   - **Linux**: Install systemd service
   - **macOS**: Install launchd service
   - Enable automatic startup

### Directory Structure

Production mode uses the following directory structure:

```
/opt/ontree/
├── treeos                  # Main binary
├── ontree.db              # Database
├── apps/                  # Installed applications
├── shared/                # Shared resources
│   └── ollama/           # Shared AI models
└── logs/                  # Log files
```

### Service Management

#### Linux (systemd)

```bash
# Start service
sudo systemctl start treeos

# Stop service
sudo systemctl stop treeos

# Restart service
sudo systemctl restart treeos

# View status
sudo systemctl status treeos

# View logs
sudo journalctl -u treeos -f

# Disable automatic startup
sudo systemctl disable treeos

# Enable automatic startup
sudo systemctl enable treeos
```

#### macOS (launchd)

```bash
# Start service
sudo launchctl load -w /Library/LaunchDaemons/com.ontree.treeos.plist

# Stop service
sudo launchctl unload /Library/LaunchDaemons/com.ontree.treeos.plist

# View status
sudo launchctl list | grep com.ontree.treeos

# View logs
tail -f /opt/ontree/logs/treeos.log

# View error logs
tail -f /opt/ontree/logs/treeos.error.log
```

## Accessing TreeOS

Once running, access TreeOS at:

- **Default**: http://localhost:3000
- **Custom port**: Set via `LISTEN_ADDR` environment variable

## Configuration

### Environment Variables

Create a `.env` file (demo mode) or set in service configuration (production):

```bash
# Port configuration
LISTEN_ADDR=:3000

# Analytics (optional)
POSTHOG_API_KEY=your_key
POSTHOG_HOST=https://app.posthog.com

# Public domain (optional)
PUBLIC_BASE_DOMAIN=example.com

# Tailscale integration (optional)
TAILSCALE_AUTH_KEY=your_key
TAILSCALE_TAGS=tag:ontree-apps

# Feature flags
MONITORING_ENABLED=true
AUTO_UPDATE_ENABLED=true
```

### Demo Mode Configuration

In demo mode, create a `.env` file in the same directory as the binary.

### Production Mode Configuration

For production, modify the service file:

- **Linux**: Edit `/etc/systemd/system/treeos.service` and add to `[Service]` section:
  ```ini
  Environment="LISTEN_ADDR=:8080"
  Environment="MONITORING_ENABLED=false"
  ```
  Then reload: `sudo systemctl daemon-reload && sudo systemctl restart treeos`

- **macOS**: Edit `/Library/LaunchDaemons/com.ontree.treeos.plist` in the `EnvironmentVariables` section, then reload the service.

## Uninstallation

### Demo Mode

Simply delete the directory containing TreeOS and its data folders.

### Production Mode

1. Stop and disable the service:
   ```bash
   # Linux
   sudo systemctl stop treeos
   sudo systemctl disable treeos
   sudo rm /etc/systemd/system/treeos.service

   # macOS
   sudo launchctl unload /Library/LaunchDaemons/com.ontree.treeos.plist
   sudo rm /Library/LaunchDaemons/com.ontree.treeos.plist
   ```

2. Remove files and directories:
   ```bash
   sudo rm -rf /opt/ontree
   ```

3. Optionally remove the user:
   ```bash
   # Linux
   sudo userdel ontree

   # macOS
   sudo dscl . -delete /Users/ontree
   ```

## Troubleshooting

### Port Already in Use

If port 3000 is already in use, either:
- Stop the conflicting service
- Change TreeOS port via `LISTEN_ADDR` environment variable

### Permission Denied

Ensure Docker is running and your user has Docker permissions:
```bash
# Linux: Add user to docker group
sudo usermod -aG docker $USER
# Then log out and back in
```

### Service Won't Start

Check logs for errors:
- **Linux**: `sudo journalctl -u treeos -n 50`
- **macOS**: `tail -50 /opt/ontree/logs/treeos.error.log`

### Existing Installation

If `/opt/ontree` already exists, the setup script will fail. To reinstall:
```bash
# Backup existing data
sudo mv /opt/ontree /opt/ontree.backup

# Or remove completely
sudo rm -rf /opt/ontree

# Then run setup again
sudo ./setup-production.sh
```

## Support

For issues, questions, or contributions:
- GitHub: https://github.com/ontree-co/treeos
- Documentation: https://docs.treeos.com