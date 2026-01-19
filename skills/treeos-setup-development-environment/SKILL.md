---
name: treeos-setup-development-environment
description: Setup TreeOS development environment with asdf and required tool versions.
---

# Setup TreeOS Development Environment

## IMPORTANT AGENT INSTRUCTIONS
**DO NOT** copy or create any setup scripts. The development setup script already exists in the repository at `skills/treeos-setup-development-environment/run.sh`. Always reference and use this existing script. Never create duplicates or copies.

## Overview
This command will set up your development environment with asdf version manager and all required development tools (Go, golangci-lint, Node.js) using the existing setup script in the repository's `skills/treeos-setup-development-environment/` folder.

## Prerequisites
Before running, ensure you have:
1. **Linux**: sudo access (needed for `apt install asdf` or `dnf install asdf`)
2. **macOS**: Homebrew installed
3. git, curl, and gpg (usually pre-installed)

## What This Setup Does

### Installs asdf via Package Manager:
- **Linux**: Installs asdf via `apt` (Ubuntu/Debian) or `dnf`/`yum` (Fedora/RHEL)
- **macOS**: Installs asdf via Homebrew (`brew install asdf`)

### Installs Development Tools:
- Go 1.24.4
- golangci-lint 2.5.0
- Node.js 22.11.0

### Automatic Features:
- Detects OS and package manager automatically
- Configures shell for asdf (~/.bashrc or ~/.zshrc)
- Installs asdf plugins (golang, golangci-lint, nodejs)
- Reads `.tool-versions` and installs exact versions
- Verifies all tools are correctly installed

## Setup Process

### Step 1: Check Prerequisites
Verify that basic tools are installed:
- git (required)
- curl (required)
- gpg (needed for nodejs plugin)

### Step 2: Check if Sudo Needed

**On Linux**, check if we can use sudo:
```bash
sudo -n true 2>/dev/null && echo "SUDO_AVAILABLE" || echo "SUDO_REQUIRED"
```

**On macOS**, no sudo needed (Homebrew runs as regular user).

### Step 3: Run Setup Script

**If sudo is available (or on macOS):**

Try to run the script directly (this is unlikely to work in Claude Code on Linux due to sudo):
```bash
cd ~/repositories/treeos  # Adjust path as needed
sudo ./skills/treeos-setup-development-environment/run.sh
```

**If sudo is required** (typical case in Claude Code):

Inform the user that sudo is needed and provide the command:

```
⚠️ This script requires sudo privileges on Linux to install asdf via package manager.
Claude Code cannot provide sudo passwords for security reasons.

Please run this command manually from your terminal:

cd ~/repositories/treeos
sudo ./skills/treeos-setup-development-environment/run.sh

After running, please paste the output back here so I can verify the installation succeeded.
```

### Step 4: Verify Success

After the script runs successfully, inform the user to:

1. **Restart terminal** (or run `source ~/.bashrc`)
2. **Verify installation**:
   ```bash
   asdf current
   # Should show:
   # golang          1.24.4
   # golangci-lint   2.5.0
   # nodejs          22.11.0
   ```
3. **Test linting** to ensure golangci-lint works correctly:
   ```bash
   make lint
   ```

## Post-Setup

After successful installation:
- Tool versions automatically switch when you're in the repository
- No need to manually manage Go, golangci-lint, or Node.js versions
- All developers and CI use the same versions from `.tool-versions`

## Troubleshooting

If installation fails:

1. **Missing prerequisites**: Install git, curl, or gpg
2. **Wrong OS/package manager**: Script supports Ubuntu (apt), Fedora/RHEL (dnf/yum), and macOS (brew)
3. **Permissions**: On Linux, must run with sudo
4. **Homebrew missing** (macOS): Install from https://brew.sh

## Important Notes

- **Linux**: Requires sudo for package manager (`apt install asdf`)
- **macOS**: No sudo needed (Homebrew installs as regular user)
- **One-time setup**: Run once per machine
- **Automatic switching**: asdf automatically uses correct versions per repository
- **Version mismatch prevention**: Ensures all developers use exact same tool versions

## Manual Alternative

If you prefer to manage tools yourself without asdf, ensure you have:
- Go 1.24.4
- golangci-lint 2.5.0 (built with Go 1.24.4+)
- Node.js 22.11.0

**Note**: Version mismatches will cause linting and CI failures.
