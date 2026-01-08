#!/bin/bash

# Push script for cross-compiling to Raspberry Pi and pushing to GitHub
# Usage: ./push.sh

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
BINARY_NAME="price-monitoring-bot"
BUILD_DIR="./build"

echo -e "${GREEN}Starting cross-compilation for Raspberry Pi...${NC}"

# Create build directory
mkdir -p "${BUILD_DIR}"

# Set Go environment variables for Raspberry Pi (ARM64)
export GOOS=linux
export GOARCH=arm64

echo -e "${YELLOW}Cross-compiling for Linux ARM64...${NC}"
go build -o "${BUILD_DIR}/${BINARY_NAME}" -ldflags="-s -w" ./cmd/bot/main.go

if [ ! -f "${BUILD_DIR}/${BINARY_NAME}" ]; then
    echo -e "${RED}Error: Binary was not created!${NC}"
    exit 1
fi

echo -e "${GREEN}Binary compiled successfully!${NC}"
ls -lh "${BUILD_DIR}/${BINARY_NAME}"

# Check git status
if ! git diff-index --quiet HEAD --; then
    echo -e "${YELLOW}Uncommitted changes detected.${NC}"
    read -p "Commit changes before pushing? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${YELLOW}Staging changes...${NC}"
        git add .
        read -p "Enter commit message: " commit_message
        if [ -z "$commit_message" ]; then
            commit_message="Update: Cross-compiled for Raspberry Pi"
        fi
        git commit -m "$commit_message"
        echo -e "${GREEN}Changes committed!${NC}"
    fi
fi

# Push to GitHub
echo -e "${YELLOW}Pushing to GitHub...${NC}"
git push

echo -e "${GREEN}Push to GitHub complete!${NC}"

