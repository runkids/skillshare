#!/usr/bin/env bash
# Run optional network-dependent install/update tests inside Docker.
set -euo pipefail

source "$(dirname "$0")/_sandbox_common.sh"
SERVICE="sandbox-online"

SKIP_BUILD=false
CUSTOM_CMD=""
DEFAULT_ONLINE_CMD="mkdir -p bin && go build -o bin/skillshare ./cmd/skillshare && SKILLSHARE_TEST_BINARY=/workspace/bin/skillshare go test -v -tags online ./tests/integration/... -run 'TestInstall_Remote'"

usage() {
  cat <<'EOF'
Usage: ./scripts/test_docker_online.sh [options]

Options:
  --skip-build         Skip docker compose build
  --cmd "<command>"    Override default online test command
  -h, --help           Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-build)
      SKIP_BUILD=true
      shift
      ;;
    --cmd)
      if [[ $# -lt 2 ]]; then
        echo "Error: --cmd requires a value" >&2
        exit 1
      fi
      CUSTOM_CMD="$2"
      shift 2
      ;;
    --cmd=*)
      CUSTOM_CMD="${1#--cmd=}"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Error: unknown option '$1'" >&2
      usage
      exit 1
      ;;
  esac
done

require_docker
cd "$PROJECT_ROOT"

if [[ "$SKIP_BUILD" != "true" ]]; then
  docker compose -f "$COMPOSE_FILE" --profile online build "$SERVICE"
fi

# Ensure named cache volumes are writable when running as host UID/GID.
docker compose -f "$COMPOSE_FILE" --profile online run --rm --user "0:0" "$SERVICE" bash -c "mkdir -p /go/pkg/mod /go/build-cache && chmod -R 0777 /go/pkg/mod /go/build-cache /tmp"

if [[ -n "$CUSTOM_CMD" ]]; then
  docker compose -f "$COMPOSE_FILE" --profile online run --rm --user "$(id -u):$(id -g)" "$SERVICE" bash -c "$CUSTOM_CMD"
else
  docker compose -f "$COMPOSE_FILE" --profile online run --rm --user "$(id -u):$(id -g)" "$SERVICE" bash -c "$DEFAULT_ONLINE_CMD"
fi
