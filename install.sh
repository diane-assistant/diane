#!/bin/sh
# DIANE MCP Server Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/Emergent-Comapny/diane/main/install.sh | sh
#
# Environment variables:
#   DIANE_VERSION  - Specific version to install (default: latest)
#   DIANE_DIR      - Installation directory (default: ~/.diane)

set -e

# Configuration
GITHUB_REPO="Emergent-Comapny/diane"
BINARY_NAME="diane"
DEFAULT_INSTALL_DIR="${HOME}/.diane"

# Colors (if terminal supports them)
if [ -t 1 ]; then
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[0;33m'
    BLUE='\033[0;34m'
    MUTED='\033[0;2m'
    NC='\033[0m' # No Color
else
    RED=''
    GREEN=''
    YELLOW=''
    BLUE=''
    MUTED=''
    NC=''
fi

info() {
    printf "${BLUE}==>${NC} %s\n" "$1"
}

success() {
    printf "${GREEN}==>${NC} %s\n" "$1"
}

warn() {
    printf "${YELLOW}Warning:${NC} %s\n" "$1"
}

error() {
    printf "${RED}Error:${NC} %s\n" "$1" >&2
    exit 1
}

# Detect OS and architecture
detect_platform() {
    OS="$(uname -s)"
    ARCH="$(uname -m)"

    case "$OS" in
        Darwin)
            OS="darwin"
            ;;
        Linux)
            OS="linux"
            ;;
        *)
            error "Unsupported operating system: $OS"
            ;;
    esac

    case "$ARCH" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            ARCH="arm64"
            ;;
        *)
            error "Unsupported architecture: $ARCH"
            ;;
    esac

    PLATFORM="${OS}-${ARCH}"
    info "Detected platform: ${PLATFORM}"
}

# Get latest version from GitHub releases
get_latest_version() {
    # Note: info messages go to stderr to avoid polluting the version output
    printf "${BLUE}==>${NC} Fetching latest version...\n" >&2
    LATEST=$(curl -fsSL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
    
    if [ -z "$LATEST" ]; then
        error "Failed to get latest version. Check your internet connection."
    fi
    
    echo "$LATEST"
}

# Get installed version by querying the MCP server
get_installed_version() {
    INSTALL_DIR="${DIANE_DIR:-$DEFAULT_INSTALL_DIR}"
    BINARY_PATH="${INSTALL_DIR}/bin/diane-mcp"
    
    if [ ! -x "$BINARY_PATH" ]; then
        echo ""
        return
    fi
    
    # Send MCP initialize request and extract version from response
    # The server responds with serverInfo.version in the initialize response
    INSTALLED=$(echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"installer","version":"1.0.0"}}}' | \
        timeout 2 "$BINARY_PATH" 2>/dev/null | \
        grep -o '"version":"[^"]*"' | head -1 | sed 's/"version":"//;s/"//')
    
    if [ -n "$INSTALLED" ]; then
        echo "v${INSTALLED}"
    else
        echo ""
    fi
}

# Check if upgrade is needed
check_version() {
    INSTALL_DIR="${DIANE_DIR:-$DEFAULT_INSTALL_DIR}"
    BINARY_PATH="${INSTALL_DIR}/bin/diane-mcp"
    
    if [ ! -x "$BINARY_PATH" ]; then
        return 0  # Not installed, proceed with installation
    fi
    
    INSTALLED_VERSION=$(get_installed_version)
    
    if [ -z "$INSTALLED_VERSION" ]; then
        info "DIANE is installed but version could not be determined. Reinstalling..."
        return 0
    fi
    
    # If specific version requested, compare with that
    if [ -n "$DIANE_VERSION" ]; then
        TARGET_VERSION="$DIANE_VERSION"
    else
        TARGET_VERSION="$VERSION"
    fi
    
    if [ "$INSTALLED_VERSION" = "$TARGET_VERSION" ]; then
        success "DIANE ${INSTALLED_VERSION} is already installed and up to date"
        exit 0
    fi
    
    info "${MUTED}Installed version:${NC} ${INSTALLED_VERSION}"
    info "${MUTED}Available version:${NC} ${TARGET_VERSION}"
    info "Upgrading..."
    return 0
}

# Download and install
install() {
    INSTALL_DIR="${DIANE_DIR:-$DEFAULT_INSTALL_DIR}"
    VERSION="${DIANE_VERSION:-$(get_latest_version)}"
    
    # Check if already up to date
    check_version
    
    info "Installing DIANE MCP Server ${VERSION}..."
    
    # Create installation directory
    mkdir -p "${INSTALL_DIR}/bin"
    mkdir -p "${INSTALL_DIR}/secrets"
    mkdir -p "${INSTALL_DIR}/tools"
    mkdir -p "${INSTALL_DIR}/data"
    mkdir -p "${INSTALL_DIR}/logs"
    
    # Construct download URL
    DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/${BINARY_NAME}-${PLATFORM}.tar.gz"
    
    info "Downloading from: ${DOWNLOAD_URL}"
    
    # Download and extract
    TMP_DIR=$(mktemp -d)
    trap "rm -rf ${TMP_DIR}" EXIT
    
    curl -fsSL "${DOWNLOAD_URL}" -o "${TMP_DIR}/diane.tar.gz" || error "Download failed. Check if version ${VERSION} exists."
    
    # Verify checksum if available
    CHECKSUM_URL="${DOWNLOAD_URL}.sha256"
    if curl -fsSL "${CHECKSUM_URL}" -o "${TMP_DIR}/diane.tar.gz.sha256" 2>/dev/null; then
        info "Verifying checksum..."
        cd "${TMP_DIR}"
        if command -v sha256sum >/dev/null 2>&1; then
            sha256sum -c diane.tar.gz.sha256 || error "Checksum verification failed!"
        elif command -v shasum >/dev/null 2>&1; then
            shasum -a 256 -c diane.tar.gz.sha256 || error "Checksum verification failed!"
        else
            warn "No sha256sum or shasum available, skipping checksum verification"
        fi
        cd - >/dev/null
    fi
    
    # Extract
    tar -xzf "${TMP_DIR}/diane.tar.gz" -C "${TMP_DIR}"
    
    # Install binary (tarball contains diane-mcp directly)
    mv "${TMP_DIR}/diane-mcp" "${INSTALL_DIR}/bin/diane-mcp"
    chmod +x "${INSTALL_DIR}/bin/diane-mcp"
    
    success "DIANE MCP Server installed to ${INSTALL_DIR}/bin/diane-mcp"
    
    # Check if bin is in PATH
    case ":${PATH}:" in
        *":${INSTALL_DIR}/bin:"*)
            ;;
        *)
            echo ""
            warn "Add ${INSTALL_DIR}/bin to your PATH:"
            echo ""
            echo "  # Add to ~/.bashrc or ~/.zshrc:"
            echo "  export PATH=\"\${HOME}/.diane/bin:\${PATH}\""
            echo ""
            ;;
    esac
    
    # Print version
    echo ""
    success "Installation complete!"
    echo ""
    echo "  Version: ${VERSION}"
    echo "  Binary:  ${INSTALL_DIR}/bin/diane-mcp"
    echo ""
    echo "Directory structure:"
    echo "  ${INSTALL_DIR}/"
    echo "  ├── bin/          # diane-mcp binary"
    echo "  ├── secrets/      # API keys and config files"
    echo "  ├── tools/        # Helper scripts (actualbudget-cli.mjs, etc.)"
    echo "  ├── data/         # Persistent data"
    echo "  └── logs/         # Log files"
    echo ""
    echo "Next steps:"
    echo "  1. Configure your MCP client to use: ${INSTALL_DIR}/bin/diane-mcp"
    echo "  2. Copy secrets to: ${INSTALL_DIR}/secrets/"
    echo "  3. Run: diane-mcp --help"
    echo ""
}

# Upgrade (alias for install with version check)
upgrade() {
    install
}

# Uninstall
uninstall() {
    INSTALL_DIR="${DIANE_DIR:-$DEFAULT_INSTALL_DIR}"
    
    if [ ! -d "${INSTALL_DIR}" ]; then
        error "DIANE is not installed at ${INSTALL_DIR}"
    fi
    
    info "Uninstalling DIANE from ${INSTALL_DIR}..."
    
    rm -f "${INSTALL_DIR}/bin/diane-mcp"
    
    success "DIANE MCP Server uninstalled"
    warn "Data directory preserved at ${INSTALL_DIR}/data"
    echo "  To remove completely: rm -rf ${INSTALL_DIR}"
}

# Show version
version() {
    INSTALL_DIR="${DIANE_DIR:-$DEFAULT_INSTALL_DIR}"
    INSTALLED_VERSION=$(get_installed_version)
    
    if [ -n "$INSTALLED_VERSION" ]; then
        echo "DIANE MCP Server ${INSTALLED_VERSION}"
        echo "Installed at: ${INSTALL_DIR}/bin/diane-mcp"
    else
        echo "DIANE MCP Server is not installed"
        echo "Run: curl -fsSL https://raw.githubusercontent.com/Emergent-Comapny/diane/main/install.sh | sh"
    fi
}

# Main
main() {
    case "${1:-install}" in
        install)
            detect_platform
            install
            ;;
        upgrade)
            detect_platform
            upgrade
            ;;
        uninstall)
            uninstall
            ;;
        version)
            version
            ;;
        --version|-v)
            echo "DIANE Installer v1.1.0"
            ;;
        --help|-h)
            echo "DIANE MCP Server Installer"
            echo ""
            echo "Usage: $0 [command]"
            echo ""
            echo "Commands:"
            echo "  install     Install or upgrade DIANE MCP Server (default)"
            echo "  upgrade     Upgrade to latest version (same as install)"
            echo "  uninstall   Remove DIANE MCP Server"
            echo "  version     Show installed version"
            echo ""
            echo "Environment variables:"
            echo "  DIANE_VERSION  Specific version to install (default: latest)"
            echo "  DIANE_DIR      Installation directory (default: ~/.diane)"
            echo ""
            echo "Examples:"
            echo "  curl -fsSL https://raw.githubusercontent.com/Emergent-Comapny/diane/main/install.sh | sh"
            echo "  DIANE_VERSION=v1.0.0 ./install.sh"
            echo "  ./install.sh upgrade"
            echo "  ./install.sh uninstall"
            ;;
        *)
            error "Unknown command: $1. Use --help for usage."
            ;;
    esac
}

main "$@"
