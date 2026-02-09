#!/bin/bash

# Capture Hero Logo
# Screenshots the website hero logo element and copies it to .github/assets/logo.png
#
# Usage:
#   ./scripts/capture_hero.sh          # auto-detect port or start dev server
#   ./scripts/capture_hero.sh 3000     # use specific port

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
CYAN='\033[0;36m'
NC='\033[0m'

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
WEBSITE_DIR="$ROOT_DIR/website"
CAPTURE_SCRIPT="$WEBSITE_DIR/capture-hero.mjs"
OUTPUT="$WEBSITE_DIR/static/img/hero-logo.png"
TARGET="$ROOT_DIR/.github/assets/logo.png"

started_server=false

cleanup() {
  if $started_server; then
    echo -e "${YELLOW}Stopping dev server (pid $server_pid)...${NC}"
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
}
trap cleanup EXIT

# Check if a port is serving the Docusaurus site (look for "skillshare" in response)
is_docusaurus() {
  curl -s "http://localhost:$1" 2>/dev/null | grep -q "skillshare"
}

# --- 1. Detect or start dev server ---
if [ -n "$1" ]; then
  PORT="$1"
  echo -e "${CYAN}Using specified port: $PORT${NC}"
else
  PORT=""
  for p in 3002 3001 3000; do
    if is_docusaurus "$p"; then
      PORT=$p
      echo -e "${CYAN}Detected Docusaurus on port $PORT${NC}"
      break
    fi
  done

  if [ -z "$PORT" ]; then
    echo -e "${YELLOW}No Docusaurus dev server detected, starting one...${NC}"
    cd "$WEBSITE_DIR"
    npm start -- --port 3099 &>/dev/null &
    server_pid=$!
    started_server=true
    PORT=3099

    for i in $(seq 1 30); do
      if is_docusaurus "$PORT"; then
        break
      fi
      if [ "$i" -eq 30 ]; then
        echo -e "${RED}Dev server failed to start within 30s${NC}"
        exit 1
      fi
      sleep 1
    done
    echo -e "${GREEN}Dev server ready on port $PORT${NC}"
  fi
fi

# --- 2. Ensure playwright is available ---
if ! node -e "require('playwright')" 2>/dev/null; then
  echo -e "${YELLOW}Installing playwright...${NC}"
  cd "$WEBSITE_DIR"
  npm install --no-save playwright >/dev/null 2>&1
  npx playwright install chromium >/dev/null 2>&1
  echo -e "${GREEN}Playwright installed${NC}"
fi

# --- 3. Capture ---
echo -e "${CYAN}Capturing hero logo from localhost:$PORT ...${NC}"
cd "$WEBSITE_DIR"
PORT="$PORT" node "$CAPTURE_SCRIPT"

# --- 4. Copy to README logo ---
cp "$OUTPUT" "$TARGET"
echo -e "${GREEN}Copied to $TARGET${NC}"

# --- 5. Show result ---
size=$(wc -c < "$TARGET" | tr -d ' ')
echo -e "${GREEN}Done!${NC} $(( size / 1024 ))KB → .github/assets/logo.png"
