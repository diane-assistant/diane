#!/bin/bash
# Build and install Diane.app with all components
# This builds the Go CLI, Swift app, and installs everything

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BUILD_DIR="$PROJECT_ROOT/dist"

# Installation paths
INSTALL_APP_DIR="${INSTALL_APP_DIR:-/Applications}"
DIANE_HOME="$HOME/.diane"
DIANE_BIN="$DIANE_HOME/bin"

# Get version from git
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_step() {
    echo -e "${BLUE}==>${NC} $1"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}!${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║           Diane - Build & Install                        ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""
echo "Version: $VERSION"
echo "Install location: $INSTALL_APP_DIR/Diane.app"
echo ""

# Check if we're on macOS
if [[ "$(uname)" != "Darwin" ]]; then
    print_error "This script only works on macOS"
    exit 1
fi

# Step 1: Build everything
print_step "Building Diane.app with embedded binaries..."
echo ""

# Run the build script
"$SCRIPT_DIR/build-app.sh"

echo ""

# Also build the additional CLI tools
print_step "Building additional CLI tools (diane-ctl, acp-server)..."
cd "$PROJECT_ROOT"
make build-ctl build-acp
print_success "Built diane-ctl and acp-server"
echo ""

# Step 2: Stop running Diane if any
print_step "Checking for running Diane instance..."
if [ -f "$DIANE_HOME/mcp.pid" ]; then
    PID=$(cat "$DIANE_HOME/mcp.pid" 2>/dev/null || echo "")
    if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
        print_warning "Stopping running Diane instance (PID: $PID)..."
        kill "$PID" 2>/dev/null || true
        sleep 2
        # Force kill if still running
        if kill -0 "$PID" 2>/dev/null; then
            kill -9 "$PID" 2>/dev/null || true
        fi
        print_success "Stopped Diane"
    fi
fi

# Step 3: Close Diane if running
print_step "Checking for running Diane app..."
if pgrep -x "Diane" > /dev/null; then
    print_warning "Closing Diane app..."
    osascript -e 'quit app "Diane"' 2>/dev/null || pkill -x "Diane" 2>/dev/null || true
    sleep 1
    print_success "Closed Diane"
fi

# Step 4: Install the app
print_step "Installing Diane.app to $INSTALL_APP_DIR..."

# Remove old installation if exists
if [ -d "$INSTALL_APP_DIR/Diane.app" ]; then
    rm -rf "$INSTALL_APP_DIR/Diane.app"
    print_success "Removed old installation"
fi

# Copy new app
cp -R "$BUILD_DIR/Diane.app" "$INSTALL_APP_DIR/"
print_success "Installed Diane.app"

# Step 5: Create ~/.diane directory structure
print_step "Setting up ~/.diane directory..."
mkdir -p "$DIANE_BIN"
mkdir -p "$DIANE_HOME/secrets"
mkdir -p "$DIANE_HOME/data"
mkdir -p "$DIANE_HOME/logs"
print_success "Created directory structure"

# Step 6: Create symlink for diane CLI
print_step "Creating symlink for diane CLI..."
BUNDLED_DIANE="$INSTALL_APP_DIR/Diane.app/Contents/MacOS/diane-server"

# Remove existing binary/symlink
if [ -e "$DIANE_BIN/diane" ] || [ -L "$DIANE_BIN/diane" ]; then
    rm -f "$DIANE_BIN/diane"
fi

# Create symlink
ln -s "$BUNDLED_DIANE" "$DIANE_BIN/diane"
print_success "Created symlink: ~/.diane/bin/diane -> $BUNDLED_DIANE"

# Step 7: Install additional CLI tools
print_step "Installing additional CLI tools..."
cp "$BUILD_DIR/diane-ctl" "$DIANE_BIN/diane-ctl"
cp "$BUILD_DIR/acp-server" "$DIANE_BIN/acp-server"
chmod +x "$DIANE_BIN/diane-ctl" "$DIANE_BIN/acp-server"
print_success "Installed diane-ctl and acp-server"

# Step 8: Check PATH
print_step "Checking PATH configuration..."
if [[ ":$PATH:" == *":$DIANE_BIN:"* ]]; then
    print_success "~/.diane/bin is already in PATH"
else
    print_warning "~/.diane/bin is not in PATH"
    echo ""
    echo "Add the following to your shell profile (~/.zshrc or ~/.bashrc):"
    echo ""
    echo "    export PATH=\"\$HOME/.diane/bin:\$PATH\""
    echo ""
fi

# Step 9: Launch the app
print_step "Launching Diane..."
open "$INSTALL_APP_DIR/Diane.app"
print_success "Diane started"

echo ""
echo "╔══════════════════════════════════════════════════════════╗"
echo "║           Installation Complete!                         ║"
echo "╚══════════════════════════════════════════════════════════╝"
echo ""
echo "Installed components:"
echo "  • Diane.app    → $INSTALL_APP_DIR/Diane.app"
echo "  • diane CLI        → ~/.diane/bin/diane (symlink to app)"
echo "  • diane-ctl        → ~/.diane/bin/diane-ctl"
echo "  • acp-server       → ~/.diane/bin/acp-server"
echo ""
echo "Version: $VERSION"
echo ""
echo "The Diane app is now running in your menu bar."
echo "Use 'diane' from the terminal to interact with the CLI."
echo ""
