#!/usr/bin/env bash
# Ensure the terminal recording tools (vhs and what it drives: ttyd, ffmpeg,
# chromium) exist, so CLI and TUI screenshots can be recorded in the
# devcontainer. record-tui.sh runs it first, so they are installed the first
# time screenshots are taken instead of on every container start; once
# installed it skips straight through.
set -euo pipefail

# command -v succeeds when any one name is found, so check each.
have() {
  for cmd in "$@"; do
    command -v "$cmd" >/dev/null 2>&1 || return 1
  done
}

if have vhs ttyd ffmpeg chromium; then
  exit 0
fi

case "$(uname -m)" in
  x86_64|amd64) ttyd_arch="x86_64"; vhs_arch="x86_64" ;;
  arm64|aarch64) ttyd_arch="aarch64"; vhs_arch="arm64" ;;
  *) echo "⚠ No recording tools for $(uname -m)." >&2; exit 1 ;;
esac

# Latest release tag via redirect (avoids the API rate limit).
latest_tag() {
  curl -fsSI "https://github.com/$1/releases/latest" \
    | grep -i '^location:' | sed 's#.*/tag/##' | tr -d '\r'
}

echo "▸ Installing recording tools (vhs, ttyd, ffmpeg, chromium) …"
export DEBIAN_FRONTEND=noninteractive
if ! have ffmpeg chromium; then
  apt-get update -qq >/dev/null
  apt-get install -y -qq ffmpeg chromium fonts-jetbrains-mono >/dev/null
fi

if ! have ttyd; then
  curl -fsSL -o /usr/local/bin/ttyd \
    "https://github.com/tsl0922/ttyd/releases/latest/download/ttyd.${ttyd_arch}"
  chmod +x /usr/local/bin/ttyd
fi

if ! have vhs; then
  tag=$(latest_tag charmbracelet/vhs)
  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT
  curl -fsSL "https://github.com/charmbracelet/vhs/releases/download/${tag}/vhs_${tag#v}_Linux_${vhs_arch}.tar.gz" \
    | tar xz -C "$tmp"
  install "$(find "$tmp" -name vhs -type f | head -1)" /usr/local/bin/vhs
fi

echo "✓ Recording tools ready: $(vhs --version)"
