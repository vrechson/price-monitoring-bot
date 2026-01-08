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
VERSION_FILE="VERSION"

echo -e "${GREEN}Starting cross-compilation for Raspberry Pi...${NC}"

# Read and increment version
if [ ! -f "${VERSION_FILE}" ]; then
    echo "1.0.0" > "${VERSION_FILE}"
fi

CURRENT_VERSION=$(cat "${VERSION_FILE}" | tr -d '[:space:]')
echo -e "${YELLOW}Current version: ${CURRENT_VERSION}${NC}"

# Increment version (simple increment of patch version)
IFS='.' read -ra VERSION_PARTS <<< "${CURRENT_VERSION}"
MAJOR="${VERSION_PARTS[0]}"
MINOR="${VERSION_PARTS[1]}"
PATCH="${VERSION_PARTS[2]}"
PATCH=$((PATCH + 1))
NEW_VERSION="${MAJOR}.${MINOR}.${PATCH}"

echo "${NEW_VERSION}" > "${VERSION_FILE}"
echo -e "${GREEN}New version: ${NEW_VERSION}${NC}"

# Create build directory
mkdir -p "${BUILD_DIR}"

# Set Go environment variables for Raspberry Pi (ARM64)
export GOOS=linux
export GOARCH=arm64

echo -e "${YELLOW}Cross-compiling for Linux ARM64...${NC}"
go build -o "${BUILD_DIR}/${BINARY_NAME}" -ldflags="-s -w -X main.Version=${NEW_VERSION}" ./cmd/bot/main.go

if [ ! -f "${BUILD_DIR}/${BINARY_NAME}" ]; then
    echo -e "${RED}Error: Binary was not created!${NC}"
    exit 1
fi

echo -e "${GREEN}Binary compiled successfully!${NC}"
ls -lh "${BUILD_DIR}/${BINARY_NAME}"

# Stage changes (including VERSION file)
echo -e "${YELLOW}Staging changes...${NC}"
git add .

# Commit with version message
COMMIT_MESSAGE="v${NEW_VERSION}: Cross-compiled for Raspberry Pi"
echo -e "${YELLOW}Committing with message: ${COMMIT_MESSAGE}${NC}"
git commit -m "${COMMIT_MESSAGE}"
echo -e "${GREEN}Changes committed!${NC}"

# Push to GitHub
echo -e "${YELLOW}Pushing to GitHub...${NC}"
git push

echo -e "${GREEN}Push to GitHub complete!${NC}"

