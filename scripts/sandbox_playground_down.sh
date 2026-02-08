#!/usr/bin/env bash
# Stop and remove the Docker playground container.
set -euo pipefail

source "$(dirname "$0")/_sandbox_common.sh"

require_docker
cd "$PROJECT_ROOT"
docker compose -f "$COMPOSE_FILE" --profile playground stop sandbox-playground
docker compose -f "$COMPOSE_FILE" --profile playground rm -f sandbox-playground
