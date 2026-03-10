#!/usr/bin/env bash
# Ensure mdproof is available in the devcontainer.
# Priority: already installed → GitHub release → go install → local dev binary
set -euo pipefail

REPO="runkids/mdproof"
BINARY_NAME="mdproof"

# Already installed and working?
if command -v "$BINARY_NAME" >/dev/null 2>&1 && "$BINARY_NAME" --version >/dev/null 2>&1; then
  echo "✓ mdproof already installed: $("$BINARY_NAME" --version)"
  exit 0
fi

# Try installing from GitHub release
install_from_release() {
  local version arch url tmp_dir

  # Detect architecture
  arch=$(uname -m)
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) echo "Unsupported architecture: $arch" >&2; return 1 ;;
  esac

  # Get latest version via redirect (avoids API rate limit)
  version=$(curl -sI "https://github.com/${REPO}/releases/latest" 2>/dev/null \
    | grep -i "^location:" \
    | sed 's/.*tag\/\([^[:space:]]*\).*/\1/' \
    | tr -d '\r')

  if [ -z "$version" ]; then
    # Fallback to API
    version=$(curl -sL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
      | grep '"tag_name"' | cut -d'"' -f4)
  fi

  if [ -z "$version" ]; then
    return 1
  fi

  url="https://github.com/${REPO}/releases/download/${version}/${BINARY_NAME}-${version}-linux-${arch}.tar.gz"

  echo "▸ Installing mdproof ${version} from release..."

  tmp_dir=$(mktemp -d)
  trap "rm -rf $tmp_dir" RETURN

  if ! curl -fsSL "$url" | tar xz -C "$tmp_dir" 2>/dev/null; then
    return 1
  fi

  # Binary inside tar is named mdproof-{version}-linux-{arch}
  local extracted
  extracted=$(find "$tmp_dir" -name "${BINARY_NAME}-*" -type f | head -1)
  if [ -z "$extracted" ]; then
    return 1
  fi

  sudo mv "$extracted" "/usr/local/bin/$BINARY_NAME"
  sudo chmod +x "/usr/local/bin/$BINARY_NAME"
  echo "✓ Installed mdproof ${version} from release"
  return 0
}

# Try release first
if install_from_release; then
  exit 0
fi

# Fallback: go install (container has Go toolchain)
if command -v go >/dev/null 2>&1; then
  echo "▸ Installing mdproof via go install..."
  if go install "github.com/${REPO}/cmd/mdproof@latest" 2>/dev/null; then
    # go install puts binary in $GOPATH/bin or $HOME/go/bin
    GOBIN="$(go env GOPATH)/bin/mdproof"
    if [ -x "$GOBIN" ]; then
      sudo ln -sf "$GOBIN" /usr/local/bin/mdproof
      echo "✓ Installed mdproof via go install"
      exit 0
    fi
  fi
fi

# Fallback: local dev binary (from `make dev-skillshare` in ../mdproof)
if [ -x /workspace/bin/mdproof ]; then
  if /workspace/bin/mdproof --version >/dev/null 2>&1; then
    sudo ln -sf /workspace/bin/mdproof /usr/local/bin/mdproof
    echo "✓ Using local dev binary: /workspace/bin/mdproof"
    exit 0
  fi
fi

echo "✗ Could not install mdproof." >&2
echo "  Run 'make dev-skillshare' from ../mdproof on the host" >&2
exit 1
