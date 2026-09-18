#!/usr/bin/env bash
# Host verification environment: the real Agent CLIs, a throwaway config.
#
# Plugin and MCP work is only exercised properly where claude, codex and the
# other CLIs actually live, which is the host rather than a container. So this
# runs a host build against a disposable HOME: every path skillshare and the
# Agent CLIs resolve is redirected under $VERIFY_HOME, leaving the real
# ~/.config/skillshare, ~/.claude and ~/.codex untouched. It defaults to
# .verify-home/ inside the repo, which is gitignored.
#
# Living inside the repo is safe because project-mode detection only looks at
# the working directory itself and never walks up to a parent, and every
# command here runs with the working directory set to $VERIFY_HOME. So the
# repo's own .skillshare/ is never picked up.
#
# The binary reports version "dev", and `skillshare ui` only downloads the
# dashboard from a GitHub Release for a numbered version. For "dev" it serves
# a cached copy if one exists and a dev-mode placeholder otherwise, so `up`
# seeds the cache from ui/dist and you see the working tree, never a release.
#
# Usage: ./scripts/verify.sh <up|shell|down|status|reset>
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$SCRIPT_DIR/_sandbox_common.sh"

REAL_HOME="$HOME"
VERIFY_HOME="${VERIFY_HOME:-$PROJECT_ROOT/.verify-home}"
VERIFY_PORT="${VERIFY_PORT:-19421}"
BIN="$PROJECT_ROOT/bin/skillshare-verify"
COMPOSE_FILE="$PROJECT_ROOT/.devcontainer/docker-compose.yml"
SERVICE="skillshare-devcontainer"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

usage() {
  echo "Usage: $(basename "$0") <command>"
  echo ""
  echo "Commands:"
  echo "  up        Build, seed the dashboard, start the UI against a throwaway HOME"
  echo "  shell     Enter a shell wired to that HOME ('ss' is on PATH)"
  echo "  down      Stop the verification UI"
  echo "  status    Show paths and whether the UI is running"
  echo "  reset     Stop the UI and delete the throwaway HOME"
  echo ""
  echo "Environment:"
  echo "  VERIFY_HOME   Throwaway HOME (default: <repo>/.verify-home)"
  echo "  VERIFY_PORT   Dashboard port (default: 19421)"
}

die() {
  echo -e "${RED}Error:${NC} $*" >&2
  exit 1
}

# A wrong VERIFY_HOME would point the whole run at real config, so refuse the
# values that would do damage before anything writes or deletes.
assert_safe_home() {
  [[ -n "$VERIFY_HOME" ]] || die "VERIFY_HOME is empty"
  [[ "$VERIFY_HOME" == /* ]] || die "VERIFY_HOME must be an absolute path: $VERIFY_HOME"
  [[ "$VERIFY_HOME" != "/" ]] || die "VERIFY_HOME must not be /"
  [[ "$VERIFY_HOME" != "$REAL_HOME" ]] || die "VERIFY_HOME must not be your real home: $REAL_HOME"
  [[ "$VERIFY_HOME" != "$PROJECT_ROOT" ]] || die "VERIFY_HOME must not be the repo root"
}

MARKER=".skillshare-verify-home"

# 'reset' runs rm -rf, so it only ever deletes a directory this script created.
assert_ours() {
  [[ -f "$VERIFY_HOME/$MARKER" ]] || die "$VERIFY_HOME was not created by this script (no $MARKER); refusing to delete it"
}

# Start from an empty environment so a stray SKILLSHARE_CONFIG or OPENCODE_CONFIG
# in the caller's shell cannot pull the run back to real paths. Every variable
# either skillshare or an Agent CLI reads for a location is set explicitly.
run_in_env() {
  env -i \
    PATH="$VERIFY_HOME/bin:$PATH" \
    HOME="$VERIFY_HOME" \
    USER="${USER:-}" \
    SHELL="${SHELL:-/bin/bash}" \
    TERM="${TERM:-xterm-256color}" \
    LANG="${LANG:-en_US.UTF-8}" \
    SSH_AUTH_SOCK="${SSH_AUTH_SOCK:-}" \
    XDG_CONFIG_HOME="$VERIFY_HOME/.config" \
    XDG_DATA_HOME="$VERIFY_HOME/.local/share" \
    XDG_STATE_HOME="$VERIFY_HOME/.local/state" \
    XDG_CACHE_HOME="$VERIFY_HOME/.cache" \
    CLAUDE_CONFIG_DIR="$VERIFY_HOME/.claude" \
    CODEX_HOME="$VERIFY_HOME/.codex" \
    GROK_HOME="$VERIFY_HOME/.grok" \
    COPILOT_HOME="$VERIFY_HOME/.copilot" \
    PI_CODING_AGENT_DIR="$VERIFY_HOME/.pi" \
    OPENCODE_CONFIG_DIR="$VERIFY_HOME/.config/opencode" \
    SKILLSHARE_VERIFY=1 \
    "$@"
}

devc_running() {
  local cid
  cid="$(docker compose -f "$COMPOSE_FILE" ps -q "$SERVICE" 2>/dev/null || true)"
  [[ -n "$cid" ]]
}

ui_url() {
  echo "http://127.0.0.1:$VERIFY_PORT"
}

ui_running() {
  curl -fsS -o /dev/null --max-time 1 "$(ui_url)/api/health" 2>/dev/null
}

cmd_up() {
  assert_safe_home
  command -v go >/dev/null 2>&1 || die "go not found on PATH"

  mkdir -p "$VERIFY_HOME"
  touch "$VERIFY_HOME/$MARKER"

  echo -e "${YELLOW}Building host binary...${NC}"
  (cd "$PROJECT_ROOT" && go build -o "$BIN" ./cmd/skillshare)

  # The frontend toolchain lives in the devcontainer; building it on the host
  # hits missing native modules and a different Node.
  echo -e "${YELLOW}Building dashboard in the devcontainer...${NC}"
  require_docker
  devc_running || die "devcontainer is not running. Start it with: make devc-up"
  docker compose -f "$COMPOSE_FILE" exec -T "$SERVICE" \
    bash -lc 'cd /workspace/ui && pnpm run build'

  echo -e "${YELLOW}Seeding the dashboard cache...${NC}"
  local cache="$VERIFY_HOME/.cache/skillshare/ui/dev"
  rm -rf "$cache"
  mkdir -p "$cache"
  cp -R "$PROJECT_ROOT/ui/dist/." "$cache/"

  mkdir -p "$VERIFY_HOME/bin"
  ln -sf "$BIN" "$VERIFY_HOME/bin/ss"
  ln -sf "$BIN" "$VERIFY_HOME/bin/skillshare"

  if [[ ! -f "$VERIFY_HOME/.config/skillshare/config.yaml" ]]; then
    echo -e "${YELLOW}Initialising the throwaway config...${NC}"
    mkdir -p "$VERIFY_HOME/.config/skillshare/skills"
    (cd "$VERIFY_HOME" && run_in_env "$BIN" init)
  fi

  if ui_running; then
    echo -e "${YELLOW}Restarting the dashboard to pick up the new build...${NC}"
    (cd "$VERIFY_HOME" && run_in_env "$BIN" ui stop --port "$VERIFY_PORT") >/dev/null 2>&1 || true
  fi
  (cd "$VERIFY_HOME" && run_in_env "$BIN" ui start --port "$VERIFY_PORT")

  echo ""
  echo -e "${GREEN}Verification environment ready.${NC}"
  echo "  Dashboard : $(ui_url)"
  echo "  HOME      : $VERIFY_HOME"
  echo "  CLI shell : make verify-shell"
}

cmd_shell() {
  assert_safe_home
  [[ -x "$BIN" ]] || die "nothing built yet. Run: make verify"
  echo -e "${GREEN}Entering the verification environment.${NC}"
  echo "  HOME is $VERIFY_HOME and 'ss' is on PATH. Your real config is untouched."
  echo "  Try: ss mcp list -g   |   ss plugin list -g   |   exit"
  echo ""
  cd "$VERIFY_HOME"
  run_in_env "${SHELL:-/bin/bash}" -i
}

cmd_down() {
  assert_safe_home
  [[ -x "$BIN" ]] || die "nothing built yet. Run: make verify"
  (cd "$VERIFY_HOME" && run_in_env "$BIN" ui stop --port "$VERIFY_PORT")
}

cmd_status() {
  assert_safe_home
  echo "Throwaway HOME : $VERIFY_HOME"
  echo "Binary         : $BIN"
  if [[ -x "$BIN" ]]; then
    echo "Version        : $("$BIN" --version 2>/dev/null || echo unknown)"
  else
    echo "Version        : not built"
  fi
  if [[ -f "$VERIFY_HOME/.cache/skillshare/ui/dev/index.html" ]]; then
    echo "Dashboard      : cached from ui/dist"
  else
    echo "Dashboard      : not seeded (run: make verify)"
  fi
  if ui_running; then
    echo -e "UI             : ${GREEN}running${NC} at $(ui_url)"
  else
    echo "UI             : stopped"
  fi
  echo ""
  echo "Real paths (must stay untouched by this environment):"
  echo "  $REAL_HOME/.config/skillshare"
  echo "  $REAL_HOME/.claude"
  echo "  $REAL_HOME/.codex"
}

cmd_reset() {
  assert_safe_home
  assert_ours
  if [[ -x "$BIN" ]] && ui_running; then
    (cd "$VERIFY_HOME" && run_in_env "$BIN" ui stop --port "$VERIFY_PORT") >/dev/null 2>&1 || true
  fi
  rm -rf "$VERIFY_HOME"
  echo -e "${GREEN}Removed${NC} $VERIFY_HOME"
}

if [[ $# -eq 0 ]]; then
  usage
  exit 1
fi

CMD="$1"
shift

case "$CMD" in
  up) cmd_up "$@" ;;
  shell) cmd_shell "$@" ;;
  down) cmd_down "$@" ;;
  status) cmd_status "$@" ;;
  reset) cmd_reset "$@" ;;
  -h | --help | help) usage ;;
  *)
    echo "Unknown command: $CMD" >&2
    echo ""
    usage
    exit 1
    ;;
esac
