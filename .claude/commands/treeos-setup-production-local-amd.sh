#!/bin/bash
# TreeOS Production Setup Script (No Confirmation)
# This script sets up TreeOS to run in production mode with proper service management
# Must be run with sudo/root privileges

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
# Use /usr/local/ontree on macOS, /opt/ontree on Linux
if [[ "$OSTYPE" == "darwin"* ]]; then
    ONTREE_DIR="/usr/local/ontree"
else
    ONTREE_DIR="/opt/ontree"
fi
ONTREE_USER="ontree"
BINARY_NAME="treeos"
SERVICE_NAME="treeos"

# Helper functions
print_error() {
    echo -e "${RED}❌ Error: $1${NC}" >&2
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}ℹ️  $1${NC}"
}

print_step() {
    echo -e "${YELLOW}→ $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

configure_gpu_permissions() {
    # Only relevant on Linux systems with AMD GPUs exposed via render/video groups
    if [[ "$OSTYPE" == "darwin"* ]]; then
        return
    fi

    local groups_to_check=(render video)
    local current_groups

    if ! command -v getent >/dev/null 2>&1; then
        print_info "Skipping GPU group configuration: getent not available"
        return
    fi

    current_groups=$(id -nG "$ONTREE_USER" 2>/dev/null || true)

    for group in "${groups_to_check[@]}"; do
        if getent group "$group" >/dev/null 2>&1; then
            if echo "$current_groups" | tr ' ' '\n' | grep -Fxq "$group"; then
                print_success "User '$ONTREE_USER' already in '$group' group"
            else
                print_info "Adding user '$ONTREE_USER' to '$group' group for GPU access..."
                usermod -aG "$group" "$ONTREE_USER"
                print_success "Added user '$ONTREE_USER' to '$group' group"
            fi
        else
            print_info "GPU group '$group' not found on this system; skipping"
        fi
    done
}

# Install AMD ROCm 7.0.1 for GPU support (automatic if AMD GPU detected)
install_rocm() {
    # Only for Linux systems with AMD GPUs
    if [[ "$OSTYPE" == "darwin"* ]]; then
        return
    fi

    # Check if AMD GPU is present
    if ! lspci 2>/dev/null | grep -qE "VGA|Display|3D.*AMD|Advanced Micro Devices"; then
        print_info "No AMD GPU detected, skipping ROCm installation"
        return
    fi

    # Check if ROCm is already installed
    if command -v rocm-smi >/dev/null 2>&1; then
        print_success "ROCm is already installed"
        return
    fi

    print_info "AMD GPU detected. Installing ROCm 7.0.1 for GPU acceleration..."

    # Detect Ubuntu/Debian
    if command -v apt-get >/dev/null 2>&1; then
        # Download amdgpu-install package
        print_info "Downloading AMD GPU installer..."
        wget -q https://repo.radeon.com/amdgpu-install/7.0.1/ubuntu/noble/amdgpu-install_7.0.1.70001-1_all.deb -O /tmp/amdgpu-install.deb || {
            # Fallback to jammy if noble fails
            wget -q https://repo.radeon.com/amdgpu-install/7.0.1/ubuntu/jammy/amdgpu-install_7.0.1.70001-1_all.deb -O /tmp/amdgpu-install.deb || {
                print_warning "Failed to download ROCm installer - continuing without GPU support"
                return 0
            }
        }

        # Install the installer package
        print_info "Installing AMD GPU repository..."
        apt install -y /tmp/amdgpu-install.deb

        # Add GPG key and fix repository
        print_info "Configuring ROCm repository..."
        wget -q -O - https://repo.radeon.com/rocm/rocm.gpg.key | gpg --dearmor | tee /etc/apt/keyrings/rocm.gpg > /dev/null

        # Configure repositories with proper signing
        echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/rocm.gpg] https://repo.radeon.com/amdgpu/7.0.1/ubuntu $(lsb_release -cs 2>/dev/null || echo 'noble') main" > /etc/apt/sources.list.d/amdgpu.list
        echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/rocm.gpg] https://repo.radeon.com/rocm/apt/7.0.1 $(lsb_release -cs 2>/dev/null || echo 'noble') main" > /etc/apt/sources.list.d/rocm.list
        echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/rocm.gpg] https://repo.radeon.com/amdgpu/7.0.1/ubuntu $(lsb_release -cs 2>/dev/null || echo 'noble') proprietary" >> /etc/apt/sources.list.d/amdgpu.list

        # Update package lists
        apt update

        # Install ROCm
        print_info "Installing ROCm runtime and libraries..."
        amdgpu-install --usecase=rocm --accept-eula -y || {
            print_warning "ROCm installation failed - continuing without GPU support"
            return 0
        }

        # Clean up
        rm -f /tmp/amdgpu-install.deb

        print_success "ROCm 7.0.1 installed successfully"
        print_info "Note: A reboot may be required for full GPU support"

    elif command -v dnf >/dev/null 2>&1 || command -v yum >/dev/null 2>&1; then
        # Fedora/RHEL/Rocky/AlmaLinux
        print_info "Installing ROCm for RPM-based system..."

        # Download and install amdgpu-install
        wget -q https://repo.radeon.com/amdgpu-install/7.0.1/rhel/9.4/amdgpu-install-7.0.70001-1.el9.noarch.rpm -O /tmp/amdgpu-install.rpm || {
            print_warning "Failed to download ROCm installer for RPM system - continuing without GPU support"
            return 0
        }

        if command -v dnf >/dev/null 2>&1; then
            dnf install -y /tmp/amdgpu-install.rpm
            amdgpu-install --usecase=rocm --accept-eula -y || {
                print_warning "ROCm installation failed - continuing without GPU support"
            }
        else
            yum install -y /tmp/amdgpu-install.rpm
            amdgpu-install --usecase=rocm --accept-eula -y || {
                print_warning "ROCm installation failed - continuing without GPU support"
            }
        fi

        rm -f /tmp/amdgpu-install.rpm
        print_success "ROCm 7.0.1 installed successfully"
    else
        print_info "Unsupported distribution for automatic ROCm installation"
        print_info "Continuing without GPU support - can be manually installed later"
    fi
}

# Install Caddy web server
install_caddy() {
    print_info "Checking for Caddy web server..."

    if command -v caddy >/dev/null 2>&1; then
        print_success "Caddy is already installed"
        return
    fi

    print_info "Installing Caddy..."

    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS - use Homebrew
        if command -v brew >/dev/null 2>&1; then
            brew install caddy || {
                print_warning "Failed to install Caddy via Homebrew - continuing without Caddy"
                return 0
            }
            print_success "Caddy installed successfully"
        else
            print_warning "Homebrew not found - please install Caddy manually: https://caddyserver.com/docs/install"
            return 0
        fi
    elif command -v apt-get >/dev/null 2>&1; then
        # Debian/Ubuntu
        apt-get install -y debian-keyring debian-archive-keyring apt-transport-https curl
        curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
        curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
        apt-get update
        apt-get install -y caddy || {
            print_warning "Failed to install Caddy - continuing without Caddy"
            return 0
        }
        print_success "Caddy installed successfully"
    elif command -v dnf >/dev/null 2>&1; then
        # Fedora/RHEL
        dnf install -y 'dnf-command(copr)'
        dnf copr enable -y @caddy/caddy
        dnf install -y caddy || {
            print_warning "Failed to install Caddy - continuing without Caddy"
            return 0
        }
        print_success "Caddy installed successfully"
    elif command -v yum >/dev/null 2>&1; then
        # CentOS/older RHEL
        yum install -y yum-plugin-copr
        yum copr enable -y @caddy/caddy
        yum install -y caddy || {
            print_warning "Failed to install Caddy - continuing without Caddy"
            return 0
        }
        print_success "Caddy installed successfully"
    else
        print_warning "Unsupported package manager - please install Caddy manually: https://caddyserver.com/docs/install"
        return 0
    fi
}

# Check if running with root privileges
check_root() {
    if [ "$EUID" -ne 0 ]; then
        print_error "This script must be run with sudo or as root"
        echo "Usage: sudo ./.claude/commands/treeos-setup-production-local-amd.sh"
        exit 1
    fi
}

# Check if /opt/ontree already exists
check_existing_installation() {
    if [ -d "$ONTREE_DIR" ]; then
        print_warning "Directory $ONTREE_DIR already exists!"
        BACKUP_DIR="${ONTREE_DIR}.backup.$(date +%Y%m%d-%H%M%S)"
        echo "Attempting to backup existing installation to ${BACKUP_DIR}"

        # Try to move the directory (this might fail on macOS due to SIP)
        if mv "$ONTREE_DIR" "$BACKUP_DIR" 2>/dev/null; then
            print_success "Existing installation backed up to $BACKUP_DIR"
        else
            # If move fails, try to stop the service first (it might be using the directory)
            print_info "Cannot move directory, attempting to stop existing service first..."

            if [[ "$OSTYPE" == "darwin"* ]]; then
                # macOS - stop launchd service
                launchctl stop com.ontree.treeos 2>/dev/null || true
                launchctl unload /Library/LaunchDaemons/com.ontree.treeos.plist 2>/dev/null || true
            else
                # Linux - stop systemd service
                systemctl stop $SERVICE_NAME 2>/dev/null || true
            fi

            # Try move again after stopping service
            if mv "$ONTREE_DIR" "$BACKUP_DIR" 2>/dev/null; then
                print_success "Existing installation backed up to $BACKUP_DIR"
            else
                # If still can't move, remove the directory with warning
                print_warning "Cannot backup existing installation due to system restrictions"
                print_warning "The existing installation will be removed and replaced"
                echo -n "Removing existing installation... "
                rm -rf "$ONTREE_DIR"
                print_success "done"
            fi
        fi
    fi
}

# Download latest stable release from GitHub
download_binary() {
    print_info "Downloading latest TreeOS release from GitHub..."

    # Determine OS and architecture
    OS=""
    ARCH=""

    if [[ "$OSTYPE" == "darwin"* ]]; then
        OS="darwin"
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        OS="linux"
    else
        print_error "Unsupported operating system: $OSTYPE"
        exit 1
    fi

    # Get architecture
    MACHINE_ARCH=$(uname -m)
    case $MACHINE_ARCH in
        x86_64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            print_error "Unsupported architecture: $MACHINE_ARCH"
            exit 1
            ;;
    esac

    # Get latest release info from GitHub API
    print_info "Fetching latest release information..."
    LATEST_RELEASE=$(curl -s https://api.github.com/repos/ontree-co/treeos/releases/latest)

    if [ -z "$LATEST_RELEASE" ] || echo "$LATEST_RELEASE" | grep -q "Not Found"; then
        print_error "Could not fetch latest release information from GitHub"
        echo "Please check your internet connection and that the repository exists"
        exit 1
    fi

    # Extract version tag (macOS and Linux compatible)
    VERSION=$(echo "$LATEST_RELEASE" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)
    if [ -z "$VERSION" ]; then
        print_error "Could not determine latest version"
        exit 1
    fi

    print_info "Latest version: $VERSION"

    # Strip 'v' prefix from version for archive name
    VERSION_NUMBER=${VERSION#v}

    # Construct download URL based on GoReleaser archive format
    # Format: treeos_{version}_{os}_{arch}.tar.gz
    if [ "$ARCH" = "amd64" ]; then
        ARCH_NAME="x86_64"
    else
        ARCH_NAME="$ARCH"
    fi

    ARCHIVE_NAME="treeos_${VERSION_NUMBER}_${OS}_${ARCH_NAME}.tar.gz"
    DOWNLOAD_URL="https://github.com/ontree-co/treeos/releases/download/${VERSION}/${ARCHIVE_NAME}"

    print_info "Downloading from: $DOWNLOAD_URL"

    # Download the archive
    if command -v wget >/dev/null 2>&1; then
        wget -q --show-progress "$DOWNLOAD_URL" -O "/tmp/${ARCHIVE_NAME}" || {
            print_error "Failed to download TreeOS archive"
            echo "URL: $DOWNLOAD_URL"
            exit 1
        }
    elif command -v curl >/dev/null 2>&1; then
        curl -L --progress-bar "$DOWNLOAD_URL" -o "/tmp/${ARCHIVE_NAME}" || {
            print_error "Failed to download TreeOS archive"
            echo "URL: $DOWNLOAD_URL"
            exit 1
        }
    else
        print_error "Neither wget nor curl is available. Please install one of them."
        exit 1
    fi

    # Extract the binary from archive
    print_info "Extracting TreeOS binary..."
    # The archive contains treeos-{os}-{arch} format binary
    tar -xzf "/tmp/${ARCHIVE_NAME}" -C /tmp || {
        print_error "Failed to extract TreeOS archive"
        exit 1
    }

    # Find and rename the extracted binary
    if [ -f "/tmp/treeos-${OS}-${ARCH}" ]; then
        mv "/tmp/treeos-${OS}-${ARCH}" "/tmp/$BINARY_NAME"
    elif [ -f "/tmp/treeos" ]; then
        # Fallback if archive contains just 'treeos'
        # Only move if the names are different, otherwise it's already correct
        if [ "/tmp/treeos" != "/tmp/$BINARY_NAME" ]; then
            mv "/tmp/treeos" "/tmp/$BINARY_NAME"
        fi
    else
        print_error "Could not find TreeOS binary in extracted archive"
        exit 1
    fi

    # Make binary executable
    chmod +x "/tmp/$BINARY_NAME"

    # Clean up archive
    rm -f "/tmp/${ARCHIVE_NAME}"

    print_success "TreeOS $VERSION downloaded and extracted successfully"
}

# Ensure Docker and Docker Compose are available
check_docker() {
    if ! command -v docker >/dev/null 2>&1; then
        print_error "Docker is required but not found in PATH."
        echo "Install Docker: https://docs.docker.com/engine/install/"
        exit 1
    fi

    # Check for Docker Compose v2 support (required)
    if docker compose version >/dev/null 2>&1; then
        print_success "Docker Compose v2 is available"
    elif command -v docker-compose >/dev/null 2>&1; then
        # v1 is installed but we need v2
        print_error "Docker Compose v2 is required (found v1 standalone)"
        echo "TreeOS requires Docker Compose v2 (plugin version), not the standalone v1"
        echo "Install Docker Compose v2:"
        echo "  Ubuntu/Debian: sudo apt update && sudo apt install docker-compose-v2"
        echo "  Via Docker repo: sudo apt update && sudo apt install docker-compose-plugin"
        echo "  macOS: Install Docker Desktop which includes Compose v2"
        echo "  Other: https://docs.docker.com/compose/install/"
        exit 1
    else
        print_error "Docker Compose v2 is required but not found."
        echo "Install Docker Compose v2:"
        echo "  Ubuntu/Debian: sudo apt update && sudo apt install docker-compose-v2"
        echo "  Via Docker repo: sudo apt update && sudo apt install docker-compose-plugin"
        echo "  macOS: Install Docker Desktop which includes Compose v2"
        echo "  Other: https://docs.docker.com/compose/install/"
        exit 1
    fi

    # Check if Docker daemon is running
    if ! docker info >/dev/null 2>&1; then
        print_error "Docker daemon is not running or not accessible."
        if [[ "$OSTYPE" == "darwin"* ]]; then
            echo "Please start Docker Desktop"
        else
            echo "Please start Docker: sudo systemctl start docker"
        fi
        exit 1
    fi

    # Docker group membership will be configured after user creation
}

# Create ontree user if it doesn't exist
create_user() {
    print_info "Checking for user '$ONTREE_USER'..."

    if id "$ONTREE_USER" &>/dev/null; then
        print_success "User '$ONTREE_USER' already exists"
    else
        print_info "Creating user '$ONTREE_USER'..."

        # Create user with home directory
        if [[ "$OSTYPE" == "darwin"* ]]; then
            # macOS user creation
            # Find next available UID > 500
            LAST_UID=$(dscl . -list /Users UniqueID | awk '{print $2}' | sort -n | tail -1)
            NEW_UID=$((LAST_UID + 1))

            # Create user
            dscl . -create /Users/$ONTREE_USER
            dscl . -create /Users/$ONTREE_USER UserShell /usr/bin/false
            dscl . -create /Users/$ONTREE_USER UniqueID $NEW_UID
            dscl . -create /Users/$ONTREE_USER PrimaryGroupID 20  # staff group
            dscl . -create /Users/$ONTREE_USER NFSHomeDirectory $ONTREE_DIR
            dscl . -create /Users/$ONTREE_USER RealName "TreeOS Service User"

            # Hide user from login window
            dscl . -create /Users/$ONTREE_USER IsHidden 1
        else
            # Linux user creation
            useradd -r -s /bin/false -d $ONTREE_DIR -m $ONTREE_USER || {
                # If useradd fails, try adduser (for some distros)
                adduser --system --shell /bin/false --home $ONTREE_DIR --no-create-home $ONTREE_USER
            }
        fi

        print_success "User '$ONTREE_USER' created"
    fi
}

# Configure docker group membership for ontree user
configure_docker_group() {
    # Add ontree user to docker group (Linux only)
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        if getent group docker >/dev/null 2>&1; then
            if ! id -nG "$ONTREE_USER" 2>/dev/null | grep -qw docker; then
                print_info "Adding user '$ONTREE_USER' to docker group..."
                usermod -aG docker "$ONTREE_USER"
                print_success "User '$ONTREE_USER' added to docker group"
                print_info "Note: You may need to restart the TreeOS service for group changes to take effect"
            else
                print_success "User '$ONTREE_USER' already in docker group"
            fi
        fi
    fi
}

# Create directory structure
create_directories() {
    print_info "Creating directory structure..."

    # Create main directories
    mkdir -p "$ONTREE_DIR"
    mkdir -p "$ONTREE_DIR/apps"
    mkdir -p "$ONTREE_DIR/shared/ollama"
    mkdir -p "$ONTREE_DIR/logs"

    # Set ownership (use appropriate group for OS)
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS uses staff group
        chown -R $ONTREE_USER:staff "$ONTREE_DIR"
    else
        # Linux uses same name as user
        chown -R $ONTREE_USER:$ONTREE_USER "$ONTREE_DIR"
    fi

    # Set permissions
    chmod 755 "$ONTREE_DIR"
    chmod 755 "$ONTREE_DIR/apps"
    chmod 755 "$ONTREE_DIR/shared"
    chmod 755 "$ONTREE_DIR/shared/ollama"
    chmod 755 "$ONTREE_DIR/logs"

    print_success "Directory structure created"
}

# Install binary
install_binary() {
    print_info "Installing TreeOS binary..."

    # Copy binary from /tmp to installation directory
    cp "/tmp/$BINARY_NAME" "$ONTREE_DIR/$BINARY_NAME"

    # Set ownership and permissions (use appropriate group for OS)
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS uses staff group
        chown $ONTREE_USER:staff "$ONTREE_DIR/$BINARY_NAME"
    else
        # Linux uses same name as user
        chown $ONTREE_USER:$ONTREE_USER "$ONTREE_DIR/$BINARY_NAME"
    fi
    chmod 755 "$ONTREE_DIR/$BINARY_NAME"

    # Clean up downloaded binary
    rm -f "/tmp/$BINARY_NAME"

    print_success "Binary installed to $ONTREE_DIR/$BINARY_NAME"
}

# Install systemd service (Linux)
install_systemd_service() {
    print_info "Installing systemd service..."

    # Create systemd service file
    cat > /etc/systemd/system/treeos.service << 'EOF'
[Unit]
Description=TreeOS Application Server
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=ontree
Group=ontree
WorkingDirectory=/opt/ontree
ExecStart=/opt/ontree/treeos
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=treeos

# Environment variables
Environment="HOME=/opt/ontree"
Environment="PATH=/usr/local/bin:/usr/bin:/bin"

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/ontree

[Install]
WantedBy=multi-user.target
EOF

    # Reload systemd daemon
    systemctl daemon-reload

    # Enable service to start on boot
    systemctl enable $SERVICE_NAME

    print_success "Systemd service installed and enabled"

    # Start the service
    print_info "Starting TreeOS service..."
    systemctl start $SERVICE_NAME

    # Check if service started successfully
    sleep 2
    if systemctl is-active --quiet $SERVICE_NAME; then
        print_success "TreeOS service is running"
    else
        print_warning "Service may still be starting. Check status with: sudo systemctl status $SERVICE_NAME"
    fi
}

# Install launchd service (macOS)
install_launchd_service() {
    print_info "Installing launchd service..."

    # Create launchd plist file
    cat > /Library/LaunchDaemons/com.ontree.treeos.plist << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.ontree.treeos</string>
    <key>ProgramArguments</key>
    <array>
        <string>${ONTREE_DIR}/treeos</string>
    </array>
    <key>WorkingDirectory</key>
    <string>${ONTREE_DIR}</string>
    <key>UserName</key>
    <string>ontree</string>
    <key>GroupName</key>
    <string>staff</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <dict>
        <key>SuccessfulExit</key>
        <false/>
    </dict>
    <key>StandardOutPath</key>
    <string>${ONTREE_DIR}/logs/treeos.log</string>
    <key>StandardErrorPath</key>
    <string>${ONTREE_DIR}/logs/treeos.error.log</string>
    <key>EnvironmentVariables</key>
    <dict>
        <key>HOME</key>
        <string>${ONTREE_DIR}</string>
        <key>PATH</key>
        <string>/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>
</dict>
</plist>
EOF

    # Set correct ownership and permissions
    chown root:wheel /Library/LaunchDaemons/com.ontree.treeos.plist
    chmod 644 /Library/LaunchDaemons/com.ontree.treeos.plist

    # Unload if already loaded (in case of reinstall)
    launchctl unload /Library/LaunchDaemons/com.ontree.treeos.plist 2>/dev/null || true

    # Load the service
    launchctl load -w /Library/LaunchDaemons/com.ontree.treeos.plist

    print_success "Launchd service installed and loaded"

    # Check if service started successfully
    sleep 2
    if launchctl list | grep -q com.ontree.treeos; then
        print_success "TreeOS service is running"
    else
        print_warning "Service may still be starting. Check status with: sudo launchctl list | grep com.ontree.treeos"
    fi
}

# Install service based on OS
install_service() {
    if [[ "$OSTYPE" == "darwin"* ]]; then
        install_launchd_service
    else
        # Assume Linux with systemd
        if command -v systemctl &> /dev/null; then
            install_systemd_service
        else
            print_error "systemd not found! Manual service configuration required."
            echo "TreeOS has been installed but you'll need to configure it to start automatically."
            return 1
        fi
    fi
}

# Main installation flow
main() {
    echo "======================================"
    echo "   TreeOS Production Setup (Auto)    "
    echo "======================================"
    echo ""

    # Perform checks
    check_root
    check_existing_installation
    check_docker

    echo ""
    print_info "Starting automatic installation..."
    echo ""

    # Download latest release
    download_binary

    # Execute installation steps
    create_user
    configure_docker_group
    install_caddy
    install_rocm
    configure_gpu_permissions
    create_directories
    install_binary
    install_service

    echo ""
    echo "======================================"
    print_success "TreeOS installation complete!"
    echo ""
    echo "TreeOS is now running in production mode."
    echo ""
    echo "Access the web interface at:"
    echo "  http://localhost:3000"
    echo ""
    echo "Service management commands:"
    if [[ "$OSTYPE" == "darwin"* ]]; then
        echo "  Start:   sudo launchctl load -w /Library/LaunchDaemons/com.ontree.treeos.plist"
        echo "  Stop:    sudo launchctl unload /Library/LaunchDaemons/com.ontree.treeos.plist"
        echo "  Status:  sudo launchctl list | grep com.ontree.treeos"
        echo "  Logs:    log show --style compact --predicate 'subsystem == \"com.ontree.treeos\"' --last 1h"
    else
        echo "  Start:   sudo systemctl start $SERVICE_NAME"
        echo "  Stop:    sudo systemctl stop $SERVICE_NAME"
        echo "  Status:  sudo systemctl status $SERVICE_NAME"
        echo "  Logs:    sudo journalctl -u $SERVICE_NAME -f"
    fi
    echo "======================================"
}

# Run main function
main