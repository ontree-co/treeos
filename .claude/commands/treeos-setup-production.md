---
description: Setup TreeOS in production mode with automatic configuration
argument-hint:
---

# Setup TreeOS Production Environment

## IMPORTANT AGENT INSTRUCTIONS
**DO NOT** copy or create any setup scripts. The production setup script already exists in the repository at `.claude/commands/treeos-setup-production-noconfirm.sh`. Always reference and use this existing script. Never create duplicates or copies.

## Overview
This command will set up TreeOS in production mode on your server with proper service management using the existing setup script in the repository's `.claude/commands/` folder.

## Prerequisites
Before running, ensure you have:
1. Docker and Docker Compose installed
2. Sudo/root access on the target machine

## What This Setup Does

### Creates System Structure:
- User: `ontree` (system service user)
- Directory: `/opt/ontree/` with proper structure
- Downloads and installs latest TreeOS release from GitHub to `/opt/ontree/treeos`
- Sets up systemd (Linux) or launchd (macOS) service

### Automatic Features:
- Downloads the latest stable TreeOS release for your OS/architecture
- Backs up existing installations if found (or cleanly removes if backup fails due to permissions)
- Installs AMD ROCm 7.0.1 if AMD GPU detected (Linux)
- Configures GPU permissions for container access
- Adds ontree user to docker group for container management
- Starts TreeOS service automatically

### Directory Structure Created:
```
/opt/ontree/
├── treeos          # Main binary
├── apps/           # Application configurations
├── shared/         # Shared data between apps
│   └── ollama/     # Ollama models storage
└── logs/           # Application logs
```

## Setup Process

### Step 1: Check Prerequisites
Verify that Docker is installed and running.

### Step 2: Use Existing Script and Guide User
NOTE: The treeos-setup-production-noconfirm.sh script is already marked as executable in the repository (chmod +x is committed in git).

1. Check if we can use sudo:
!sudo -n true 2>/dev/null && echo "SUDO_AVAILABLE" || echo "SUDO_REQUIRED"

2. If sudo is not available (which is typical in Claude Code), inform the user:
   - This script requires sudo privileges to create user, directories, and install services
   - Claude Code cannot provide sudo passwords for security reasons
   - The user needs to run this command manually from the repository root

3. Provide the command for the user to run from the repository root directory:
```bash
cd ~/repositories/ontree/treeos
sudo ./.claude/commands/treeos-setup-production-noconfirm.sh
```

4. Ask the user to paste the output back after running the script so you can verify success

### Step 3: Report Results
If sudo was available and setup succeeded:
!echo "TreeOS has been successfully installed in production mode!"
!echo "Access the web interface at: http://localhost:3000"

If sudo is required (typical case):
!echo "Please run the following commands manually on your server with sudo access:"
!echo ""
!echo "cd ~/repositories/ontree/treeos"
!echo "sudo ./.claude/commands/treeos-setup-production-noconfirm.sh"
!echo ""
!echo "After running, please paste the output back here so I can verify the installation succeeded."

## Post-Setup

After successful installation:
- TreeOS web interface: http://localhost:3000
- Default credentials are in your .env file
- Service will start automatically on boot

## Service Management

### Linux (systemd):
```bash
sudo systemctl status treeos   # Check status
sudo systemctl stop treeos     # Stop service
sudo systemctl start treeos    # Start service
sudo journalctl -u treeos -f   # View logs
```

### macOS (launchd):
```bash
sudo launchctl list | grep com.ontree.treeos  # Check status
sudo launchctl stop com.ontree.treeos         # Stop service
sudo launchctl start com.ontree.treeos        # Start service
```

## Troubleshooting

If installation fails:
1. Ensure Docker is running
2. Verify sudo access
3. Review error messages in the output (download failures will be shown)

**IMPORTANT REMINDERS**:
- Script requires sudo privileges to create system user and directories
- This is a one-time setup that configures TreeOS as a system service
- The service will run as the 'ontree' user for security
- Docker group membership allows container management without sudo