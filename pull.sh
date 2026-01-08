#!/bin/bash

# Pull script to run on Raspberry Pi
# This script pulls the latest code from git and restarts the service
# Usage: ./pull.sh

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
SERVICE_NAME="price-monitoring-bot"
BINARY_NAME="price-monitoring-bot"
PROJECT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo -e "${GREEN}Starting update process...${NC}"

# Check if we're in a git repository
if [ ! -d ".git" ]; then
    echo -e "${RED}Error: Not in a git repository!${NC}"
    exit 1
fi

# Check if there are uncommitted changes
if ! git diff-index --quiet HEAD --; then
    echo -e "${YELLOW}Warning: You have uncommitted changes.${NC}"
    read -p "Continue anyway? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${YELLOW}Update cancelled.${NC}"
        exit 0
    fi
fi

# Pull latest changes from GitHub
echo -e "${YELLOW}Pulling latest changes from GitHub...${NC}"

# Fetch from origin (GitHub)
git fetch origin

if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Failed to fetch from GitHub!${NC}"
    exit 1
fi

# Pull from origin/main (or origin/master)
BRANCH=$(git branch --show-current)
if [ -z "$BRANCH" ]; then
    BRANCH="main"
fi

echo -e "${YELLOW}Pulling from origin/${BRANCH}...${NC}"
git pull origin "${BRANCH}"

if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Failed to pull from GitHub!${NC}"
    exit 1
fi

echo -e "${GREEN}GitHub pull successful!${NC}"

# Check if service exists
if ! systemctl list-unit-files | grep -q "${SERVICE_NAME}.service"; then
    echo -e "${YELLOW}Warning: Service ${SERVICE_NAME} not found.${NC}"
    echo -e "${YELLOW}You may need to create a systemd service file.${NC}"
    read -p "Continue with build only? (y/n) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 0
    fi
    SKIP_SERVICE=true
else
    SKIP_SERVICE=false
fi

# Build the application
echo -e "${YELLOW}Building application...${NC}"
CGO_ENABLED=1 go build -o "${BINARY_NAME}" -ldflags="-s -w" ./cmd/bot/main.go

if [ ! -f "${BINARY_NAME}" ]; then
    echo -e "${RED}Error: Build failed!${NC}"
    exit 1
fi

echo -e "${GREEN}Build successful!${NC}"

# Restart service if it exists
if [ "$SKIP_SERVICE" = false ]; then
    echo -e "${YELLOW}Restarting ${SERVICE_NAME} service...${NC}"
    sudo systemctl restart "${SERVICE_NAME}"
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}Service restarted successfully!${NC}"
        echo -e "${YELLOW}Service status:${NC}"
        sudo systemctl status "${SERVICE_NAME}" --no-pager -l
    else
        echo -e "${RED}Error: Failed to restart service!${NC}"
        exit 1
    fi
fi

echo -e "${GREEN}Update complete!${NC}"

