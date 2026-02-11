#!/usr/bin/env bash
# Stop and remove the Docker playground container.
# Usage: ./sandbox_playground_down.sh [--volumes]
#   --volumes  Also remove the playground-home volume (full reset)
set -euo pipefail

source "$(dirname "$0")/_sandbox_common.sh"

REMOVE_VOLUMES=false
for arg in "$@"; do
  case "$arg" in
    --volumes|-v) REMOVE_VOLUMES=true ;;
  esac
done

require_docker
cd "$PROJECT_ROOT"
docker compose -f "$COMPOSE_FILE" --profile playground stop sandbox-playground
docker compose -f "$COMPOSE_FILE" --profile playground rm -f sandbox-playground

if [ "$REMOVE_VOLUMES" = true ]; then
  echo "Removing playground-home volume..."
  docker volume rm skillshare_playground-home 2>/dev/null && echo "Volume removed." || echo "Volume not found (already removed)."
fi
