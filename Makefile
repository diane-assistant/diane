# DIANE Makefile
# Usage: make [target]

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BINARY_NAME := diane
CTL_BINARY_NAME := diane-ctl
ACP_BINARY_NAME := acp-server
BUILD_DIR := dist
SERVER_DIR := server/mcp
CTL_DIR := server/cmd/diane-ctl
ACP_DIR := server/cmd/acp-server

# Build flags
LDFLAGS := -s -w -X main.Version=$(VERSION)
CGO_ENABLED := 1

# Platforms
PLATFORMS := darwin-arm64 darwin-amd64 linux-amd64 linux-arm64

.PHONY: all build build-ctl build-acp clean install test release help

## Default target
all: build build-ctl build-acp

## Build for current platform
build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	cd $(SERVER_DIR) && go build -ldflags="$(LDFLAGS)" -o ../../$(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"

## Build diane-ctl for current platform
build-ctl:
	@echo "Building $(CTL_BINARY_NAME)..."
	cd $(CTL_DIR) && go build -ldflags="$(LDFLAGS)" -o ../../../$(BUILD_DIR)/$(CTL_BINARY_NAME) .
	@echo "Built: $(BUILD_DIR)/$(CTL_BINARY_NAME)"

## Build acp-server for current platform
build-acp:
	@echo "Building $(ACP_BINARY_NAME)..."
	cd $(ACP_DIR) && go build -ldflags="$(LDFLAGS)" -o ../../../$(BUILD_DIR)/$(ACP_BINARY_NAME) .
	@echo "Built: $(BUILD_DIR)/$(ACP_BINARY_NAME)"

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
install: build build-ctl build-acp
	@echo "Installing to ~/.diane/bin/..."
	@mkdir -p ~/.diane/bin
	@cp $(BUILD_DIR)/$(BINARY_NAME) ~/.diane/bin/$(BINARY_NAME)
	@cp $(BUILD_DIR)/$(CTL_BINARY_NAME) ~/.diane/bin/$(CTL_BINARY_NAME)
	@cp $(BUILD_DIR)/$(ACP_BINARY_NAME) ~/.diane/bin/$(ACP_BINARY_NAME)
	@chmod +x ~/.diane/bin/$(BINARY_NAME)
	@chmod +x ~/.diane/bin/$(CTL_BINARY_NAME)
	@chmod +x ~/.diane/bin/$(ACP_BINARY_NAME)
	@echo "Installed: ~/.diane/bin/$(BINARY_NAME)"
	@echo "Installed: ~/.diane/bin/$(CTL_BINARY_NAME)"
	@echo "Installed: ~/.diane/bin/$(ACP_BINARY_NAME)"

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
	@echo "DIANE Build System"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build       Build diane for current platform"
	@echo "  build-ctl   Build diane-ctl for current platform"
	@echo "  build-acp   Build acp-server for current platform"
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
	@echo "  make                    # Build diane, diane-ctl, and acp-server"
	@echo "  make install            # Build and install locally"
	@echo "  make VERSION=v1.0.0 release  # Create versioned release"
