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

declare -A PORTS=([api]=19420 [vite]=5173 [docusaurus]=3000)
declare -A DIRS=([api]=/workspace [vite]=/workspace/ui [docusaurus]=/workspace/website)
declare -A CMDS=(
  [api]="go run ./cmd/skillshare ui --no-open --host 0.0.0.0"
  [vite]="pnpm run dev"
  [docusaurus]="pnpm start --host 0.0.0.0"
)
ALL=(api vite docusaurus)

is_running() {
  local pf="$PIDDIR/$1.pid"
  [ -f "$pf" ] && kill -0 "$(cat "$pf")" 2>/dev/null
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
  if is_running "$n"; then
    printf "  %-12s already running (port %s)\n" "$n" "${PORTS[$n]}"
    return
  fi
  ( cd "${DIRS[$n]}" && nohup ${CMDS[$n]} > "$LOGDIR/${n}-dev.log" 2>&1 & echo $! > "$PIDDIR/${n}.pid" )
  sleep 0.3
  if is_running "$n"; then
    printf "  %-12s started → http://localhost:%s\n" "$n" "${PORTS[$n]}"
    return
  fi
  rm -f "$PIDDIR/${n}.pid"
  printf "  %-12s failed to start (see %s/%s-dev.log)\n" "$n" "$LOGDIR" "$n" >&2
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
  kill "$pid" 2>/dev/null || true
  for _ in 1 2 3 4 5; do
    kill -0 "$pid" 2>/dev/null || break
    sleep 0.2
  done
  kill -9 "$pid" 2>/dev/null || true
  rm -f "$pf"
  printf "  %-12s stopped\n" "$n"
}

do_status() {
  local n=$1
  if is_running "$n"; then
    printf "  %-12s ✓ running  port %-5s  PID %s\n" "$n" "${PORTS[$n]}" "$(cat "$PIDDIR/$n.pid")"
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

case "${1:-help}" in
  start)   for n in $(targets "${2:-}"); do do_start "$n"; done ;;
  stop)    for n in $(targets "${2:-}"); do do_stop "$n"; done ;;
  restart)
    for n in $(targets "${2:-}"); do do_stop "$n"; done
    sleep 1
    for n in $(targets "${2:-}"); do do_start "$n"; done
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
