#!/bin/sh
# skillshare runner - npx-style execution without installation
# Usage: sh run.sh [command] [args]
# Download: curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/skills/skillshare/scripts/run.sh -o run.sh
# Or:    sh run.sh [command] [args]
set -e

REPO="runkids/skillshare"
CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/skillshare"
BIN_DIR="$CACHE_DIR/bin"
BINARY="$BIN_DIR/skillshare"
VERSION_FILE="$CACHE_DIR/.version"
MAX_AGE=86400  # Check for updates every 24 hours

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info() { printf "${CYAN}[skillshare]${NC} %s\n" "$1" >&2; }
success() { printf "${GREEN}[skillshare]${NC} %s\n" "$1" >&2; }
warn() { printf "${YELLOW}[skillshare]${NC} %s\n" "$1" >&2; }
error() { printf "${RED}[skillshare]${NC} %s\n" "$1" >&2; exit 1; }

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

# Get file modification time in seconds since epoch (cross-platform)
get_file_age() {
  if [ "$(uname -s)" = "Darwin" ]; then
    stat -f %m "$1" 2>/dev/null || echo 0
  else
    stat -c %Y "$1" 2>/dev/null || echo 0
  fi
}

# Check if we need to update (version file older than MAX_AGE)
needs_update() {
  [ ! -f "$VERSION_FILE" ] && return 0
  [ ! -x "$BINARY" ] && return 0

  file_time=$(get_file_age "$VERSION_FILE")
  current_time=$(date +%s)
  age=$((current_time - file_time))

  [ "$age" -ge "$MAX_AGE" ]
}

# Get latest version from GitHub API
get_latest_version() {
  LATEST=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
  if [ -z "$LATEST" ]; then
    error "Failed to get latest version. Check your internet connection."
  fi
  VERSION=${LATEST#v}
}

# Download and install to cache
download_binary() {
  detect_os
  detect_arch
  get_latest_version

  URL="https://github.com/${REPO}/releases/download/${LATEST}/skillshare_${VERSION}_${OS}_${ARCH}.tar.gz"

  info "Downloading skillshare ${VERSION} for ${OS}/${ARCH}..."

  mkdir -p "$BIN_DIR"

  TMP_DIR=$(mktemp -d)
  trap "rm -rf $TMP_DIR" EXIT

  if ! curl -sL "$URL" | tar xz -C "$TMP_DIR" 2>/dev/null; then
    error "Failed to download. URL: $URL"
  fi

  if [ ! -f "$TMP_DIR/skillshare" ]; then
    error "Binary not found in archive"
  fi

  mv "$TMP_DIR/skillshare" "$BINARY"
  chmod +x "$BINARY"
  echo "$VERSION" > "$VERSION_FILE"

  success "Cached skillshare ${VERSION} at ${BIN_DIR}"
}

# Main logic
main() {
  # 1. Check if skillshare is already in PATH
  if command -v skillshare >/dev/null 2>&1; then
    exec skillshare "$@"
  fi

  # 2. Check cached binary
  if [ -x "$BINARY" ]; then
    if ! needs_update; then
      exec "$BINARY" "$@"
    fi

    # Try to update, but fall back to cached version on failure
    if download_binary 2>/dev/null; then
      exec "$BINARY" "$@"
    else
      warn "Update check failed, using cached version"
      touch "$VERSION_FILE"  # Reset timer
      exec "$BINARY" "$@"
    fi
  fi

  # 3. First run - download binary
  download_binary
  exec "$BINARY" "$@"
}

main "$@"
