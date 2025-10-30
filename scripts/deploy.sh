#!/bin/bash

# ORLY Relay Deployment Script
# This script installs Go, builds the relay, and sets up systemd service

set -e

# Configuration
GO_VERSION="1.23.1"
GOROOT="$HOME/go"
GOPATH="$HOME"
GOBIN="$HOME/.local/bin"
GOENV_FILE="$HOME/.goenv"
BASHRC_FILE="$HOME/.bashrc"
SERVICE_NAME="orly"
BINARY_NAME="orly"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root for certain operations
check_root() {
    if [[ $EUID -eq 0 ]]; then
        return 0
    else
        return 1
    fi
}

# Check if Go is installed and get version
check_go_installation() {
    if command -v go >/dev/null 2>&1; then
        local installed_version=$(go version | grep -o 'go[0-9]\+\.[0-9]\+\.[0-9]\+' | sed 's/go//')
        local required_version=$(echo $GO_VERSION | sed 's/go//')
        
        if [[ "$installed_version" == "$required_version" ]]; then
            log_success "Go $installed_version is already installed"
            return 0
        else
            log_warning "Go $installed_version is installed, but version $required_version is required"
            return 1
        fi
    else
        log_info "Go is not installed"
        return 1
    fi
}

# Install Go
install_go() {
    log_info "Installing Go $GO_VERSION..."
    
    # Save original directory
    local original_dir=$(pwd)
    
    # Determine architecture
    local arch=$(uname -m)
    case $arch in
        x86_64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        armv7l) arch="armv6l" ;;
        *) log_error "Unsupported architecture: $arch"; exit 1 ;;
    esac
    
    local go_archive="go${GO_VERSION}.linux-${arch}.tar.gz"
    local download_url="https://golang.org/dl/${go_archive}"
    
    # Create directories
    mkdir -p "$GOBIN"
    
    # Change to home directory and download Go
    log_info "Downloading Go from $download_url..."
    cd ~
    wget -q "$download_url" || {
        log_error "Failed to download Go"
        exit 1
    }
    
    # Remove existing installation if present
    if [[ -d "$GOROOT" ]]; then
        log_info "Removing existing Go installation..."
        rm -rf "$GOROOT"
    fi
    
    # Extract Go to a temporary location first, then move to final destination
    log_info "Extracting Go..."
    tar -xf "$go_archive" -C /tmp
    mv /tmp/go "$GOROOT"

    # Clean up
    rm -f "$go_archive"
    
    # Return to original directory
    cd "$original_dir"
    
    log_success "Go $GO_VERSION installed successfully"
}

# Setup Go environment
setup_go_environment() {
    log_info "Setting up Go environment..."
    
    # Create .goenv file
    cat > "$GOENV_FILE" << EOF
# Go environment configuration
export GOROOT="$GOROOT"
export GOPATH="$GOPATH"
export GOBIN="$GOBIN"
export PATH="\$GOBIN:\$GOROOT/bin:\$PATH"
EOF
    
    # Source the environment for current session
    source "$GOENV_FILE"
    
    # Add to .bashrc if not already present
    if ! grep -q "source $GOENV_FILE" "$BASHRC_FILE" 2>/dev/null; then
        log_info "Adding Go environment to $BASHRC_FILE..."
        echo "" >> "$BASHRC_FILE"
        echo "# Go environment" >> "$BASHRC_FILE"
        echo "if [[ -f \"$GOENV_FILE\" ]]; then" >> "$BASHRC_FILE"
        echo "    source \"$GOENV_FILE\"" >> "$BASHRC_FILE"
        echo "fi" >> "$BASHRC_FILE"
        log_success "Go environment added to $BASHRC_FILE"
    else
        log_info "Go environment already configured in $BASHRC_FILE"
    fi
}

# Install build dependencies
install_dependencies() {
    log_info "Installing build dependencies..."
    
    if check_root; then
        # Install as root
        ./scripts/ubuntu_install_libsecp256k1.sh
    else
        # Request sudo for dependency installation
        log_info "Root privileges required for installing build dependencies..."
        sudo ./scripts/ubuntu_install_libsecp256k1.sh
    fi
    
    log_success "Build dependencies installed"
}

# Build the application
build_application() {
    log_info "Building ORLY relay..."
    
    # Source Go environment
    source "$GOENV_FILE"
    
    # Update embedded web assets
    log_info "Updating embedded web assets..."
    ./scripts/update-embedded-web.sh
    
    # Build the binary in the current directory
    log_info "Building binary in current directory..."
    CGO_ENABLED=1 go build -o "$BINARY_NAME"
    
    if [[ -f "./$BINARY_NAME" ]]; then
        log_success "ORLY relay built successfully"
    else
        log_error "Failed to build ORLY relay"
        exit 1
    fi
}

# Set capabilities for port 443 binding
set_capabilities() {
    log_info "Setting capabilities for port 443 binding..."
    
    if check_root; then
        setcap 'cap_net_bind_service=+ep' "./$BINARY_NAME"
    else
        sudo setcap 'cap_net_bind_service=+ep' "./$BINARY_NAME"
    fi
    
    log_success "Capabilities set for port 443 binding"
}

# Install binary
install_binary() {
    log_info "Installing binary to $GOBIN..."
    
    # Ensure GOBIN directory exists
    mkdir -p "$GOBIN"
    
    # Copy binary
    cp "./$BINARY_NAME" "$GOBIN/"
    chmod +x "$GOBIN/$BINARY_NAME"
    
    log_success "Binary installed to $GOBIN/$BINARY_NAME"
}

# Create systemd service
create_systemd_service() {
    log_info "Creating systemd service..."
    
    local service_file="/etc/systemd/system/${SERVICE_NAME}.service"
    local working_dir=$(pwd)
    
    # Create service file content
    local service_content="[Unit]
Description=ORLY Nostr Relay
After=network.target
Wants=network.target

[Service]
Type=simple
User=$USER
Group=$USER
WorkingDirectory=$working_dir
ExecStart=$GOBIN/$BINARY_NAME
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=$SERVICE_NAME

# Security settings
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$working_dir $HOME/.local/share/ORLY $HOME/.cache/ORLY
PrivateTmp=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true

# Network settings
AmbientCapabilities=CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target"

    # Write service file
    if check_root; then
        echo "$service_content" > "$service_file"
    else
        echo "$service_content" | sudo tee "$service_file" > /dev/null
    fi
    
    # Reload systemd and enable service
    if check_root; then
        systemctl daemon-reload
        systemctl enable "$SERVICE_NAME"
    else
        sudo systemctl daemon-reload
        sudo systemctl enable "$SERVICE_NAME"
    fi
    
    log_success "Systemd service created and enabled"
}

# Main deployment function
main() {
    log_info "Starting ORLY relay deployment..."
    
    # Check if we're in the right directory
    if [[ ! -f "go.mod" ]] || ! grep -q "next.orly.dev" go.mod; then
        log_error "This script must be run from the next.orly.dev project root directory"
        exit 1
    fi
    
    # Check and install Go if needed
    if ! check_go_installation; then
        install_go
        setup_go_environment
    fi
    
    # Install dependencies
    install_dependencies
    
    # Build application
    build_application
    
    # Set capabilities
    set_capabilities
    
    # Install binary
    install_binary
    
    # Create systemd service
    create_systemd_service
    
    log_success "ORLY relay deployment completed successfully!"
    echo ""
    log_info "Next steps:"
    echo "  1. Reload your terminal environment: source ~/.bashrc"
    echo "  2. Configure your relay by setting environment variables"
    echo "  3. Start the service: sudo systemctl start $SERVICE_NAME"
    echo "  4. Check service status: sudo systemctl status $SERVICE_NAME"
    echo "  5. View logs: sudo journalctl -u $SERVICE_NAME -f"
    echo ""
    log_info "Service management commands:"
    echo "  Start:   sudo systemctl start $SERVICE_NAME"
    echo "  Stop:    sudo systemctl stop $SERVICE_NAME"
    echo "  Restart: sudo systemctl restart $SERVICE_NAME"
    echo "  Enable:  sudo systemctl enable $SERVICE_NAME --now"
    echo "  Disable: sudo systemctl disable $SERVICE_NAME --now"
    echo "  Status:  sudo systemctl status $SERVICE_NAME"
    echo "  Logs:    sudo journalctl -u $SERVICE_NAME -f"
}

# Handle command line arguments
case "${1:-}" in
    --help|-h)
        echo "ORLY Relay Deployment Script"
        echo ""
        echo "Usage: $0 [options]"
        echo ""
        echo "Options:"
        echo "  --help, -h    Show this help message"
        echo ""
        echo "This script will:"
        echo "  1. Install Go $GO_VERSION if not present"
        echo "  2. Set up Go environment in ~/.goenv"
        echo "  3. Install build dependencies (requires sudo)"
        echo "  4. Build the ORLY relay"
        echo "  5. Set capabilities for port 443 binding"
        echo "  6. Install the binary to ~/.local/bin"
        echo "  7. Create and enable systemd service"
        exit 0
        ;;
    *)
        main "$@"
        ;;
esac
