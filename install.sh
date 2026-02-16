#!/bin/sh
# DIANE Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/diane-assistant/diane/main/install.sh | sh
#
# Environment variables:
#   DIANE_VERSION  - Specific version to install (default: latest)
#   DIANE_DIR      - Installation directory (default: ~/.diane)

set -e

# Configuration
GITHUB_REPO="diane-assistant/diane"
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
    # info "Detected platform: ${PLATFORM}"
}

# Check for Arch Linux / Pacman
is_arch_linux() {
    if [ -f "/etc/arch-release" ] || [ -f "/etc/manjaro-release" ]; then
        return 0
    fi
    if command -v pacman >/dev/null 2>&1; then
        return 0
    fi
    return 1
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

# Install using Pacman (Arch Linux)
install_arch() {
    VERSION="${DIANE_VERSION:-$(get_latest_version)}"
    # Strip 'v' prefix if present for PKGBUILD versioning compatibility
    CLEAN_VERSION="${VERSION#v}"
    
    info "Arch Linux detected. Preparing to install Diane ${VERSION}..."

    # Check dependencies
    if ! command -v makepkg >/dev/null 2>&1; then
        error "makepkg not found. Please install 'base-devel' group: sudo pacman -S base-devel"
    fi

    TMP_DIR=$(mktemp -d)
    trap "rm -rf ${TMP_DIR}" EXIT
    
    info "Downloading package resources..."
    
    # Download helper files from the repo
    BASE_URL="https://raw.githubusercontent.com/${GITHUB_REPO}/main/pkg/arch"
    
    # We download service and config to include in the package
    curl -fsSL "${BASE_URL}/diane.service" -o "${TMP_DIR}/diane.service" || error "Failed to download diane.service"
    curl -fsSL "${BASE_URL}/diane.config.json" -o "${TMP_DIR}/diane.config.json" || error "Failed to download diane.config.json"
    
    cd "${TMP_DIR}"

    # Generate a PKGBUILD that uses the remote binaries
    cat > PKGBUILD <<EOF
# Maintainer: Diane <diane@diane-assistant.com>
pkgname=diane
pkgver=${CLEAN_VERSION}
pkgrel=1
pkgdesc="Diane AI assistant - MCP server and control utility"
arch=('x86_64' 'aarch64')
url="https://github.com/${GITHUB_REPO}"
license=('MIT')
depends=('glibc')
makedepends=()
backup=('etc/diane/config.json')
source_x86_64=("https://github.com/${GITHUB_REPO}/releases/download/v\${pkgver}/diane-linux-amd64.tar.gz")
source_aarch64=("https://github.com/${GITHUB_REPO}/releases/download/v\${pkgver}/diane-linux-arm64.tar.gz")
sha256sums_x86_64=('SKIP')
sha256sums_aarch64=('SKIP')

package() {
    # Binaries are in the root of the tarball
    install -Dm755 "\${srcdir}/diane" "\${pkgdir}/usr/bin/diane"
    
    if [ -f "\${srcdir}/acp-server" ]; then
        install -Dm755 "\${srcdir}/acp-server" "\${pkgdir}/usr/bin/acp-server"
    fi

    # Service file
    install -Dm644 "${TMP_DIR}/diane.service" "\${pkgdir}/usr/lib/systemd/system/diane.service"
    
    # Config file
    install -Dm644 "${TMP_DIR}/diane.config.json" "\${pkgdir}/etc/diane/config.json"
    
    # Data directory
    install -dm755 "\${pkgdir}/var/lib/diane"
}
EOF

    info "Building package..."
    # -s: Install missing dependencies
    # --noconfirm: Non-interactive
    makepkg -s --noconfirm
    
    PKG_FILE=$(ls diane-*.pkg.tar.zst | head -1)
    if [ -z "$PKG_FILE" ]; then
        error "Package creation failed"
    fi
    
    info "Installing ${PKG_FILE}..."
    if command -v sudo >/dev/null 2>&1; then
        sudo pacman -U --noconfirm "${PKG_FILE}"
    else
        su -c "pacman -U --noconfirm ${PKG_FILE}"
    fi
    
    success "Diane installed successfully via Pacman!"
    
    # Post-install info
    echo ""
    info "Service installed at /usr/lib/systemd/system/diane.service"
    info "Config installed at /etc/diane/config.json"
    echo ""
    info "To enable and start the service:"
    echo "  sudo systemctl enable --now diane"
    echo ""
}

# Generic Download and install (macOS, non-Arch Linux)
install_generic() {
    INSTALL_DIR="${DIANE_DIR:-$DEFAULT_INSTALL_DIR}"
    VERSION="${DIANE_VERSION:-$(get_latest_version)}"
    
    detect_platform
    
    info "Installing DIANE ${VERSION} to ${INSTALL_DIR}..."
    
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
    
    # Extract
    tar -xzf "${TMP_DIR}/diane.tar.gz" -C "${TMP_DIR}"
    
    # Install binary
    if [ -f "${TMP_DIR}/diane" ]; then
        mv "${TMP_DIR}/diane" "${INSTALL_DIR}/bin/diane"
    elif [ -f "${TMP_DIR}/diane-mcp" ]; then
        mv "${TMP_DIR}/diane-mcp" "${INSTALL_DIR}/bin/diane"
    else
        error "Binary not found in tarball"
    fi
    chmod +x "${INSTALL_DIR}/bin/diane"
    
    success "DIANE installed to ${INSTALL_DIR}/bin/diane"
    
    # Path warning
    case ":${PATH}:" in
        *":${INSTALL_DIR}/bin:"*)
            ;;
        *)
            echo ""
            warn "Add ${INSTALL_DIR}/bin to your PATH:"
            echo ""
            echo "  export PATH=\"\${HOME}/.diane/bin:\${PATH}\""
            echo ""
            ;;
    esac
}

# Uninstall
uninstall() {
    detect_platform
    
    if [ "$OS" = "linux" ] && is_arch_linux; then
        info "Uninstalling via pacman..."
        if command -v sudo >/dev/null 2>&1; then
            sudo pacman -Rns diane
        else
            su -c "pacman -Rns diane"
        fi
        success "Uninstalled."
    else
        INSTALL_DIR="${DIANE_DIR:-$DEFAULT_INSTALL_DIR}"
        if [ ! -d "${INSTALL_DIR}" ]; then
            error "DIANE is not installed at ${INSTALL_DIR}"
        fi
        
        info "Uninstalling from ${INSTALL_DIR}..."
        rm -rf "${INSTALL_DIR}/bin/diane"
        rm -rf "${INSTALL_DIR}/bin/diane-mcp"
        
        success "Binaries removed."
        warn "Data directory preserved at ${INSTALL_DIR}/data"
        echo "  To remove completely: rm -rf ${INSTALL_DIR}"
    fi
}

version() {
    detect_platform
    if [ "$OS" = "linux" ] && is_arch_linux; then
        if pacman -Qi diane >/dev/null 2>&1; then
            pacman -Qi diane | grep Version
        else
            echo "diane is not installed via pacman"
        fi
    else
        INSTALL_DIR="${DIANE_DIR:-$DEFAULT_INSTALL_DIR}"
        if [ -x "${INSTALL_DIR}/bin/diane" ]; then
            "${INSTALL_DIR}/bin/diane" version
        else
            echo "diane is not installed at ${INSTALL_DIR}/bin/diane"
        fi
    fi
}

# Main
main() {
    CMD="${1:-install}"
    detect_platform
    
    case "$CMD" in
        install|upgrade)
            # Check for Arch
            if [ "$OS" = "linux" ] && is_arch_linux; then
                install_arch
            else
                install_generic
            fi
            ;;
        uninstall)
            uninstall
            ;;
        version)
            version
            ;;
        --help|-h)
            echo "DIANE Installer"
            echo "Usage: $0 [install|upgrade|uninstall|version]"
            ;;
        *)
            # If no argument or unknown, default to install
            if [ "$OS" = "linux" ] && is_arch_linux; then
                install_arch
            else
                install_generic
            fi
            ;;
    esac
}

main "$@"
