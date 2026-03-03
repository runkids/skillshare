#!/usr/bin/env bash
# Unified devcontainer lifecycle script (no VS Code required).
# Usage: ./scripts/devc.sh <command>
#
# Commands:
#   up        Start devcontainer (build if needed, run setup on first start)
#   shell     Enter running devcontainer shell
#   down      Stop and remove devcontainer
#   restart   Restart devcontainer
#   reset     Stop + remove volumes (full reset)
#   status    Show devcontainer status
#   logs      Tail devcontainer logs
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

source "$SCRIPT_DIR/_sandbox_common.sh"

COMPOSE_FILE="$PROJECT_ROOT/.devcontainer/docker-compose.yml"
SERVICE="skillshare-devcontainer"

usage() {
  echo "Usage: $(basename "$0") <command>"
  echo ""
  echo "Commands:"
  echo "  up        Start devcontainer (build + init on first run)"
  echo "  shell     Enter running devcontainer shell"
  echo "  down      Stop and remove devcontainer"
  echo "  restart   Restart devcontainer"
  echo "  reset     Stop + remove volumes (full reset)"
  echo "  status    Show devcontainer status"
  echo "  logs      Tail devcontainer logs"
}

# Check if container is running.
is_running() {
  local cid
  cid="$(docker compose -f "$COMPOSE_FILE" ps -q "$SERVICE" 2>/dev/null || true)"
  [[ -n "$cid" ]]
}

# Check if one-time data setup has completed (sentinel on persistent volume).
is_initialised() {
  docker compose -f "$COMPOSE_FILE" exec -T "$SERVICE" \
    test -f /home/developer/.devcontainer-initialized 2>/dev/null
}

cmd_up() {
  require_docker
  cd "$PROJECT_ROOT"

  echo "▸ Starting devcontainer …"
  docker compose -f "$COMPOSE_FILE" up -d --build

  if is_initialised; then
    echo "▸ Already initialised — running start-dev.sh …"
    docker compose -f "$COMPOSE_FILE" exec -T -w /workspace "$SERVICE" \
      bash -c '/workspace/.devcontainer/start-dev.sh'
  else
    echo "▸ First run — running setup.sh …"
    docker compose -f "$COMPOSE_FILE" exec -T -w /workspace "$SERVICE" \
      bash -c '/workspace/.devcontainer/setup.sh'
  fi
}

cmd_shell() {
  require_docker
  cd "$PROJECT_ROOT"

  if ! is_running; then
    echo "Devcontainer is not running." >&2
    echo "Start it with:  make devc-up  (or ./scripts/devc.sh up)" >&2
    exit 1
  fi

  docker compose -f "$COMPOSE_FILE" exec -w /workspace "$SERVICE" bash -l
}

cmd_down() {
  require_docker
  cd "$PROJECT_ROOT"
  docker compose -f "$COMPOSE_FILE" down
}

cmd_restart() {
  require_docker
  cd "$PROJECT_ROOT"

  docker compose -f "$COMPOSE_FILE" restart

  echo "▸ Running start-dev.sh …"
  docker compose -f "$COMPOSE_FILE" exec -T -w /workspace "$SERVICE" \
    bash -c '/workspace/.devcontainer/start-dev.sh'
}

cmd_reset() {
  require_docker
  cd "$PROJECT_ROOT"
  docker compose -f "$COMPOSE_FILE" down -v
  echo "Volumes removed. Run 'make devc' to re-initialise."
}

cmd_status() {
  require_docker
  cd "$PROJECT_ROOT"
  docker compose -f "$COMPOSE_FILE" ps
}

cmd_logs() {
  require_docker
  cd "$PROJECT_ROOT"
  docker compose -f "$COMPOSE_FILE" logs -f "$SERVICE"
}

if [[ $# -eq 0 ]]; then
  usage
  exit 1
fi

CMD="$1"
shift

case "$CMD" in
  up)       cmd_up "$@" ;;
  shell)    cmd_shell "$@" ;;
  down)     cmd_down "$@" ;;
  restart)  cmd_restart "$@" ;;
  reset)    cmd_reset "$@" ;;
  status)   cmd_status "$@" ;;
  logs)     cmd_logs "$@" ;;
  help|--help|-h)
    usage
    ;;
  *)
    echo "Error: unknown command '$CMD'" >&2
    echo ""
    usage
    exit 1
    ;;
esac
