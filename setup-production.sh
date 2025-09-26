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
