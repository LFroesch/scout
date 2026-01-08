#!/usr/bin/env bash
set -euo pipefail

REPO="LFroesch/scout"
BINARY_NAME="scout"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

error() {
    echo -e "${RED}Error: $1${NC}" >&2
    exit 1
}

info() {
    echo -e "${GREEN}$1${NC}"
}

warn() {
    echo -e "${YELLOW}$1${NC}"
}

# Detect OS and architecture
detect_platform() {
    local os arch

    case "$(uname -s)" in
        Linux*)     os="linux" ;;
        Darwin*)    os="darwin" ;;
        MINGW*|MSYS*|CYGWIN*) os="windows" ;;
        *)          error "Unsupported OS: $(uname -s)" ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64)   arch="amd64" ;;
        aarch64|arm64)  arch="arm64" ;;
        *)              error "Unsupported architecture: $(uname -m)" ;;
    esac

    echo "${os}-${arch}"
}

# Get latest release tag from GitHub
get_latest_version() {
    curl -sL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name":' \
        | sed -E 's/.*"([^"]+)".*/\1/' \
        || error "Failed to fetch latest version"
}

# Main installation
main() {
    info "Installing Scout..."

    # Detect platform
    PLATFORM=$(detect_platform)
    info "Detected platform: $PLATFORM"

    # Get latest version
    VERSION=$(get_latest_version)
    info "Latest version: $VERSION"

    # Determine binary name based on OS
    if [[ "$PLATFORM" == windows* ]]; then
        BINARY_FILE="${BINARY_NAME}-${PLATFORM}.exe"
    else
        BINARY_FILE="${BINARY_NAME}-${PLATFORM}"
    fi

    # Download binary
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_FILE}"
    info "Downloading from: $DOWNLOAD_URL"

    TEMP_FILE=$(mktemp)
    if ! curl -sL "$DOWNLOAD_URL" -o "$TEMP_FILE"; then
        rm -f "$TEMP_FILE"
        error "Failed to download binary"
    fi

    # Determine install location
    if [[ -w "/usr/local/bin" ]]; then
        INSTALL_DIR="/usr/local/bin"
    elif [[ -d "$HOME/.local/bin" ]]; then
        INSTALL_DIR="$HOME/.local/bin"
    else
        mkdir -p "$HOME/.local/bin"
        INSTALL_DIR="$HOME/.local/bin"
        warn "Created $HOME/.local/bin - add it to your PATH if not already there"
    fi

    # Install binary
    INSTALL_PATH="$INSTALL_DIR/$BINARY_NAME"
    if [[ -w "$INSTALL_DIR" ]]; then
        mv "$TEMP_FILE" "$INSTALL_PATH"
    else
        sudo mv "$TEMP_FILE" "$INSTALL_PATH" || error "Failed to move binary (try running with sudo)"
    fi

    chmod +x "$INSTALL_PATH"

    info "✓ Scout installed successfully to $INSTALL_PATH"
    info "Run 'scout' to get started!"

    # Check if install dir is in PATH
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        warn "Note: $INSTALL_DIR is not in your PATH"
        warn "Add this line to your shell config (~/.bashrc, ~/.zshrc, etc.):"
        echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
    fi
}

main
