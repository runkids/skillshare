#!/bin/sh
set -e

REPO="runkids/skillshare"
BINARY_NAME="skillshare"
if [ -z "${INSTALL_DIR}" ]; then
	INSTALL_DIR="/usr/local/bin"
fi

# Colors (if terminal supports it)
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

info() {
  printf "${GREEN}%s${NC}\n" "$1"
}

warn() {
  printf "${YELLOW}%s${NC}\n" "$1"
}

error() {
  printf "${RED}%s${NC}\n" "$1" >&2
  exit 1
}

# Detect OS
detect_os() {
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$OS" in
    darwin) OS="darwin" ;;
    linux) OS="linux" ;;
    mingw*|msys*|cygwin*) error "Use PowerShell: irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex" ;;
    *) error "Unsupported OS: $OS" ;;
  esac
}

# Detect architecture
detect_arch() {
  ARCH=$(uname -m)
  case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *) error "Unsupported architecture: $ARCH" ;;
  esac
}

# Get latest version using redirect (avoids API rate limit)
get_latest_version() {
  # Use redirect to get latest version (no API rate limit)
  LATEST=$(curl -sI "https://github.com/${REPO}/releases/latest" | grep -i "^location:" | sed 's/.*tag\/\([^[:space:]]*\).*/\1/' | tr -d '\r')

  # Fallback to API if redirect fails
  if [ -z "$LATEST" ]; then
    LATEST=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
  fi

  if [ -z "$LATEST" ]; then
    error "Failed to get latest version. Please check your internet connection."
  fi
  VERSION=${LATEST#v}
}

# Download and install
install() {
  URL="https://github.com/${REPO}/releases/download/${LATEST}/${BINARY_NAME}_${VERSION}_${OS}_${ARCH}.tar.gz"

  info "Downloading skillshare ${VERSION} for ${OS}/${ARCH}..."

  # Create temp directory
  TMP_DIR=$(mktemp -d)
  trap "rm -rf $TMP_DIR" EXIT

  # Download and extract
  if ! curl -sL "$URL" | tar xz -C "$TMP_DIR" 2>/dev/null; then
    error "Failed to download or extract. URL: $URL"
  fi

  # Check if binary exists
  if [ ! -f "$TMP_DIR/$BINARY_NAME" ]; then
    error "Binary not found in archive"
  fi

  # Install
  if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/"
  else
    warn "Need sudo to install to $INSTALL_DIR"
    sudo mv "$TMP_DIR/$BINARY_NAME" "$INSTALL_DIR/"
  fi

  chmod +x "$INSTALL_DIR/$BINARY_NAME"
}

# Verify installation
verify() {
  if command -v "$BINARY_NAME" >/dev/null 2>&1; then
    info ""
    info "Successfully installed skillshare to $INSTALL_DIR/$BINARY_NAME"
    info ""
    "$BINARY_NAME" version
    info ""
    info "Get started:"
    info "  skillshare init"
    info "  skillshare --help"
  else
    warn "Installed but '$BINARY_NAME' not in PATH. Add $INSTALL_DIR to your PATH."
  fi
}

main() {
  info "Installing skillshare..."
  info ""

  detect_os
  detect_arch
  get_latest_version
  install
  verify
}

main
