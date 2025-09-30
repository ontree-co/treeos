#!/bin/bash
# TreeOS Production Setup Script
# This script sets up TreeOS to run in production mode with proper service management
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
    echo -e "${RED}❌ Error: $1${NC}" >&2
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_info() {
    echo -e "${YELLOW}ℹ️  $1${NC}"
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

# Install AMD ROCm 7.0.1 for GPU support (optional)
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

    print_info "AMD GPU detected. Would you like to install ROCm 7.0.1 for GPU acceleration?"
    read -p "Install ROCm? (y/n): " -n 1 -r
    echo ""

    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Skipping ROCm installation"
        return
    fi

    print_info "Installing ROCm 7.0.1..."

    # Detect Ubuntu/Debian
    if command -v apt-get >/dev/null 2>&1; then
        # Download amdgpu-install package
        print_info "Downloading AMD GPU installer..."
        wget -q https://repo.radeon.com/amdgpu-install/7.0.1/ubuntu/noble/amdgpu-install_7.0.1.70001-1_all.deb -O /tmp/amdgpu-install.deb || {
            # Fallback to jammy if noble fails
            wget -q https://repo.radeon.com/amdgpu-install/7.0.1/ubuntu/jammy/amdgpu-install_7.0.1.70001-1_all.deb -O /tmp/amdgpu-install.deb || {
                print_error "Failed to download ROCm installer"
                return 1
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
            print_error "ROCm installation failed"
            return 1
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
            print_error "Failed to download ROCm installer for RPM system"
            return 1
        }

        if command -v dnf >/dev/null 2>&1; then
            dnf install -y /tmp/amdgpu-install.rpm
            amdgpu-install --usecase=rocm --accept-eula -y
        else
            yum install -y /tmp/amdgpu-install.rpm
            amdgpu-install --usecase=rocm --accept-eula -y
        fi

        rm -f /tmp/amdgpu-install.rpm
        print_success "ROCm 7.0.1 installed successfully"
    else
        print_info "Unsupported distribution for automatic ROCm installation"
        print_info "Please install ROCm 7.0.1 manually from: https://rocm.docs.amd.com/"
    fi
}

# Check if running with root privileges
check_root() {
    if [ "$EUID" -ne 0 ]; then
        print_error "This script must be run with sudo or as root"
        echo "Usage: sudo ./setup-production.sh"
        exit 1
    fi
}

# Check if /opt/ontree already exists
check_existing_installation() {
    if [ -d "$ONTREE_DIR" ]; then
        print_error "Directory $ONTREE_DIR already exists!"
        echo "This appears to be an existing installation."
        echo "To start fresh, please manually remove or rename the existing directory:"
        echo "  sudo mv $ONTREE_DIR ${ONTREE_DIR}.backup"
        echo "or"
        echo "  sudo rm -rf $ONTREE_DIR"
        exit 1
    fi
}

# Check if binary exists in current directory
check_binary() {
    if [ ! -f "./$BINARY_NAME" ]; then
        print_error "Binary '$BINARY_NAME' not found in current directory!"
        echo "Please run this script from the directory containing the TreeOS binary."
        exit 1
    fi
}

# Ensure Podman 4+ with built-in compose is available
check_podman() {
    if ! command -v podman >/dev/null 2>&1; then
        print_error "Podman is required but not found in PATH."
        echo "Install Podman 4.0 or later: https://podman.io/docs/installation"
        exit 1
    fi

    if ! podman compose --help >/dev/null 2>&1; then
        print_error "Podman 4+ with built-in compose support is required."
        echo "TreeOS requires Podman 4.0 or later with built-in compose."
        echo "Please upgrade your Podman installation."
        echo "Verify with: podman compose version"
        exit 1
    fi

    print_success "Podman compose is available"

    # Check and install podman-plugins for DNS support
    print_step "Checking Podman DNS plugins..."
    if [[ "$OS" == "Linux" ]]; then
        if command -v apt-get >/dev/null 2>&1; then
            # Debian/Ubuntu
            if ! dpkg -l | grep -q "golang-github-containernetworking-plugin-dnsname"; then
                print_step "Installing Podman DNS plugins..."
                sudo apt-get update >/dev/null 2>&1
                sudo apt-get install -y golang-github-containernetworking-plugin-dnsname containernetworking-plugins >/dev/null 2>&1 || {
                    print_warning "Could not install DNS plugins. Container DNS resolution might not work properly."
                    echo "Manual installation: sudo apt-get install golang-github-containernetworking-plugin-dnsname"
                }
            fi
        elif command -v dnf >/dev/null 2>&1; then
            # Fedora/RHEL/Rocky/AlmaLinux
            if ! rpm -qa | grep -q "podman-plugins"; then
                print_step "Installing Podman DNS plugins..."
                sudo dnf install -y podman-plugins >/dev/null 2>&1 || {
                    print_warning "Could not install DNS plugins. Container DNS resolution might not work properly."
                    echo "Manual installation: sudo dnf install podman-plugins"
                }
            fi
        fi
        print_success "Podman DNS plugins are available"
    fi
}

# Create ontree user if it doesn't exist
create_user() {
    print_info "Checking for user '$ONTREE_USER'..."

    if id "$ONTREE_USER" &>/dev/null; then
        print_success "User '$ONTREE_USER' already exists"
    else
        print_info "Creating user '$ONTREE_USER'..."

        # Create user with home directory in /opt/ontree
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

# Create directory structure
create_directories() {
    print_info "Creating directory structure..."

    # Create main directories
    mkdir -p "$ONTREE_DIR"
    mkdir -p "$ONTREE_DIR/apps"
    mkdir -p "$ONTREE_DIR/shared/ollama"
    mkdir -p "$ONTREE_DIR/logs"

    # Set ownership
    chown -R $ONTREE_USER:$ONTREE_USER "$ONTREE_DIR"

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

    # Copy binary to /opt/ontree
    cp "./$BINARY_NAME" "$ONTREE_DIR/$BINARY_NAME"

    # Set ownership and permissions
    chown $ONTREE_USER:$ONTREE_USER "$ONTREE_DIR/$BINARY_NAME"
    chmod 755 "$ONTREE_DIR/$BINARY_NAME"

    print_success "Binary installed to $ONTREE_DIR/$BINARY_NAME"
}

# Install systemd service (Linux)
install_systemd_service() {
    print_info "Installing systemd service..."

    # Check if treeos.service exists in current directory
    if [ ! -f "./treeos.service" ]; then
        print_error "treeos.service file not found in current directory!"
        echo "Please ensure the service file is present."
        return 1
    fi

    # Copy service file
    cp ./treeos.service /etc/systemd/system/

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
        print_error "Failed to start TreeOS service"
        echo "Check logs with: sudo journalctl -u $SERVICE_NAME -n 50"
        return 1
    fi
}

# Install launchd service (macOS)
install_launchd_service() {
    print_info "Installing launchd service..."

    # Check if plist file exists in current directory
    if [ ! -f "./com.ontree.treeos.plist" ]; then
        print_error "com.ontree.treeos.plist file not found in current directory!"
        echo "Please ensure the plist file is present."
        return 1
    fi

    # Copy plist file to LaunchDaemons
    cp ./com.ontree.treeos.plist /Library/LaunchDaemons/

    # Set correct ownership and permissions
    chown root:wheel /Library/LaunchDaemons/com.ontree.treeos.plist
    chmod 644 /Library/LaunchDaemons/com.ontree.treeos.plist

    # Load the service
    launchctl load -w /Library/LaunchDaemons/com.ontree.treeos.plist

    print_success "Launchd service installed and loaded"

    # Check if service started successfully
    sleep 2
    if launchctl list | grep -q com.ontree.treeos; then
        print_success "TreeOS service is running"
    else
        print_error "Failed to start TreeOS service"
        echo "Check logs with: sudo launchctl log show --style compact --predicate 'subsystem == \"com.ontree.treeos\"'"
        return 1
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
    echo "   TreeOS Production Setup Script    "
    echo "======================================"
    echo ""

    # Perform checks
    check_root
    check_existing_installation
    check_binary
    check_podman

    echo "This script will:"
    echo "  1. Create user '$ONTREE_USER' (if not exists)"
    echo "  2. Create directory structure at $ONTREE_DIR"
    echo "  3. Install TreeOS binary"
    echo "  4. Configure service to run at startup"
    echo ""
    read -p "Continue with installation? (y/n): " -n 1 -r
    echo ""

    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Installation cancelled."
        exit 0
    fi

    echo ""

    # Execute installation steps
    create_user
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
