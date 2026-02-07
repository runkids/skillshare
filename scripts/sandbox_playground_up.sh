#!/usr/bin/env bash
# Start a persistent Docker playground for interactive skillshare usage.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
COMPOSE_FILE="$PROJECT_ROOT/docker-compose.sandbox.yml"
SERVICE="sandbox-playground"

if ! command -v docker >/dev/null 2>&1; then
  echo "Error: docker command not found" >&2
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "Error: docker compose plugin not available" >&2
  exit 1
fi

cd "$PROJECT_ROOT"

docker compose -f "$COMPOSE_FILE" --profile playground build "$SERVICE"

# Prepare shared volumes for host UID/GID access.
docker compose -f "$COMPOSE_FILE" --profile playground run --rm --user "0:0" "$SERVICE" \
  bash -c "mkdir -p /go/pkg/mod /go/build-cache /sandbox-home /tmp && chmod -R 0777 /go/pkg/mod /go/build-cache /sandbox-home /tmp"

docker compose -f "$COMPOSE_FILE" --profile playground up -d "$SERVICE"

# Copy pre-built frontend assets for go:embed, then build skillshare binary.
docker compose -f "$COMPOSE_FILE" --profile playground exec --user "$(id -u):$(id -g)" "$SERVICE" \
  bash -c "rm -rf internal/server/dist && cp -r /ui-dist internal/server/dist && mkdir -p /sandbox-home/.local/bin && go build -o /sandbox-home/.local/bin/skillshare ./cmd/skillshare && ln -sf /sandbox-home/.local/bin/skillshare /sandbox-home/.local/bin/ss && touch /sandbox-home/.bashrc && { grep -qxF \"alias ss='skillshare'\" /sandbox-home/.bashrc || echo \"alias ss='skillshare'\" >> /sandbox-home/.bashrc; }"

echo "Playground is running."
echo "Enter it with: ./scripts/sandbox_playground_shell.sh"
echo "Inside playground you can directly run: skillshare  (and alias: ss)"
echo "  skillshare ui --no-open    # start web dashboard (port 19420)"
