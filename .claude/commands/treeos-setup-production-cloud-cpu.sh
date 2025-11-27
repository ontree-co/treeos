#!/bin/bash
# TreeOS Production Setup Script (Cloud CPU)
# Lean version for cloud VPS/dedicated servers with CPU-only inference
# No GPU drivers, no ROCm - just the essentials
# Must be run with sudo/root privileges

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
ONTREE_DIR="/opt/ontree"
ONTREE_USER="ontree"
BINARY_NAME="treeos"
SERVICE_NAME="treeos"

# Helper functions
print_error() {
    echo -e "${RED}Error: $1${NC}" >&2
}

print_success() {
    echo -e "${GREEN}$1${NC}"
}

print_info() {
    echo -e "${YELLOW}$1${NC}"
}

print_step() {
    echo -e "${YELLOW}-> $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}Warning: $1${NC}"
}

# Check if running with root privileges
check_root() {
    if [ "$EUID" -ne 0 ]; then
        print_error "This script must be run with sudo or as root"
        echo "Usage: sudo ./.claude/commands/treeos-setup-production-cloud-cpu.sh"
        exit 1
    fi
}

# Check if /opt/ontree already exists
check_existing_installation() {
    if [ -d "$ONTREE_DIR" ]; then
        print_warning "Directory $ONTREE_DIR already exists!"
        BACKUP_DIR="${ONTREE_DIR}.backup.$(date +%Y%m%d-%H%M%S)"
        echo "Attempting to backup existing installation to ${BACKUP_DIR}"

        # Try to stop the service first
        systemctl stop $SERVICE_NAME 2>/dev/null || true

        if mv "$ONTREE_DIR" "$BACKUP_DIR" 2>/dev/null; then
            print_success "Existing installation backed up to $BACKUP_DIR"
        else
            print_warning "Cannot backup existing installation"
            print_warning "The existing installation will be removed and replaced"
            echo -n "Removing existing installation... "
            rm -rf "$ONTREE_DIR"
            print_success "done"
        fi
    fi
}

# Download latest stable release from GitHub
download_binary() {
    print_info "Downloading latest TreeOS release from GitHub..."

    # Cloud servers are Linux amd64
    OS="linux"
    ARCH="amd64"

    # Get latest release info from GitHub API
    print_info "Fetching latest release information..."
    LATEST_RELEASE=$(curl -s https://api.github.com/repos/ontree-co/treeos/releases/latest)

    if [ -z "$LATEST_RELEASE" ] || echo "$LATEST_RELEASE" | grep -q "Not Found"; then
        print_error "Could not fetch latest release information from GitHub"
        echo "Please check your internet connection and that the repository exists"
        exit 1
    fi

    # Extract version tag
    VERSION=$(echo "$LATEST_RELEASE" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)
    if [ -z "$VERSION" ]; then
        print_error "Could not determine latest version"
        exit 1
    fi

    print_info "Latest version: $VERSION"

    # Strip 'v' prefix from version for archive name
    VERSION_NUMBER=${VERSION#v}

    # Construct download URL based on GoReleaser archive format
    ARCHIVE_NAME="treeos_${VERSION_NUMBER}_${OS}_x86_64.tar.gz"
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
    tar -xzf "/tmp/${ARCHIVE_NAME}" -C /tmp || {
        print_error "Failed to extract TreeOS archive"
        exit 1
    }

    # Find and rename the extracted binary
    if [ -f "/tmp/treeos-${OS}-${ARCH}" ]; then
        mv "/tmp/treeos-${OS}-${ARCH}" "/tmp/$BINARY_NAME"
    elif [ -f "/tmp/treeos" ]; then
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
        print_error "Docker Compose v2 is required (found v1 standalone)"
        echo "TreeOS requires Docker Compose v2 (plugin version), not the standalone v1"
        echo "Install Docker Compose v2:"
        echo "  Ubuntu/Debian: sudo apt update && sudo apt install docker-compose-v2"
        echo "  Via Docker repo: sudo apt update && sudo apt install docker-compose-plugin"
        exit 1
    else
        print_error "Docker Compose v2 is required but not found."
        echo "Install Docker Compose v2:"
        echo "  Ubuntu/Debian: sudo apt update && sudo apt install docker-compose-v2"
        echo "  Via Docker repo: sudo apt update && sudo apt install docker-compose-plugin"
        exit 1
    fi

    # Check if Docker daemon is running
    if ! docker info >/dev/null 2>&1; then
        print_error "Docker daemon is not running or not accessible."
        echo "Please start Docker: sudo systemctl start docker"
        exit 1
    fi
}

# Create ontree user if it doesn't exist
create_user() {
    print_info "Checking for user '$ONTREE_USER'..."

    if id "$ONTREE_USER" &>/dev/null; then
        print_success "User '$ONTREE_USER' already exists"
    else
        print_info "Creating user '$ONTREE_USER'..."
        useradd -r -s /bin/false -d $ONTREE_DIR -m $ONTREE_USER || {
            adduser --system --shell /bin/false --home $ONTREE_DIR --no-create-home $ONTREE_USER
        }
        print_success "User '$ONTREE_USER' created"
    fi
}

# Configure docker group membership for ontree user
configure_docker_group() {
    if getent group docker >/dev/null 2>&1; then
        if ! id -nG "$ONTREE_USER" 2>/dev/null | grep -qw docker; then
            print_info "Adding user '$ONTREE_USER' to docker group..."
            usermod -aG docker "$ONTREE_USER"
            print_success "User '$ONTREE_USER' added to docker group"
        else
            print_success "User '$ONTREE_USER' already in docker group"
        fi
    fi
}

# Create directory structure
create_directories() {
    print_info "Creating directory structure..."

    mkdir -p "$ONTREE_DIR"
    mkdir -p "$ONTREE_DIR/apps"
    mkdir -p "$ONTREE_DIR/shared/ollama"
    mkdir -p "$ONTREE_DIR/logs"

    chown -R $ONTREE_USER:$ONTREE_USER "$ONTREE_DIR"

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

    cp "/tmp/$BINARY_NAME" "$ONTREE_DIR/$BINARY_NAME"
    chown $ONTREE_USER:$ONTREE_USER "$ONTREE_DIR/$BINARY_NAME"
    chmod 755 "$ONTREE_DIR/$BINARY_NAME"

    rm -f "/tmp/$BINARY_NAME"

    print_success "Binary installed to $ONTREE_DIR/$BINARY_NAME"
}

# Install systemd service
install_systemd_service() {
    print_info "Installing systemd service..."

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

    systemctl daemon-reload
    systemctl enable $SERVICE_NAME

    print_success "Systemd service installed and enabled"

    print_info "Starting TreeOS service..."
    systemctl start $SERVICE_NAME

    sleep 2
    if systemctl is-active --quiet $SERVICE_NAME; then
        print_success "TreeOS service is running"
    else
        print_warning "Service may still be starting. Check status with: sudo systemctl status $SERVICE_NAME"
    fi
}

# Main installation flow
main() {
    echo "======================================"
    echo "  TreeOS Production Setup (Cloud CPU) "
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
    create_directories
    install_binary
    install_systemd_service

    echo ""
    echo "======================================"
    print_success "TreeOS installation complete!"
    echo ""
    echo "TreeOS is now running in production mode (CPU inference)."
    echo ""
    echo "Access the web interface at:"
    echo "  http://localhost:3000"
    echo ""
    echo "Service management commands:"
    echo "  Start:   sudo systemctl start $SERVICE_NAME"
    echo "  Stop:    sudo systemctl stop $SERVICE_NAME"
    echo "  Status:  sudo systemctl status $SERVICE_NAME"
    echo "  Logs:    sudo journalctl -u $SERVICE_NAME -f"
    echo "======================================"
}

# Run main function
main
