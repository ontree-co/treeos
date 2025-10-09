#!/bin/bash
# TreeOS Development Environment Setup Script
# This script installs asdf and required development tools
# Must be run with sudo on Linux (for package manager), no sudo needed on macOS

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

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

# Detect OS and package manager
detect_os() {
    if [[ "$OSTYPE" == "darwin"* ]]; then
        OS="macos"
        PKG_MANAGER="brew"
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        OS="linux"
        # Detect Linux package manager
        if command -v apt-get >/dev/null 2>&1; then
            PKG_MANAGER="apt"
        elif command -v dnf >/dev/null 2>&1; then
            PKG_MANAGER="dnf"
        elif command -v yum >/dev/null 2>&1; then
            PKG_MANAGER="yum"
        else
            print_error "No supported package manager found (apt, dnf, yum)"
            exit 1
        fi
    else
        print_error "Unsupported operating system: $OSTYPE"
        exit 1
    fi

    print_info "Detected OS: $OS"
    print_info "Package manager: $PKG_MANAGER"
}

# Check prerequisites
check_prerequisites() {
    print_step "Checking prerequisites..."

    # Check for git
    if ! command -v git >/dev/null 2>&1; then
        print_error "git is required but not installed"
        echo "Install: $PKG_MANAGER install git"
        exit 1
    fi
    print_success "git is installed"

    # Check for curl
    if ! command -v curl >/dev/null 2>&1; then
        print_error "curl is required but not installed"
        echo "Install: $PKG_MANAGER install curl"
        exit 1
    fi
    print_success "curl is installed"

    # Check for gpg (needed by nodejs plugin)
    if ! command -v gpg >/dev/null 2>&1; then
        print_warning "gpg is not installed (needed for nodejs plugin)"
        echo "Install: $PKG_MANAGER install gpg"
        if [[ "$OS" == "linux" ]]; then
            print_info "Attempting to install gpg..."
            if [[ "$PKG_MANAGER" == "apt" ]]; then
                apt-get update && apt-get install -y gnupg
            elif [[ "$PKG_MANAGER" == "dnf" ]] || [[ "$PKG_MANAGER" == "yum" ]]; then
                $PKG_MANAGER install -y gnupg2
            fi
        fi
    else
        print_success "gpg is installed"
    fi

    # On macOS, check for Homebrew
    if [[ "$OS" == "macos" ]] && ! command -v brew >/dev/null 2>&1; then
        print_error "Homebrew is required on macOS but not installed"
        echo "Install from: https://brew.sh"
        exit 1
    fi
}

# Check if running with appropriate privileges
check_privileges() {
    if [[ "$OS" == "linux" ]]; then
        if [ "$EUID" -eq 0 ]; then
            print_warning "Running with sudo detected"
            print_info "On Linux, asdf will be installed to the user's home directory (~/.asdf)"
            print_info "Script will handle proper file ownership automatically"
        else
            print_success "Running as regular user (recommended)"
        fi
    else
        if [ "$EUID" -eq 0 ]; then
            print_warning "Running as root on macOS is not recommended"
            print_info "asdf and Homebrew should be installed as regular user"
        fi
    fi
}

# Install asdf via Homebrew (macOS) or git clone (Linux)
install_asdf() {
    print_step "Installing asdf..."

    # Check if asdf is already installed
    if command -v asdf >/dev/null 2>&1; then
        print_success "asdf is already installed"
        ASDF_VERSION=$(asdf version | head -n1)
        print_info "Version: $ASDF_VERSION"
        return
    fi

    # Install based on OS - prioritize lowest friction on each platform
    if [[ "$OS" == "macos" ]]; then
        print_info "Installing asdf via Homebrew (lowest friction on macOS)..."
        brew install asdf
        print_success "asdf installed via Homebrew"
    else
        # Linux: use git clone (lowest friction, works everywhere)
        print_info "Installing asdf via git clone (lowest friction on Linux)..."

        # Determine installation directory
        ASDF_DIR="$HOME/.asdf"
        if [ "$EUID" -eq 0 ] && [[ -n "$SUDO_USER" ]]; then
            # Running as sudo, install for the real user
            REAL_HOME=$(eval echo "~$SUDO_USER")
            ASDF_DIR="$REAL_HOME/.asdf"
        fi

        # Clone asdf if not already present
        if [ -d "$ASDF_DIR" ]; then
            print_warning "asdf directory already exists at $ASDF_DIR"
            print_info "Updating existing installation..."
            cd "$ASDF_DIR"
            git fetch origin --tags
            git checkout "$(git describe --abbrev=0 --tags)"
            cd - > /dev/null
        else
            print_info "Cloning asdf to $ASDF_DIR..."
            git clone https://github.com/asdf-vm/asdf.git "$ASDF_DIR" --branch v0.14.0

            # If running as sudo, fix ownership
            if [ "$EUID" -eq 0 ] && [[ -n "$SUDO_USER" ]]; then
                chown -R "$SUDO_USER:$SUDO_USER" "$ASDF_DIR"
            fi
        fi

        print_success "asdf installed to $ASDF_DIR"
    fi
}

# Configure shell for asdf
configure_shell() {
    print_step "Configuring shell for asdf..."

    # Determine which shell config file to use
    SHELL_CONFIG=""
    if [[ -n "$BASH_VERSION" ]] || [[ "$SHELL" == *"bash"* ]]; then
        SHELL_CONFIG="$HOME/.bashrc"
    elif [[ -n "$ZSH_VERSION" ]] || [[ "$SHELL" == *"zsh"* ]]; then
        SHELL_CONFIG="$HOME/.zshrc"
    else
        print_warning "Could not determine shell type, using ~/.bashrc"
        SHELL_CONFIG="$HOME/.bashrc"
    fi

    # If running as sudo, get the real user's home directory
    if [[ "$OS" == "linux" ]] && [ "$EUID" -eq 0 ]; then
        if [[ -n "$SUDO_USER" ]]; then
            REAL_HOME=$(eval echo "~$SUDO_USER")
            if [[ -f "$REAL_HOME/.bashrc" ]]; then
                SHELL_CONFIG="$REAL_HOME/.bashrc"
            elif [[ -f "$REAL_HOME/.zshrc" ]]; then
                SHELL_CONFIG="$REAL_HOME/.zshrc"
            fi
        fi
    fi

    print_info "Shell config file: $SHELL_CONFIG"

    # Check if asdf is already sourced
    if grep -q "asdf.sh" "$SHELL_CONFIG" 2>/dev/null; then
        print_success "asdf is already configured in $SHELL_CONFIG"
        return
    fi

    # Add asdf initialization to shell config
    print_info "Adding asdf to $SHELL_CONFIG..."

    if [[ "$OS" == "macos" ]]; then
        # macOS Homebrew installation
        echo '' >> "$SHELL_CONFIG"
        echo '# asdf version manager' >> "$SHELL_CONFIG"
        echo '. $(brew --prefix asdf)/libexec/asdf.sh' >> "$SHELL_CONFIG"
    else
        # Linux git clone installation
        echo '' >> "$SHELL_CONFIG"
        echo '# asdf version manager' >> "$SHELL_CONFIG"
        echo '. "$HOME/.asdf/asdf.sh"' >> "$SHELL_CONFIG"
    fi

    print_success "asdf configured in $SHELL_CONFIG"
    print_warning "You will need to restart your terminal or run: source $SHELL_CONFIG"
}

# Source asdf for this script
source_asdf() {
    print_step "Loading asdf for this session..."

    if [[ "$OS" == "macos" ]]; then
        if [ -f "$(brew --prefix asdf)/libexec/asdf.sh" ]; then
            . "$(brew --prefix asdf)/libexec/asdf.sh"
            print_success "asdf loaded"
        else
            print_error "Could not find asdf.sh after installation"
            exit 1
        fi
    else
        # Determine asdf directory (considering sudo usage)
        ASDF_DIR="$HOME/.asdf"
        if [ "$EUID" -eq 0 ] && [[ -n "$SUDO_USER" ]]; then
            REAL_HOME=$(eval echo "~$SUDO_USER")
            ASDF_DIR="$REAL_HOME/.asdf"
        fi

        if [ -f "$ASDF_DIR/asdf.sh" ]; then
            . "$ASDF_DIR/asdf.sh"
            print_success "asdf loaded"
        else
            print_error "Could not find asdf.sh at $ASDF_DIR"
            exit 1
        fi
    fi
}

# Install asdf plugins
install_plugins() {
    print_step "Installing asdf plugins..."

    # golang plugin
    if asdf plugin list | grep -q "^golang$"; then
        print_success "golang plugin already installed"
    else
        print_info "Adding golang plugin..."
        asdf plugin add golang
        print_success "golang plugin added"
    fi

    # golangci-lint plugin
    if asdf plugin list | grep -q "^golangci-lint$"; then
        print_success "golangci-lint plugin already installed"
    else
        print_info "Adding golangci-lint plugin..."
        asdf plugin add golangci-lint
        print_success "golangci-lint plugin added"
    fi

    # nodejs plugin
    if asdf plugin list | grep -q "^nodejs$"; then
        print_success "nodejs plugin already installed"
    else
        print_info "Adding nodejs plugin..."
        asdf plugin add nodejs
        print_success "nodejs plugin added"
    fi

    # Import nodejs GPG keys
    print_info "Importing nodejs release team GPG keys..."
    bash -c '${ASDF_DATA_DIR:=$HOME/.asdf}/plugins/nodejs/bin/import-release-team-keyring' 2>/dev/null || {
        print_warning "Could not import nodejs GPG keys automatically"
        print_info "This may not be critical - installation will continue"
    }
}

# Install tools from .tool-versions
install_tools() {
    print_step "Installing tools from .tool-versions..."

    # Change to repository root (where .tool-versions is)
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

    cd "$REPO_ROOT"
    print_info "Repository root: $REPO_ROOT"

    if [ ! -f ".tool-versions" ]; then
        print_error ".tool-versions file not found in repository root"
        exit 1
    fi

    print_info "Reading .tool-versions:"
    cat .tool-versions

    # Install all tools specified in .tool-versions
    print_info "Running: asdf install"
    asdf install

    print_success "All tools installed successfully"
}

# Verify installation
verify_installation() {
    print_step "Verifying installation..."

    # Change to repository root
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
    cd "$REPO_ROOT"

    echo ""
    print_info "Installed versions:"
    asdf current

    echo ""
    print_step "Checking individual tools..."

    # Check Go
    if command -v go >/dev/null 2>&1; then
        GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
        print_success "Go: $GO_VERSION"
    else
        print_warning "Go command not found (may need to restart terminal)"
    fi

    # Check golangci-lint
    if command -v golangci-lint >/dev/null 2>&1; then
        LINT_VERSION=$(golangci-lint version 2>&1 | head -n1)
        print_success "golangci-lint: $LINT_VERSION"
    else
        print_warning "golangci-lint command not found (may need to restart terminal)"
    fi

    # Check Node.js
    if command -v node >/dev/null 2>&1; then
        NODE_VERSION=$(node --version)
        print_success "Node.js: $NODE_VERSION"
    else
        print_warning "node command not found (may need to restart terminal)"
    fi
}

# Main installation flow
main() {
    echo "=========================================="
    echo "  TreeOS Development Environment Setup   "
    echo "=========================================="
    echo ""

    # Detect OS and package manager
    detect_os

    echo ""

    # Check privileges
    check_privileges

    echo ""

    # Check prerequisites
    check_prerequisites

    echo ""

    # Install asdf
    install_asdf

    echo ""

    # Configure shell
    configure_shell

    echo ""

    # Source asdf for this script
    source_asdf

    echo ""

    # Install plugins
    install_plugins

    echo ""

    # Install tools
    install_tools

    echo ""

    # Verify installation
    verify_installation

    echo ""
    echo "=========================================="
    print_success "Development environment setup complete!"
    echo ""
    echo "Next steps:"
    echo "1. Restart your terminal (or run: source ~/.bashrc)"
    echo "2. Verify installation: asdf current"
    echo "3. Build the project: make build"
    echo "4. Run linting: make lint"
    echo ""
    print_info "Tool versions will automatically switch when you cd into the repository"
    echo "=========================================="
}

# Run main function
main
