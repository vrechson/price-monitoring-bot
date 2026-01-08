#!/bin/bash

# Push script for cross-compiling and deploying to Raspberry Pi 5
# Usage: ./push.sh [raspberry-pi-host] [user]

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
BINARY_NAME="price-monitoring-bot"
SERVICE_NAME="price-monitoring-bot"
RASPBERRY_PI_HOST="${1:-raspberrypi}"
RASPBERRY_PI_USER="${2:-vrechson}"
REMOTE_DIR="/home/${RASPBERRY_PI_USER}/price-monitoring-bot"
BUILD_DIR="./build"

echo -e "${GREEN}Starting cross-compilation for Raspberry Pi 5...${NC}"

# Create build directory
mkdir -p "${BUILD_DIR}"

# Set Go environment variables for Raspberry Pi 5 (ARM64)
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

# Check if we should deploy
read -p "Deploy to ${RASPBERRY_PI_USER}@${RASPBERRY_PI_HOST}? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}Deployment cancelled. Binary is available at ${BUILD_DIR}/${BINARY_NAME}${NC}"
    exit 0
fi

echo -e "${YELLOW}Deploying to Raspberry Pi...${NC}"

# Create remote directory if it doesn't exist
ssh "${RASPBERRY_PI_USER}@${RASPBERRY_PI_HOST}" "mkdir -p ${REMOTE_DIR}"

# Copy binary to Raspberry Pi
scp "${BUILD_DIR}/${BINARY_NAME}" "${RASPBERRY_PI_USER}@${RASPBERRY_PI_HOST}:${REMOTE_DIR}/${BINARY_NAME}"

# Make binary executable
ssh "${RASPBERRY_PI_USER}@${RASPBERRY_PI_HOST}" "chmod +x ${REMOTE_DIR}/${BINARY_NAME}"

echo -e "${GREEN}Binary deployed successfully!${NC}"

# Ask if user wants to restart the service
read -p "Restart ${SERVICE_NAME} service? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${YELLOW}Restarting service...${NC}"
    ssh "${RASPBERRY_PI_USER}@${RASPBERRY_PI_HOST}" "sudo systemctl restart ${SERVICE_NAME}"
    echo -e "${GREEN}Service restarted!${NC}"
    echo -e "${YELLOW}Service status:${NC}"
    ssh "${RASPBERRY_PI_USER}@${RASPBERRY_PI_HOST}" "sudo systemctl status ${SERVICE_NAME} --no-pager -l"
fi

echo -e "${GREEN}Deployment complete!${NC}"

