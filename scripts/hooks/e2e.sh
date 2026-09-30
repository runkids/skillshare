#!/usr/bin/env bash
# Run only inside the devcontainer through a fresh ssenv HOME. No hook executes.
set -euo pipefail

test -f /workspace/go.mod
test -n "${SSENV_ACTIVE:-}"
command -v jq >/dev/null
command -v grep >/dev/null
HOOKS_BINARY=${HOOKS_BINARY:-/workspace/bin/skillshare}
test -x "$HOOKS_BINARY"
HOOKS_CASE=$(mktemp -d "$HOME/hooks-e2e.XXXXXX")
export SKILLSHARE_CONFIG="$HOOKS_CASE/config.yaml"
unset CLAUDE_CONFIG_DIR CODEX_HOME COPILOT_HOME PI_CODING_AGENT_DIR
printf 'targets: {}\n' > "$SKILLSHARE_CONFIG"

case "${1:-}" in
  lifecycle)
    cat > "$HOOKS_CASE/entry.yaml" <<'EOF'
description: Inert configuration generation fixture
bindings:
  claude:
    events:
      Stop: [{hooks: [{type: command, command: "printf ready"}]}]
  codex:
    events:
      Stop: [{hooks: [{type: command, command: "printf ready"}]}]
  gemini:
    events:
      AfterAgent: [{hooks: [{type: command, command: "printf ready", timeout: 60000}]}]
  qwen:
    events:
      Stop: [{hooks: [{type: command, command: "printf ready"}]}]
  droid:
    events:
      Stop: [{hooks: [{type: command, command: "printf ready"}]}]
  cursor:
    events:
      stop: [{command: "printf ready"}]
  copilot:
    events:
      sessionEnd: [{type: command, bash: "printf ready", timeoutSec: 30}]
  pi:
    code: "export default function () {}"
  amp:
    code: "export default function () {}"
  opencode:
    code: "export default function () {}"
EOF
    "$HOOKS_BINARY" hooks add e2e --file "$HOOKS_CASE/entry.yaml" --dry-run --json -g > "$HOOKS_CASE/preview.json"
    ! grep -q 'e2e:' "$SKILLSHARE_CONFIG"
    "$HOOKS_BINARY" hooks add e2e --file "$HOOKS_CASE/entry.yaml" --sync --json -g > "$HOOKS_CASE/applied.json"
    jq -e '.applied | length == 10' "$HOOKS_CASE/applied.json" >/dev/null
    jq -e '.hooks.Stop | length == 1' "$HOME/.claude/settings.json" >/dev/null
    jq -e '.hooks.Stop | length == 1' "$HOME/.codex/hooks.json" >/dev/null
    jq -e '.Stop | length == 1' "$HOME/.factory/hooks.json" >/dev/null
    jq -e '.version == 1 and (.hooks.stop | length == 1)' "$HOME/.cursor/hooks.json" >/dev/null
    jq -e '.version == 1' "$HOME/.copilot/hooks/skillshare-e2e.json" >/dev/null
    for output in "$HOME/.pi/agent/extensions/skillshare-e2e.ts" "$XDG_CONFIG_HOME/amp/plugins/skillshare-e2e.ts" "$XDG_CONFIG_HOME/opencode/plugins/skillshare-e2e.ts"; do
      test "$(cat "$output")" = 'export default function () {}'
    done
    "$HOOKS_BINARY" hooks sync --json -g | jq -e '.applied | length == 0' >/dev/null
    "$HOOKS_BINARY" hooks disable e2e --sync --json -g > "$HOOKS_CASE/disabled.json"
    "$HOOKS_BINARY" hooks list --json -g | jq -e '.source.entries.e2e.enabled == false' >/dev/null
    test ! -e "$HOME/.pi/agent/extensions/skillshare-e2e.ts"
    "$HOOKS_BINARY" hooks enable e2e --sync -g >/dev/null
    "$HOOKS_BINARY" hooks remove e2e --sync --json -g > "$HOOKS_CASE/removed.json"
    HOOKS_BACKUP=$(jq -r '.backupIds[0]' "$HOOKS_CASE/removed.json")
    test "$HOOKS_BACKUP" != null
    "$HOOKS_BINARY" hooks restore "$HOOKS_BACKUP" --dry-run --json -g | jq -e '.blocked == false' >/dev/null
    "$HOOKS_BINARY" hooks restore "$HOOKS_BACKUP" --json -g | jq -e '.applied | length == 1' >/dev/null
    ;;
  preservation)
    mkdir -p "$HOME/.claude"
    cat > "$HOME/.claude/settings.json" <<'EOF'
{
  // Keep this comment and unrelated field.
  "personal": true,
  "hooks": {"Stop": [{"hooks": [{"type": "command", "command": "printf personal"}]}]}
}
EOF
    cat > "$HOOKS_CASE/entry.yaml" <<'EOF'
bindings:
  claude:
    events:
      Stop: [{hooks: [{type: command, command: "printf owned"}]}]
EOF
    "$HOOKS_BINARY" hooks add owned --file "$HOOKS_CASE/entry.yaml" --sync -g >/dev/null
    grep -q '// Keep this comment' "$HOME/.claude/settings.json"
    grep -q 'printf personal' "$HOME/.claude/settings.json"
    sed -i 's/printf owned/printf externally-edited/' "$HOME/.claude/settings.json"
    cp "$HOME/.claude/settings.json" "$HOOKS_CASE/before.json"
    if "$HOOKS_BINARY" hooks disable owned --sync -g > "$HOOKS_CASE/conflict.txt" 2>&1; then exit 1; fi
    cmp "$HOME/.claude/settings.json" "$HOOKS_CASE/before.json"
    "$HOOKS_BINARY" hooks disable owned -g >/dev/null
    cmp "$HOME/.claude/settings.json" "$HOOKS_CASE/before.json"
    "$HOOKS_BINARY" hooks list --json -g | jq -e '.source.entries.owned.enabled == false and .plan.blocked == true' >/dev/null
    ;;
  stale)
    printf 'bindings:\n  claude:\n    events:\n      Stop: [{hooks: [{type: command, command: "printf inert"}]}]\n' > "$HOOKS_CASE/entry.yaml"
    "$HOOKS_BINARY" hooks add stale --file "$HOOKS_CASE/entry.yaml" -g >/dev/null
    HOOKS_REVISION=$("$HOOKS_BINARY" hooks sync --dry-run --json -g | jq -r .revision)
    printf '\n# Changed after preview\n' >> "$SKILLSHARE_CONFIG"
    if "$HOOKS_BINARY" hooks sync --revision "$HOOKS_REVISION" -g > "$HOOKS_CASE/error.txt" 2>&1; then exit 1; fi
    test ! -e "$HOME/.claude/settings.json"
    ;;
  project)
    HOOKS_PROJECT="$HOOKS_CASE/project"
    mkdir -p "$HOOKS_PROJECT/.skillshare"
    printf 'targets: []\n' > "$HOOKS_PROJECT/.skillshare/config.yaml"
    printf 'bindings:\n  claude:\n    events:\n      Stop: [{hooks: [{type: command, command: "printf project"}]}]\n  pi:\n    code: "export default function () {}"\n' > "$HOOKS_CASE/entry.yaml"
    unset SKILLSHARE_CONFIG
    cd "$HOOKS_PROJECT"
    "$HOOKS_BINARY" hooks add local --file "$HOOKS_CASE/entry.yaml" --sync --json -p >/dev/null
    test -f .claude/settings.json
    test -f .pi/extensions/skillshare-local.ts
    test ! -e "$HOME/.claude/settings.json"
    test ! -e "$HOME/.pi/agent/extensions/skillshare-local.ts"
    "$HOOKS_BINARY" hooks import --from claude --file .claude/settings.json --json -p | jq -e 'length > 0' >/dev/null
    "$HOOKS_BINARY" hooks remove local --sync -p >/dev/null
    test ! -e .pi/extensions/skillshare-local.ts
    ;;
  *) printf 'Usage: bash scripts/hooks/e2e.sh lifecycle|preservation|stale|project\n' >&2; exit 2 ;;
esac
printf '{"case":"%s","passed":true}\n' "$1"
