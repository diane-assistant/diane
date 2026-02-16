#!/bin/bash
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

echo "Running pre-commit checks..."

# Check if we are in the root directory
if [ ! -d "server" ]; then
    echo -e "${RED}Error: server directory not found. Please run from project root.${NC}"
    exit 1
fi

cd server

# 1. Format Check
echo "Checking code formatting..."
files=$(gofmt -l .)
if [ -n "$files" ]; then
    echo -e "${RED}Go code is not formatted. The following files need formatting:${NC}"
    echo "$files"
    echo -e "${RED}Run 'cd server && go fmt ./...' to fix.${NC}"
    exit 1
fi

# 2. Go Vet (Linting)
echo "Running go vet..."
if ! go vet ./...; then
    echo -e "${RED}go vet failed.${NC}"
    exit 1
fi

# 3. Build Check
echo "Verifying build..."
if ! go build ./...; then
    echo -e "${RED}Build failed.${NC}"
    exit 1
fi

echo -e "${GREEN}All pre-commit checks passed!${NC}"
exit 0
