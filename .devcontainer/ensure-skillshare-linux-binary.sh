#!/usr/bin/env bash
# Ensure devcontainer commands point to a Linux-built skillshare binary.
set -euo pipefail

if [ ! -d /workspace ] || [ ! -f /workspace/go.mod ]; then
  echo "ensure-skillshare-linux-binary: /workspace is not available" >&2
  exit 1
fi

BIN_DIR=/workspace/bin
BIN=$BIN_DIR/skillshare
NEED_REBUILD=0
REASON=""
GO_BIN=/usr/local/go/bin/go

mkdir -p "$BIN_DIR"

if [ ! -x "$BIN" ]; then
  NEED_REBUILD=1
  REASON="binary missing"
elif command -v file >/dev/null 2>&1; then
  FILE_TYPE="$(file -b "$BIN" || true)"
  if [[ "$FILE_TYPE" != *ELF* ]]; then
    NEED_REBUILD=1
    REASON="non-Linux binary detected: $FILE_TYPE"
  fi
fi

if [ "$NEED_REBUILD" -eq 0 ] && ! "$BIN" version >/dev/null 2>&1; then
  NEED_REBUILD=1
  REASON="binary exists but cannot execute in container"
fi

if [ ! -x "$GO_BIN" ]; then
  GO_BIN="$(command -v go || true)"
fi
if [ -z "${GO_BIN:-}" ]; then
  echo "ensure-skillshare-linux-binary: go toolchain not found" >&2
  exit 1
fi

if [ "$NEED_REBUILD" -eq 1 ]; then
  echo "▸ Rebuilding skillshare for Linux ($REASON) ..."
  (
    cd /workspace
    "$GO_BIN" build -o "$BIN" ./cmd/skillshare
  )
fi

# Keep command resolution deterministic in interactive shells.
ln -sf "$BIN" /workspace/bin/ss
ln -sf "$BIN" /usr/local/bin/skillshare
ln -sf "$BIN" /usr/local/bin/ss
