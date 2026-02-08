#!/usr/bin/env bash
# Shared boilerplate for Docker sandbox scripts.
# Source this file: source "$(dirname "$0")/_sandbox_common.sh"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$PROJECT_ROOT/docker-compose.sandbox.yml"

require_docker() {
  if ! command -v docker >/dev/null 2>&1; then
    echo "Error: docker command not found" >&2
    exit 1
  fi
  if ! docker compose version >/dev/null 2>&1; then
    echo "Error: docker compose plugin not available" >&2
    exit 1
  fi
}
