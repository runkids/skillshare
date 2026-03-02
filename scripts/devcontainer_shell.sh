#!/usr/bin/env bash
# Enter the running devcontainer or execute a command inside it.
# Usage:
#   ./scripts/devcontainer_shell.sh          # interactive shell
#   ./scripts/devcontainer_shell.sh make test # run a command
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Reuse require_docker from sandbox common, then override COMPOSE_FILE.
source "$SCRIPT_DIR/_sandbox_common.sh"
require_docker

COMPOSE_FILE="$PROJECT_ROOT/.devcontainer/docker-compose.yml"
SERVICE="skillshare-devcontainer"

if [[ -z "$(docker compose -f "$COMPOSE_FILE" ps -q "$SERVICE" 2>/dev/null || true)" ]]; then
  echo "Devcontainer is not running." >&2
  echo "Start it with one of:" >&2
  echo "  1. VS Code → 'Reopen in Container'" >&2
  echo "  2. docker compose -f .devcontainer/docker-compose.yml up -d" >&2
  exit 1
fi

if [[ $# -gt 0 ]]; then
  docker compose -f "$COMPOSE_FILE" exec -w /workspace "$SERVICE" bash -c "$*"
else
  docker compose -f "$COMPOSE_FILE" exec -w /workspace "$SERVICE" bash
fi
