#!/bin/bash
#
# ttime.ai install script for Linux
# Usage: curl -sSL https://ttime.ai/install-linux.sh | bash
#

set -e

REPO="tokentimeai/client"
BINARY_NAME="ttime"
INSTALL_DIR="/usr/local/bin"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_info() {
    echo -e "${YELLOW}→${NC} $1"
}

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        print_error "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

if [[ "$OS" != "linux" ]]; then
    print_error "This installer is for Linux only. For macOS, use: curl -sSL https://ttime.ai/install.sh | bash"
    exit 1
fi

print_info "Installing ttime for Linux ($ARCH)..."

# Get latest release
LATEST_VERSION=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [[ -z "$LATEST_VERSION" ]]; then
    print_error "Could not determine latest version"
    exit 1
fi

print_info "Downloading ttime $LATEST_VERSION..."

# Download URL
DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_VERSION/${BINARY_NAME}_Linux_${ARCH}.tar.gz"

# Create temp directory
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

# Download and extract
curl -sSL "$DOWNLOAD_URL" -o "$TEMP_DIR/${BINARY_NAME}.tar.gz"
tar -xzf "$TEMP_DIR/${BINARY_NAME}.tar.gz" -C "$TEMP_DIR"

# Install binary
if [[ -w "$INSTALL_DIR" ]]; then
    mv "$TEMP_DIR/$BINARY_NAME" "$INSTALL_DIR/"
else
    print_info "Requesting sudo access to install to $INSTALL_DIR..."
    sudo mv "$TEMP_DIR/$BINARY_NAME" "$INSTALL_DIR/"
fi

chmod +x "$INSTALL_DIR/$BINARY_NAME"

print_success "ttime $LATEST_VERSION installed to $INSTALL_DIR/$BINARY_NAME"
echo ""
echo "Run 'ttime setup' to configure, then 'ttime install' to set up the service."