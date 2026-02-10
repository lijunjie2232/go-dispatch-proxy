#!/bin/bash

# Go Dispatch Proxy Installation Script
# This script installs the go-dispatch-proxy as a system service

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SERVICE_NAME="go-dispatch-proxy"
BINARY_NAME="go-dispatch-proxy"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc"
SYSTEMD_DIR="/etc/systemd/system"
SERVICE_FILE="go-dispatch-proxy.service"
CONFIG_FILE="go-dispatch-proxy.yaml"

# Function to print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if running as root
check_root() {
    if [[ $EUID -ne 0 ]]; then
        print_error "This script must be run as root. Please use sudo."
        exit 1
    fi
}

# Detect OS and package manager
detect_os() {
    if [[ -f /etc/os-release ]]; then
        . /etc/os-release
        OS=$NAME
        VERSION=$VERSION_ID
    elif [[ -f /etc/redhat-release ]]; then
        OS=$(cat /etc/redhat-release | cut -d ' ' -f 1)
        VERSION=$(cat /etc/redhat-release | grep -oE '[0-9]+\.[0-9]+' | head -1)
    else
        OS=$(uname -s)
        VERSION=$(uname -r)
    fi
    
    print_status "Detected OS: $OS $VERSION"
    
    # Detect package manager
    if command -v apt-get &> /dev/null; then
        PKG_MANAGER="apt"
    elif command -v yum &> /dev/null; then
        PKG_MANAGER="yum"
    elif command -v dnf &> /dev/null; then
        PKG_MANAGER="dnf"
    else
        PKG_MANAGER="unknown"
    fi
    
    print_status "Package manager: $PKG_MANAGER"
}

# Install required dependencies
install_dependencies() {
    print_status "Installing required dependencies..."
    
    case $PKG_MANAGER in
        apt)
            apt update
            apt install -y systemd
            ;;
        yum|dnf)
            if [[ $PKG_MANAGER == "yum" ]]; then
                yum install -y systemd
            else
                dnf install -y systemd
            fi
            ;;
        *)
            print_warning "Unknown package manager. Please ensure systemd is installed."
            ;;
    esac
}

# Create system user and group
create_user_group() {
    print_status "Creating system user and group..."
    
    # Create proxy group if it doesn't exist
    if ! getent group proxy >/dev/null; then
        groupadd --system proxy
        print_success "Created group 'proxy'"
    else
        print_status "Group 'proxy' already exists"
    fi
    
    # Create proxy user if it doesn't exist
    if ! getent passwd proxy >/dev/null; then
        useradd --system --no-create-home --shell /usr/sbin/nologin \
                --gid proxy --home-dir /nonexistent proxy
        print_success "Created user 'proxy'"
    else
        print_status "User 'proxy' already exists"
    fi
}

# Install binary
install_binary() {
    print_status "Installing binary..."
    
    # Check if binary exists in current directory
    if [[ ! -f "./$BINARY_NAME" ]]; then
        print_error "Binary '$BINARY_NAME' not found in current directory"
        print_error "Please build the binary first using 'go build' or download it"
        exit 1
    fi
    
    # Copy binary to installation directory
    cp "./$BINARY_NAME" "$INSTALL_DIR/$BINARY_NAME"
    chmod 755 "$INSTALL_DIR/$BINARY_NAME"
    chown root:root "$INSTALL_DIR/$BINARY_NAME"
    
    print_success "Installed binary to $INSTALL_DIR/$BINARY_NAME"
}

# Install configuration file
install_config() {
    print_status "Installing configuration file..."
    
    # Check if config file exists
    if [[ -f "./$CONFIG_FILE" ]]; then
        # Copy user's config file
        cp "./$CONFIG_FILE" "$CONFIG_DIR/$CONFIG_FILE"
        print_success "Copied existing configuration to $CONFIG_DIR/$CONFIG_FILE"
    elif [[ -f "./${CONFIG_FILE}.example" ]]; then
        # Copy example config file
        cp "./${CONFIG_FILE}.example" "$CONFIG_DIR/$CONFIG_FILE"
        print_warning "Copied example configuration to $CONFIG_DIR/$CONFIG_FILE"
        print_warning "Please edit $CONFIG_DIR/$CONFIG_FILE to configure your load balancers"
    else
        # Create basic config file
        cat > "$CONFIG_DIR/$CONFIG_FILE" << EOF
# Go Dispatch Proxy Configuration
listen_host: "127.0.0.1"
listen_port: 8080
tunnel_mode: false
quiet_mode: false
use_devices: false

# Configure your load balancers below
load_balancers:
  # Example configuration - replace with your actual network interfaces
  # - device: "eth0"
  #   cont_ratio: 1
  # - device: "wlan0"  
  #   cont_ratio: 1
EOF
        print_warning "Created basic configuration file at $CONFIG_DIR/$CONFIG_FILE"
        print_warning "Please edit this file to configure your load balancers"
    fi
    
    # Set proper permissions
    chmod 644 "$CONFIG_DIR/$CONFIG_FILE"
    chown root:proxy "$CONFIG_DIR/$CONFIG_FILE"
}

# Install systemd service
install_service() {
    print_status "Installing systemd service..."
    
    # Check if systemd service file exists
    if [[ ! -f "./systemd/$SERVICE_FILE" ]]; then
        print_error "Service file './systemd/$SERVICE_FILE' not found"
        exit 1
    fi
    
    # Copy service file
    cp "./systemd/$SERVICE_FILE" "$SYSTEMD_DIR/$SERVICE_FILE"
    chmod 644 "$SYSTEMD_DIR/$SERVICE_FILE"
    
    # Reload systemd daemon
    systemctl daemon-reload
    
    print_success "Installed systemd service"
}

# Set up capabilities (for Linux interface binding)
setup_capabilities() {
    if [[ "$OS" == *"Linux"* ]] || [[ "$OS" == *"Ubuntu"* ]] || [[ "$OS" == *"Debian"* ]] || [[ "$OS" == *"CentOS"* ]] || [[ "$OS" == *"Red Hat"* ]]; then
        print_status "Setting up network capabilities..."
        
        # Check if setcap is available
        if command -v setcap &> /dev/null; then
            # Grant CAP_NET_RAW capability to bind to network interfaces
            setcap cap_net_raw=eip "$INSTALL_DIR/$BINARY_NAME" 2>/dev/null || {
                print_warning "Failed to set network capabilities. You may need to run as root or use sudo."
                print_warning "Alternatively, you can enable tunnel mode which doesn't require special capabilities."
            }
            print_success "Set network capabilities for $BINARY_NAME"
        else
            print_warning "setcap not found. Install libcap2-bin package for network interface binding support."
        fi
    fi
}

# Enable and start service
enable_service() {
    print_status "Enabling and starting service..."
    
    # Enable service to start on boot
    systemctl enable "$SERVICE_NAME"
    
    # Start the service
    if systemctl start "$SERVICE_NAME"; then
        print_success "Service started successfully"
    else
        print_error "Failed to start service"
        print_error "Check configuration file and service status:"
        echo "  sudo systemctl status $SERVICE_NAME"
        echo "  sudo journalctl -u $SERVICE_NAME -f"
        exit 1
    fi
}

# Show post-installation information
show_post_install() {
    echo
    echo "=========================================="
    echo "  Go Dispatch Proxy Installation Complete"
    echo "=========================================="
    echo
    print_success "Installation completed successfully!"
    echo
    echo "Next steps:"
    echo "1. Edit the configuration file: $CONFIG_DIR/$CONFIG_FILE"
    echo "2. Configure your load balancers in the config file"
    echo "3. Restart the service: sudo systemctl restart $SERVICE_NAME"
    echo
    echo "Service management commands:"
    echo "  Start service:    sudo systemctl start $SERVICE_NAME"
    echo "  Stop service:     sudo systemctl stop $SERVICE_NAME" 
    echo "  Restart service:  sudo systemctl restart $SERVICE_NAME"
    echo "  Check status:     sudo systemctl status $SERVICE_NAME"
    echo "  View logs:        sudo journalctl -u $SERVICE_NAME -f"
    echo
    echo "Configuration file location: $CONFIG_DIR/$CONFIG_FILE"
    echo "Binary location: $INSTALL_DIR/$BINARY_NAME"
    echo
}

# Main installation function
main() {
    print_status "Starting Go Dispatch Proxy installation..."
    
    check_root
    detect_os
    install_dependencies
    create_user_group
    install_binary
    install_config
    install_service
    setup_capabilities
    enable_service
    show_post_install
}

# Run main function
main "$@"