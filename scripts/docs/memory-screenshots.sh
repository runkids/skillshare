#!/usr/bin/env bash
# Serves the isolated demo homes behind the Memory screenshots in
# website/docs/how-to/daily-tasks/sharing-memory.md. Run inside the devcontainer:
#
#   scripts/docs/memory-screenshots.sh onboarding   # fresh home, memory not created yet
#   scripts/docs/memory-screenshots.sh docs         # notes, shared AGENTS.md, connected guidance
#   scripts/docs/memory-screenshots.sh stop         # hand :49420 back to air
#
# Each mode rebuilds its home from scratch and serves it on :49420, behind the
# Vite dev server on :45173. Shoot in a fresh en-US browser context with
# ?theme=clean at 1512x771.
set -euo pipefail

BIN=/tmp/ss-shots
PORT=49420
API="http://localhost:$PORT/api"

stop_api() {
  # -x matches the process name only, so this shell is never a match.
  pkill -x skillshare || true
  pkill -x ss-shots || true
  for _ in $(seq 50); do
    curl -s -o /dev/null "$API/health" || return 0
    sleep 0.2
  done
  echo "port $PORT is still in use" >&2
  exit 1
}

serve() {
  local home=$1
  stop_api
  rm -rf "$home"
  mkdir -p "$home/.claude" "$home/.codex"
  (cd /workspace && go build -o "$BIN" ./cmd/skillshare)
  # Run from the home so the repository's .skillshare/ is never picked up as a project.
  cd "$home"
  HOME="$home" "$BIN" init -g --no-copy --no-git --no-skill --targets claude,codex >/dev/null
  HOME="$home" nohup "$BIN" ui -g --no-open --host 0.0.0.0 --port "$PORT" >/tmp/ss-shots.log 2>&1 &
  for _ in $(seq 100); do
    curl -sf -o /dev/null "$API/health" && return 0
    sleep 0.2
  done
  echo "demo API did not start; see /tmp/ss-shots.log" >&2
  exit 1
}

call() {
  local method=$1 path=$2 body=${3:-}
  curl -sf -X "$method" -H 'Content-Type: application/json' ${body:+-d "$body"} "$API$path"
}

note() {
  local path=$1 content=$2
  call PUT /extras/memory/notes/content "$(jq -n --arg p "$path" --arg c "$content" '{path: $p, content: $c, version: ""}')" >/dev/null
  local index
  index=$(call GET /extras/memory/notes | jq -r '.index.version')
  call PUT /extras/memory/index "$(jq -n --arg p "$path" --arg v "$index" '{path: $p, version: $v}')" >/dev/null
}

docs() {
  serve /tmp/skillshare-memory-docs
  call POST /extras/memory/init >/dev/null
  call POST /instructions "$(jq -n --arg c $'# Project conventions\n\nUse English identifiers and verify changes before reporting completion.\n' '{name: "shared-memory", content: $c}')" >/dev/null
  call POST /instructions/assign '{"targets":["claude","codex"],"extras":["shared-memory"]}' >/dev/null
  call PUT /instructions/shared-memory/targets/claude/mode '{"mode":"import"}' >/dev/null
  call PUT /instructions/shared-memory/targets/codex/mode '{"mode":"symlink"}' >/dev/null
  local token
  token=$(call POST /extras/memory/guidance/plan '{"targets":["claude","codex"]}' | jq -r '.token')
  call POST /extras/memory/guidance/apply "$(jq -n --arg t "$token" '{targets: ["claude","codex"], token: $t}')" >/dev/null
  note wiki/architecture.md $'# Architecture decisions\n\n## Shared memory\n\nClaude and Codex read the same Markdown notes from Skillshare.\nKeep durable decisions here and verify facts that may have changed.\n\n## Retrieval\n\nRead INDEX.md first, then only the notes relevant to the current task.\nUpdate notes when the user asks you to remember a decision.\n'
  note wiki/workflow-check.md $'# Workflow check\n\nRun the tests that cover a change before reporting it as done.\n'
}

case ${1:-} in
  onboarding) serve /tmp/skillshare-memory-redesign-onboarding ;;
  docs) docs ;;
  stop)
    pkill -x ss-shots || true
    # air restarts the dev API on a write under cmd/ (touch alone is ignored);
    # writing the same bytes back leaves no diff.
    f=/workspace/cmd/skillshare/main.go
    c=$(cat "$f"; printf x)
    printf '%s' "${c%x}" > "$f"
    ;;
  *) sed -n '2,11p' "$0" >&2; exit 2 ;;
esac
