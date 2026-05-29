#!/usr/bin/env bash
# Manage devcontainer dev servers (API, Vite, Docusaurus).
# Usage: dev-servers {start|stop|restart|status|logs} [name]
set -euo pipefail

PIDDIR=/tmp/dev-servers
LOGDIR=/tmp
mkdir -p "$PIDDIR"

if [ "${BASH_VERSINFO[0]}" -lt 4 ]; then
  echo "dev-servers requires bash >= 4 (associative arrays unsupported)." >&2
  exit 1
fi

PNPM_BIN=""
if command -v pnpm >/dev/null 2>&1; then
  PNPM_BIN="$(command -v pnpm)"
elif [ -x /usr/local/share/pnpm/pnpm ]; then
  PNPM_BIN="/usr/local/share/pnpm/pnpm"
elif [ -x /usr/local/bin/pnpm ]; then
  PNPM_BIN="/usr/local/bin/pnpm"
elif [ -x /usr/local/share/nvm/current/bin/pnpm ]; then
  PNPM_BIN="/usr/local/share/nvm/current/bin/pnpm"
fi

declare -A PORTS=([api]=19420 [vite]=5173 [docusaurus]=8888)
declare -A DIRS=([api]=/workspace [vite]=/workspace/ui [docusaurus]=/workspace/website)
declare -A CMDS=(
  [api]="/usr/local/go/bin/go run ./cmd/skillshare ui --no-open --host 0.0.0.0 --port 19420"
  [vite]="${PNPM_BIN:-pnpm} exec vite --host 0.0.0.0 --port 5173 --strictPort"
  [docusaurus]="${PNPM_BIN:-pnpm} exec docusaurus start --host 0.0.0.0 --port 8888 --no-open"
)
declare -A VERIFY=([api]="skillshare" [vite]="vite" [docusaurus]="docusaurus")
ALL=(vite docusaurus)

is_running() {
  local n="$1" pf="$PIDDIR/$1.pid"
  [ -f "$pf" ] || return 1
  local pid
  pid=$(cat "$pf")
  kill -0 "$pid" 2>/dev/null || { rm -f "$pf"; return 1; }
  # Guard against PID reuse: verify cmdline contains expected keyword
  if [ -r "/proc/$pid/cmdline" ]; then
    tr '\0' ' ' < "/proc/$pid/cmdline" | grep -qi "${VERIFY[$n]}" || { rm -f "$pf"; return 1; }
  fi
}

is_port_open() {
  local port="$1"
  timeout 1 bash -c "echo > /dev/tcp/127.0.0.1/$port" >/dev/null 2>&1
}

wait_for_port() {
  local port="$1"
  # Wait up to ~60s (webpack/docusaurus first boot can be slow).
  for _ in $(seq 1 120); do
    if is_port_open "$port"; then
      return 0
    fi
    sleep 0.5
  done
  return 1
}

ensure_api_initialized() {
  local cfg="$HOME/.config/skillshare/config.yaml"
  if [ -f "$cfg" ]; then
    return 0
  fi
  (
    cd /workspace
    /usr/local/go/bin/go run ./cmd/skillshare init -g --no-copy --all-targets --no-git --skill >/tmp/api-init.log 2>&1
  )
}

ensure_frontend_deps() {
  local n="$1"
  local dir="${DIRS[$n]}"
  local bin=""
  local dep_log="$LOGDIR/${n}-deps.log"
  case "$n" in
    vite) bin="$dir/node_modules/.bin/vite" ;;
    docusaurus) bin="$dir/node_modules/.bin/docusaurus" ;;
    *) return 0 ;;
  esac

  if [ -x "$bin" ]; then
    return 0
  fi

  (
    cd "$dir"
    "$PNPM_BIN" install --frozen-lockfile >"$dep_log" 2>&1
  )
}

print_failure_tail() {
  local n="$1"
  local log="$LOGDIR/${n}-dev.log"
  if [ -f "$log" ]; then
    echo "    Last log lines ($log):" >&2
    tail -n 20 "$log" | sed 's/^/      /' >&2
  fi
}

validate_target() {
  local n="$1"
  if [ -z "${PORTS[$n]+x}" ]; then
    echo "Unknown server: $n (available: ${ALL[*]})" >&2
    exit 1
  fi
}

do_start() {
  local n=$1
  if { [ "$n" = "vite" ] || [ "$n" = "docusaurus" ]; } && [ -z "$PNPM_BIN" ]; then
    printf "  %-12s failed: pnpm not found in PATH\n" "$n" >&2
    return 1
  fi
  if [ "$n" = "api" ] && ! ensure_api_initialized; then
    printf "  %-12s bootstrap failed (see /tmp/api-init.log)\n" "$n" >&2
    if [ -f /tmp/api-init.log ]; then
      tail -n 20 /tmp/api-init.log | sed 's/^/      /' >&2
    fi
    return 1
  fi
  if { [ "$n" = "vite" ] || [ "$n" = "docusaurus" ]; } && ! ensure_frontend_deps "$n"; then
    printf "  %-12s dependency bootstrap failed (see %s/%s-deps.log)\n" "$n" "$LOGDIR" "$n" >&2
    if [ -f "$LOGDIR/${n}-deps.log" ]; then
      tail -n 20 "$LOGDIR/${n}-deps.log" | sed 's/^/      /' >&2
    fi
    return 1
  fi
  if ! is_running "$n" && is_port_open "${PORTS[$n]}"; then
    printf "  %-12s port %s already in use (unmanaged process)\n" "$n" "${PORTS[$n]}"
    return 0
  fi
  if is_running "$n"; then
    if is_port_open "${PORTS[$n]}"; then
      printf "  %-12s already running (port %s)\n" "$n" "${PORTS[$n]}"
      return
    fi
    printf "  %-12s running (PID %s) but port %s is not reachable; restarting\n" "$n" "$(cat "$PIDDIR/${n}.pid")" "${PORTS[$n]}"
    do_stop "$n"
  fi
  (
    cd "${DIRS[$n]}"
    nohup bash -lc "${CMDS[$n]}" > "$LOGDIR/${n}-dev.log" 2>&1 &
    echo $! > "$PIDDIR/${n}.pid"
  )
  if is_running "$n" && wait_for_port "${PORTS[$n]}"; then
    printf "  %-12s started → http://localhost:%s\n" "$n" "${PORTS[$n]}"
    return
  fi
  if is_running "$n"; then
    kill "$(cat "$PIDDIR/${n}.pid")" 2>/dev/null || true
  fi
  rm -f "$PIDDIR/${n}.pid"
  printf "  %-12s failed to start (see %s/%s-dev.log)\n" "$n" "$LOGDIR" "$n" >&2
  print_failure_tail "$n"
  return 1
}

do_stop() {
  local n=$1 pf="$PIDDIR/$n.pid"
  if ! is_running "$n"; then
    printf "  %-12s not running\n" "$n"
    rm -f "$pf"
    return
  fi
  local pid; pid=$(cat "$pf")
  # Kill children first (go run → compiled binary), then wrapper bash.
  pkill -P "$pid" 2>/dev/null || true
  kill "$pid" 2>/dev/null || true
  for _ in 1 2 3 4 5; do
    kill -0 "$pid" 2>/dev/null || break
    sleep 0.2
  done
  pkill -9 -P "$pid" 2>/dev/null || true
  kill -9 "$pid" 2>/dev/null || true
  # Last resort: free the port directly.
  fuser -k "${PORTS[$n]}/tcp" 2>/dev/null || true
  rm -f "$pf"
  printf "  %-12s stopped\n" "$n"
}

do_status() {
  local n=$1
  if is_running "$n"; then
    if is_port_open "${PORTS[$n]}"; then
      printf "  %-12s ✓ running  port %-5s  PID %s\n" "$n" "${PORTS[$n]}" "$(cat "$PIDDIR/$n.pid")"
    else
      printf "  %-12s ! running  port %-5s unreachable  PID %s\n" "$n" "${PORTS[$n]}" "$(cat "$PIDDIR/$n.pid")"
    fi
  elif is_port_open "${PORTS[$n]}"; then
    printf "  %-12s ~ listening port %-5s (unmanaged)\n" "$n" "${PORTS[$n]}"
  else
    printf "  %-12s ✗ stopped\n" "$n"
    rm -f "$PIDDIR/$n.pid"
  fi
}

targets() {
  if [ -n "${1:-}" ]; then
    validate_target "$1"
    echo "$1"
  else
    echo "${ALL[@]}"
  fi
}

start_targets() {
  local failed=0
  local t="$1"
  for n in $(targets "$t"); do
    if ! do_start "$n"; then
      failed=1
    fi
  done
  return $failed
}

stop_targets() {
  local t="$1"
  for n in $(targets "$t"); do
    do_stop "$n"
  done
}

case "${1:-help}" in
  start)   start_targets "${2:-}" ;;
  stop)    stop_targets "${2:-}" ;;
  restart)
    stop_targets "${2:-}"
    sleep 1
    start_targets "${2:-}"
    ;;
  status)  for n in $(targets "${2:-}"); do do_status "$n"; done ;;
  logs)
    [ -z "${2:-}" ] && echo "Usage: dev-servers logs {api|vite|docusaurus}" >&2 && exit 1
    validate_target "${2}"
    tail -f "$LOGDIR/${2}-dev.log"
    ;;
  *)
    echo "Usage: dev-servers {start|stop|restart|status|logs} [api|vite|docusaurus]"
    echo ""
    echo "Commands:"
    echo "  start [name]    Start all or one server"
    echo "  stop [name]     Stop all or one server"
    echo "  restart [name]  Restart all or one server"
    echo "  status [name]   Show running state"
    echo "  logs <name>     Tail server log"
    ;;
esac
