# DIANE MCP Server Makefile
# Usage: make [target]

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BINARY_NAME := diane-mcp
BUILD_DIR := dist
SERVER_DIR := server/mcp

# Build flags
LDFLAGS := -s -w -X main.Version=$(VERSION)
CGO_ENABLED := 0

# Platforms
PLATFORMS := darwin-arm64 darwin-amd64 linux-amd64 linux-arm64

.PHONY: all build clean install test release help

## Default target
all: build

## Build for current platform
build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	cd $(SERVER_DIR) && go build -ldflags="$(LDFLAGS)" -o ../../$(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"

## Build for all platforms
build-all: $(PLATFORMS)

darwin-arm64:
	@echo "Building for darwin/arm64..."
	@mkdir -p $(BUILD_DIR)
	cd $(SERVER_DIR) && GOOS=darwin GOARCH=arm64 CGO_ENABLED=$(CGO_ENABLED) go build -ldflags="$(LDFLAGS)" -o ../../$(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .

darwin-amd64:
	@echo "Building for darwin/amd64..."
	@mkdir -p $(BUILD_DIR)
	cd $(SERVER_DIR) && GOOS=darwin GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) go build -ldflags="$(LDFLAGS)" -o ../../$(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .

linux-amd64:
	@echo "Building for linux/amd64..."
	@mkdir -p $(BUILD_DIR)
	cd $(SERVER_DIR) && GOOS=linux GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) go build -ldflags="$(LDFLAGS)" -o ../../$(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .

linux-arm64:
	@echo "Building for linux/arm64..."
	@mkdir -p $(BUILD_DIR)
	cd $(SERVER_DIR) && GOOS=linux GOARCH=arm64 CGO_ENABLED=$(CGO_ENABLED) go build -ldflags="$(LDFLAGS)" -o ../../$(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 .

## Create release archives
release: build-all
	@echo "Creating release archives..."
	@cd $(BUILD_DIR) && for f in $(BINARY_NAME)-*; do \
		tar -czvf $${f}.tar.gz $${f}; \
		shasum -a 256 $${f}.tar.gz > $${f}.tar.gz.sha256; \
	done
	@cd $(BUILD_DIR) && cat *.sha256 > checksums.txt
	@echo "Release archives created in $(BUILD_DIR)/"

## Install locally
install: build
	@echo "Installing to ~/.diane/bin/..."
	@mkdir -p ~/.diane/bin
	@cp $(BUILD_DIR)/$(BINARY_NAME) ~/.diane/bin/$(BINARY_NAME)
	@chmod +x ~/.diane/bin/$(BINARY_NAME)
	@echo "Installed: ~/.diane/bin/$(BINARY_NAME)"

## Run tests
test:
	cd server && go test -v ./...

## Run MCP server locally (for testing)
run: build
	./$(BUILD_DIR)/$(BINARY_NAME)

## Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@echo "Done"

## Show version
version:
	@echo "$(VERSION)"

## Show help
help:
	@echo "DIANE MCP Server Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build       Build for current platform (default)"
	@echo "  build-all   Build for all platforms (darwin/linux, arm64/amd64)"
	@echo "  release     Build all platforms and create release archives"
	@echo "  install     Install to ~/.diane/bin/"
	@echo "  test        Run tests"
	@echo "  run         Build and run locally"
	@echo "  clean       Remove build artifacts"
	@echo "  version     Show version"
	@echo "  help        Show this help"
	@echo ""
	@echo "Examples:"
	@echo "  make                    # Build for current platform"
	@echo "  make install            # Build and install locally"
	@echo "  make VERSION=v1.0.0 release  # Create versioned release"
