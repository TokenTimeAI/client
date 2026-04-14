#!/bin/bash
#
# ttime.ai install script for macOS and Linux
# Usage: curl -sSL https://ttime.ai/install.sh | bash
#

set -e

REPO="tokentimeai/client"
BINARY_NAME="ttime"
INSTALL_DIR="/usr/local/bin"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_success() { echo -e "${GREEN}✓${NC} $1"; }
print_error() { echo -e "${RED}✗${NC} $1"; }
print_info() { echo -e "${YELLOW}→${NC} $1"; }
print_step() { echo -e "${BLUE}➜${NC} $1"; }

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) print_error "Unsupported architecture: $ARCH"; exit 1 ;;
esac

print_step "Installing ttime for $(uname -s) ($ARCH)..."

# macOS: prefer Homebrew
if [[ "$OS" == "darwin" ]]; then
    if command -v brew &> /dev/null; then
        print_info "Homebrew detected, installing via tap..."
        
        if ! brew tap | grep -q "^tokentimeai/tap$"; then
            brew tap tokentimeai/tap https://github.com/tokentimeai/homebrew-tap
        fi
        
        brew install tokentimeai/tap/ttime
        print_success "ttime installed via Homebrew!"
        echo ""
        echo "Run 'ttime setup' to configure, then 'ttime install' to set up the service."
        exit 0
    fi
    
    print_info "Homebrew not found, installing binary directly..."
    PLATFORM="Darwin"
    EXT="zip"
else
    PLATFORM="Linux"
    EXT="tar.gz"
fi

# Get latest release
print_info "Fetching latest version..."
LATEST_VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [[ -z "$LATEST_VERSION" ]]; then
    print_error "Could not determine latest version"
    exit 1
fi

print_info "Downloading ttime $LATEST_VERSION..."

DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_VERSION/${BINARY_NAME}_${PLATFORM}_${ARCH}.${EXT}"
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

curl -fsSL "$DOWNLOAD_URL" -o "$TEMP_DIR/${BINARY_NAME}.${EXT}"

if [[ "$EXT" == "zip" ]]; then
    unzip -q "$TEMP_DIR/${BINARY_NAME}.zip" -d "$TEMP_DIR"
else
    tar -xzf "$TEMP_DIR/${BINARY_NAME}.tar.gz" -C "$TEMP_DIR"
fi

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