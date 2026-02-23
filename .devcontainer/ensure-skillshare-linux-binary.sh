#!/usr/bin/env bash
# Ensure devcontainer commands point to a Linux-built skillshare binary.
set -euo pipefail

if [ ! -d /workspace ] || [ ! -f /workspace/go.mod ]; then
  echo "ensure-skillshare-linux-binary: /workspace is not available" >&2
  exit 1
fi

BIN_DIR=/workspace/bin
BIN=$BIN_DIR/skillshare
WRAPPER_DIR=/workspace/.devcontainer/bin
WRAPPER_SKILLSHARE=$WRAPPER_DIR/skillshare
WRAPPER_SS=$WRAPPER_DIR/ss
WRAPPER_SSENV=$WRAPPER_DIR/ssenv
WRAPPER_SSHELP=$WRAPPER_DIR/sshelp
WRAPPER_SSNEW=$WRAPPER_DIR/ssnew
WRAPPER_SSUSE=$WRAPPER_DIR/ssuse
WRAPPER_SSBACK=$WRAPPER_DIR/ssback
WRAPPER_SSLS=$WRAPPER_DIR/ssls
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

# Prefer wrapper entrypoints for /usr/local/bin so commands stay Linux-safe
# even when /workspace/bin/skillshare is overwritten by a host-built binary.
if [ -x "$WRAPPER_SKILLSHARE" ] && [ -x "$WRAPPER_SS" ]; then
  ln -sf "$WRAPPER_SKILLSHARE" /usr/local/bin/skillshare
  ln -sf "$WRAPPER_SS" /usr/local/bin/ss
else
  ln -sf "$BIN" /usr/local/bin/skillshare
  ln -sf "$BIN" /usr/local/bin/ss
fi

if [ -x "$WRAPPER_SSENV" ]; then
  ln -sf "$WRAPPER_SSENV" /usr/local/bin/ssenv
fi

if [ -x "$WRAPPER_SSHELP" ]; then
  ln -sf "$WRAPPER_SSHELP" /usr/local/bin/sshelp
fi

if [ -x "$WRAPPER_SSNEW" ]; then
  ln -sf "$WRAPPER_SSNEW" /usr/local/bin/ssnew
fi
if [ -x "$WRAPPER_SSUSE" ]; then
  ln -sf "$WRAPPER_SSUSE" /usr/local/bin/ssuse
fi
if [ -x "$WRAPPER_SSBACK" ]; then
  ln -sf "$WRAPPER_SSBACK" /usr/local/bin/ssback
fi
if [ -x "$WRAPPER_SSLS" ]; then
  ln -sf "$WRAPPER_SSLS" /usr/local/bin/ssls
fi
