#!/bin/bash
# Build script for DianeMenu.app with embedded diane binary
# This builds both the Go CLI and Swift app together

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BUILD_DIR="$PROJECT_ROOT/dist"
APP_BUILD_DIR="$PROJECT_ROOT/DianeMenu/build"

# Get version from git
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"

# Detect architecture
ARCH=$(uname -m)
if [ "$ARCH" = "arm64" ]; then
    GO_ARCH="arm64"
elif [ "$ARCH" = "x86_64" ]; then
    GO_ARCH="amd64"
else
    echo "Unsupported architecture: $ARCH"
    exit 1
fi

echo "=== Building DianeMenu.app with embedded diane binary ==="
echo "Version: $VERSION"
echo "Architecture: darwin/$GO_ARCH"
echo ""

# Step 1: Build Go binary
echo "=== Step 1: Building diane Go binary ==="
mkdir -p "$BUILD_DIR"
cd "$PROJECT_ROOT/server/mcp"
CGO_ENABLED=1 GOOS=darwin GOARCH=$GO_ARCH go build \
    -ldflags="-s -w -X main.Version=$VERSION" \
    -o "$BUILD_DIR/diane" .
echo "Built: $BUILD_DIR/diane"

# Step 2: Build Swift app
echo ""
echo "=== Step 2: Building DianeMenu Swift app ==="
cd "$PROJECT_ROOT/DianeMenu"

# Determine build configuration
CONFIGURATION="${CONFIGURATION:-Release}"

xcodebuild \
    -project DianeMenu.xcodeproj \
    -scheme DianeMenu \
    -configuration "$CONFIGURATION" \
    -derivedDataPath "$APP_BUILD_DIR" \
    -arch "$ARCH" \
    build \
    MARKETING_VERSION="$VERSION" \
    2>&1 | grep -E "^(Build|Compile|Link|Sign|warning:|error:|\*\*)" || true

# Find the built app
APP_PATH="$APP_BUILD_DIR/Build/Products/$CONFIGURATION/DianeMenu.app"
if [ ! -d "$APP_PATH" ]; then
    echo "Error: App not found at $APP_PATH"
    exit 1
fi
echo "Built: $APP_PATH"

# Step 3: Embed diane binary in app bundle
echo ""
echo "=== Step 3: Embedding diane binary in app bundle ==="
MACOS_DIR="$APP_PATH/Contents/MacOS"
cp "$BUILD_DIR/diane" "$MACOS_DIR/diane"
chmod +x "$MACOS_DIR/diane"
echo "Embedded: $MACOS_DIR/diane"

# Step 4: Copy to dist folder for easy access
echo ""
echo "=== Step 4: Copying app to dist/ ==="
rm -rf "$BUILD_DIR/DianeMenu.app"
cp -R "$APP_PATH" "$BUILD_DIR/DianeMenu.app"
echo "Copied: $BUILD_DIR/DianeMenu.app"

echo ""
echo "=== Build Complete ==="
echo ""
echo "App bundle: $BUILD_DIR/DianeMenu.app"
echo "  - DianeMenu (Swift menu bar app)"
echo "  - diane (Go CLI binary embedded)"
echo ""
echo "To install:"
echo "  1. Copy DianeMenu.app to /Applications"
echo "  2. The app will create ~/.diane/bin/diane symlink on first launch"
echo ""
