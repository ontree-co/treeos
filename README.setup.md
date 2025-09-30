# TreeOS Setup Instructions

TreeOS can run in two modes: **Demo Mode** for quick testing and development, or **Production Mode** for permanent installations with automatic startup.

## Requirements

- Podman 4.x installed (on macOS, ensure a Podman machine is running)
- macOS (Apple Silicon or Intel) or Linux (amd64 or arm64)
- For production mode: sudo/root access

## System Check

TreeOS automatically performs a system check during setup and from the Settings page. It verifies required directories, Podman/Podman Compose availability, and the Caddy binary. If a dependency is missing, the check displays installation tips tailored to your platform. You can rerun the system check at any time from Settings → System Check.

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

### AMD GPU Support (Linux)

For AMD GPU acceleration with Ollama and other AI workloads, you need:

#### 1. ROCm 7.0.1 Installation

Install the latest ROCm drivers for optimal GPU performance:

```bash
# Download the installer
wget https://repo.radeon.com/amdgpu-install/7.0.1/ubuntu/noble/amdgpu-install_7.0.1.70001-1_all.deb

# For Ubuntu 22.04 (jammy), use:
# wget https://repo.radeon.com/amdgpu-install/7.0.1/ubuntu/jammy/amdgpu-install_7.0.1.70001-1_all.deb

# Install the installer package
sudo apt install ./amdgpu-install_7.0.1.70001-1_all.deb

# Add AMD GPG key and configure repositories
wget -q -O - https://repo.radeon.com/rocm/rocm.gpg.key | gpg --dearmor | sudo tee /etc/apt/keyrings/rocm.gpg > /dev/null

# Configure signed repositories (replace 'noble' with your Ubuntu version if needed)
echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/rocm.gpg] https://repo.radeon.com/amdgpu/7.0.1/ubuntu noble main" | sudo tee /etc/apt/sources.list.d/amdgpu.list
echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/rocm.gpg] https://repo.radeon.com/rocm/apt/7.0.1 noble main" | sudo tee /etc/apt/sources.list.d/rocm.list
echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/rocm.gpg] https://repo.radeon.com/amdgpu/7.0.1/ubuntu noble proprietary" | sudo tee -a /etc/apt/sources.list.d/amdgpu.list

# Update and install ROCm
sudo apt update
sudo amdgpu-install --usecase=rocm --accept-eula -y

# Verify installation
rocm-smi
```

#### 2. User Group Membership

Add your user to the GPU access groups:

```bash
sudo usermod -aG render,video "$USER"
# Apply the new groups to your current shell or log out and back in
newgrp render
newgrp video
```

Confirm the groups with `id`. Without these memberships, Podman containers cannot access `/dev/kfd` or `/dev/dri` devices.

#### 3. Supported Templates

TreeOS includes optimized templates for various AMD GPUs:
- `ollama-amd-ai370` - For AMD Radeon AI 370
- `ollama-amd-ryzen-ai-395` - For AMD Ryzen AI MAX+ 395 with Radeon 8060S
- `ollama-amd` - Generic AMD GPU support

These templates include the necessary environment variables (like `HSA_OVERRIDE_GFX_VERSION`) for optimal performance.

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

3. **AMD GPU Support (Optional)**
   - Detect AMD GPUs and offer ROCm 7.0.1 installation
   - Configure GPU access permissions for the `ontree` user
   - Add user to `render` and `video` groups

4. **Set Up Directories**
   - Create `/opt/ontree` directory structure
   - Set proper ownership and permissions

5. **Install Binary**
   - Copy TreeOS binary to `/opt/ontree/treeos`

6. **Configure Service**
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

Ensure Podman is installed and the runtime is ready for your user:
```bash
podman info
```

If this command fails on macOS, initialise the Podman machine first:
```bash
podman machine init
podman machine start
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
