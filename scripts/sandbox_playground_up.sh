#!/usr/bin/env bash
# Start a persistent Docker playground for interactive skillshare usage.
set -euo pipefail

source "$(dirname "$0")/_sandbox_common.sh"
SERVICE="sandbox-playground"

require_docker
cd "$PROJECT_ROOT"

docker compose -f "$COMPOSE_FILE" --profile playground build "$SERVICE"

# Prepare shared volumes for host UID/GID access.
docker compose -f "$COMPOSE_FILE" --profile playground run --rm --user "0:0" "$SERVICE" \
  bash -c "mkdir -p /go/pkg/mod /go/build-cache /sandbox-home /tmp && chmod -R 0777 /go/pkg/mod /go/build-cache /sandbox-home /tmp"

docker compose -f "$COMPOSE_FILE" --profile playground up -d "$SERVICE"

# Build skillshare binary and set up aliases.
docker compose -f "$COMPOSE_FILE" --profile playground exec --user "$(id -u):$(id -g)" "$SERVICE" \
  bash -c '
    mkdir -p /sandbox-home/.local/bin
    go build -o /sandbox-home/.local/bin/skillshare ./cmd/skillshare
    ln -sf /sandbox-home/.local/bin/skillshare /sandbox-home/.local/bin/ss
    touch /sandbox-home/.bashrc
    grep -qxF "alias ss='"'"'skillshare'"'"'" /sandbox-home/.bashrc || echo "alias ss='"'"'skillshare'"'"'" >> /sandbox-home/.bashrc
    grep -qxF "alias skillshare-ui='"'"'skillshare ui --host 0.0.0.0 --no-open'"'"'" /sandbox-home/.bashrc || echo "alias skillshare-ui='"'"'skillshare ui --host 0.0.0.0 --no-open'"'"'" >> /sandbox-home/.bashrc
  '

echo "Playground is running."
echo "Enter it with: ./scripts/sandbox_playground_shell.sh or run make sandbox-shell"
echo "Inside playground you can directly run: skillshare  (and alias: ss)"
echo ""
echo "Quick start:"
echo "  skillshare init         # required before first use"
echo "  skillshare-ui           # start web dashboard (port 19420)"
