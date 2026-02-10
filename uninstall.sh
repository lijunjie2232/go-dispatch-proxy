#!/bin/bash

# Go Dispatch Proxy Uninstallation Script
# This script completely removes the go-dispatch-proxy service and all related files

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

# Stop and disable service
stop_service() {
    print_status "Stopping and disabling service..."
    
    # Check if service exists
    if systemctl list-unit-files | grep -q "^$SERVICE_NAME.service"; then
        # Stop service if running
        if systemctl is-active --quiet "$SERVICE_NAME"; then
            systemctl stop "$SERVICE_NAME"
            print_success "Stopped service $SERVICE_NAME"
        fi
        
        # Disable service
        systemctl disable "$SERVICE_NAME" 2>/dev/null || true
        print_success "Disabled service $SERVICE_NAME"
    else
        print_warning "Service $SERVICE_NAME not found"
    fi
}

# Remove systemd service file
remove_service_file() {
    print_status "Removing systemd service file..."
    
    SERVICE_PATH="$SYSTEMD_DIR/$SERVICE_FILE"
    
    if [[ -f "$SERVICE_PATH" ]]; then
        rm -f "$SERVICE_PATH"
        print_success "Removed service file $SERVICE_PATH"
    else
        print_warning "Service file $SERVICE_PATH not found"
    fi
    
    # Reload systemd daemon
    systemctl daemon-reload 2>/dev/null || true
}

# Remove binary
remove_binary() {
    print_status "Removing binary..."
    
    BINARY_PATH="$INSTALL_DIR/$BINARY_NAME"
    
    if [[ -f "$BINARY_PATH" ]]; then
        rm -f "$BINARY_PATH"
        print_success "Removed binary $BINARY_PATH"
    else
        print_warning "Binary $BINARY_PATH not found"
    fi
}

# Remove configuration file
remove_config() {
    print_status "Removing configuration file..."
    
    CONFIG_PATH="$CONFIG_DIR/$CONFIG_FILE"
    
    if [[ -f "$CONFIG_PATH" ]]; then
        # Ask user if they want to keep the config file
        echo
        read -p "Do you want to keep the configuration file ($CONFIG_PATH) for backup? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            print_status "Keeping configuration file for backup"
            print_warning "Configuration file preserved at $CONFIG_PATH"
        else
            rm -f "$CONFIG_PATH"
            print_success "Removed configuration file $CONFIG_PATH"
        fi
    else
        print_warning "Configuration file $CONFIG_PATH not found"
    fi
}

# Remove capabilities from binary (if they exist)
remove_capabilities() {
    if command -v setcap &> /dev/null; then
        BINARY_PATH="$INSTALL_DIR/$BINARY_NAME"
        if [[ -f "$BINARY_PATH" ]]; then
            # Remove capabilities
            setcap -r "$BINARY_PATH" 2>/dev/null || true
            print_status "Removed capabilities from binary"
        fi
    fi
}

# Remove user and group (optional)
remove_user_group() {
    print_status "Checking system user and group..."
    
    # Only remove if no other services are using them
    # For safety, we won't automatically remove the user/group
    # Users can remove them manually if needed
    
    if getent passwd proxy >/dev/null; then
        print_warning "User 'proxy' still exists. Remove manually if no longer needed:"
        echo "  sudo userdel proxy"
    fi
    
    if getent group proxy >/dev/null; then
        print_warning "Group 'proxy' still exists. Remove manually if no longer needed:"
        echo "  sudo groupdel proxy"
    fi
}

# Clean up any remaining files
cleanup_remaining() {
    print_status "Cleaning up remaining files..."
    
    # Remove any log files or temporary files
    # Note: go-dispatch-proxy doesn't typically create persistent files outside of config
    print_status "No additional cleanup needed"
}

# Show post-uninstallation information
show_post_uninstall() {
    echo
    echo "============================================"
    echo "  Go Dispatch Proxy Uninstallation Complete"
    echo "============================================"
    echo
    print_success "Uninstallation completed successfully!"
    echo
    echo "What was removed:"
    echo "  • Systemd service: $SERVICE_NAME"
    echo "  • Binary: $INSTALL_DIR/$BINARY_NAME"
    echo "  • Service file: $SYSTEMD_DIR/$SERVICE_FILE"
    echo
    echo "What you might want to clean up manually:"
    echo "  • Configuration file: $CONFIG_DIR/$CONFIG_FILE (kept for backup)"
    echo "  • User 'proxy': sudo userdel proxy"
    echo "  • Group 'proxy': sudo groupdel proxy"
    echo
    echo "To completely remove all traces:"
    echo "  sudo rm -f $CONFIG_DIR/$CONFIG_FILE"
    echo "  sudo userdel proxy"
    echo "  sudo groupdel proxy"
    echo
}

# Confirmation prompt
confirm_uninstall() {
    echo
    echo "=================================="
    echo "  GO DISPATCH PROXY UNINSTALLATION"
    echo "=================================="
    echo
    print_warning "This will completely remove go-dispatch-proxy from your system."
    echo
    echo "The following will be removed:"
    echo "  • Systemd service and configuration"
    echo "  • Binary file ($INSTALL_DIR/$BINARY_NAME)"
    echo "  • Service definition file"
    echo
    read -p "Are you sure you want to continue? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_status "Uninstallation cancelled."
        exit 0
    fi
}

# Main uninstallation function
main() {
    confirm_uninstall
    print_status "Starting Go Dispatch Proxy uninstallation..."
    
    check_root
    stop_service
    remove_service_file
    remove_binary
    remove_config
    remove_capabilities
    remove_user_group
    cleanup_remaining
    show_post_uninstall
}

# Run main function
main "$@"