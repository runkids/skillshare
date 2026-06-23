#!/bin/bash

# Skillshare Build Script
# Build executable for local testing

set -e

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Project directory
PROJECT_DIR="$(cd "$(dirname "$0")" && pwd)"
OUTPUT_DIR="${PROJECT_DIR}/bin"
BINARY_NAME="skillshare"

# Header
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Skillshare Build Script${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo

# Create output directory
mkdir -p "${OUTPUT_DIR}"

# Get version info. Only exact, clean release tags produce update-checkable
# versions; local builds from commits or dirty trees are dev builds.
if git describe --tags --exact-match >/dev/null 2>&1 && git diff --quiet && git diff --cached --quiet; then
    VERSION=$(git describe --tags --exact-match 2>/dev/null)
    VERSION="${VERSION#v}"
else
    VERSION="dev"
fi
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')

echo -e "${YELLOW}Version:${NC} ${VERSION}"
echo -e "${YELLOW}Time:${NC}    ${BUILD_TIME}"
echo

# Build
echo -e "${CYAN}Building...${NC}"
cd "${PROJECT_DIR}"

go build \
    -ldflags "-X main.version=${VERSION}" \
    -o "${OUTPUT_DIR}/${BINARY_NAME}" \
    ./cmd/skillshare

echo -e "${GREEN}✓${NC} Build complete: ${OUTPUT_DIR}/${BINARY_NAME}"
echo

# Show file info
ls -lh "${OUTPUT_DIR}/${BINARY_NAME}"
echo

# Run tests
echo -e "${CYAN}Running tests...${NC}"
go test ./... -v 2>&1 | grep -E '(PASS|FAIL|ok|---)'
echo

# Usage instructions
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}Usage:${NC}"
echo
echo -e "  ${YELLOW}Run directly:${NC}"
echo "    ./bin/skillshare help"
echo "    ./bin/skillshare status"
echo "    ./bin/skillshare backup --list"
echo
echo -e "  ${YELLOW}Add to PATH (current session):${NC}"
echo "    export PATH=\"${OUTPUT_DIR}:\$PATH\""
echo "    skillshare help"
echo
echo -e "  ${YELLOW}Create alias:${NC}"
echo "    alias ss='${OUTPUT_DIR}/${BINARY_NAME}'"
echo "    ss status"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
