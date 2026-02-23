#!/usr/bin/env bash
# Resolve GITHUB_TOKEN and print available commands on container start.
set -euo pipefail

# Re-assert Linux-safe command entrypoints on every container start.
if [ -x /workspace/.devcontainer/ensure-skillshare-linux-binary.sh ]; then
  /workspace/.devcontainer/ensure-skillshare-linux-binary.sh
fi

# Auto-detect GITHUB_TOKEN from gh CLI if not already set
# (via .env, remoteEnv, or manual export).
if [ -z "${GITHUB_TOKEN:-}" ] && command -v gh &>/dev/null; then
  token="$(gh auth token 2>/dev/null || true)"
  if [ -n "$token" ]; then
    export GITHUB_TOKEN="$token"
    echo "GITHUB_TOKEN auto-detected from gh CLI"
  fi
fi

echo "Dev servers ready:"
echo "  ui          # global-mode dashboard → :5173"
echo "  ui -p       # project-mode dashboard → :5173"
echo "  ui stop     # stop dashboard"
echo "  docs        # documentation site → :3000"
echo "  docs stop   # stop docs"
echo "  ss ...      # instant mode (go run, no manual rebuild)"
