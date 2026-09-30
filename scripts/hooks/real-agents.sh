#!/usr/bin/env bash
# Prove synced hooks fire in real Agents: ai_docs/tests/hooks_real_agents_runbook.md.
# Run only inside the devcontainer through a fresh ssenv HOME. Each hook only writes
# a marker file. Agents run with a throwaway HOME and fake credentials, so they start
# a session, run their hooks and then fail at the model call; no login is used.
#
#   real-agents.sh install                        Agent CLIs into $HOOKS_AGENTS_DIR
#   real-agents.sh project <target> [agent-dir]   project scope in /workspace/tmp/hooks-e2e-<target>
#   real-agents.sh global <target>                global scope in /tmp/hooks-e2e-home-<target>
#   real-agents.sh fire <target> project|global   run the Agent once; prints its markers as JSON
#   real-agents.sh clean                          remove fixtures, homes and markers
#
# <agent-dir> is the project directory as the Agent will see it (a host path when the
# Agent runs on the host), so markers land in the fixture no matter the hook's cwd.
set -euo pipefail

test -f /workspace/go.mod
test -n "${SSENV_ACTIVE:-}"
command -v jq >/dev/null
BIN=${HOOKS_BINARY:-/workspace/bin/skillshare}
AGENTS=${HOOKS_AGENTS_DIR:-/tmp/hooks-agents}
unset CLAUDE_CONFIG_DIR CODEX_HOME COPILOT_HOME PI_CODING_AGENT_DIR SKILLSHARE_CONFIG

# entry prints the Entry JSON for one target; markers are written under $2.
entry() {
  local t=$1 dir=$2
  mark() { printf 'echo fired > "%s/.hook-fired-%s"' "$dir" "$1"; }
  code() {
    printf 'import { writeFileSync } from "node:fs";\nconst mark = (e) => writeFileSync(%s + "/.hook-fired-" + e, "fired\\n");\n%s\n' "$(jq -Rn --arg d "$dir" '$d')" "$1"
  }
  case "$t" in
    claude | codex | qwen | droid)
      jq -n --arg t "$t" --arg a "$(mark SessionStart)" --arg b "$(mark UserPromptSubmit)" \
        '{bindings:{($t):{events:{SessionStart:[{hooks:[{type:"command",command:$a}]}],UserPromptSubmit:[{hooks:[{type:"command",command:$b}]}]}}}}' ;;
    gemini)
      jq -n --arg a "$(mark SessionStart)" --arg b "$(mark BeforeAgent)" \
        '{bindings:{gemini:{events:{SessionStart:[{hooks:[{type:"command",command:$a,timeout:60000}]}],BeforeAgent:[{hooks:[{type:"command",command:$b,timeout:60000}]}]}}}}' ;;
    copilot)
      jq -n --arg a "$(mark sessionStart)" --arg b "$(mark userPromptSubmitted)" \
        '{bindings:{copilot:{events:{sessionStart:[{type:"command",bash:$a,timeoutSec:30}],userPromptSubmitted:[{type:"command",bash:$b,timeoutSec:30}]}}}}' ;;
    cursor)
      jq -n --arg a "$(mark sessionStart)" --arg b "$(mark beforeSubmitPrompt)" \
        '{bindings:{cursor:{events:{sessionStart:[{command:$a}],beforeSubmitPrompt:[{command:$b}]}}}}' ;;
    antigravity)
      jq -n --arg a "$(mark PreInvocation)" --arg b "$(mark Stop)" \
        '{bindings:{antigravity:{events:{PreInvocation:[{hooks:[{type:"command",command:$a,timeout:10}]}],Stop:[{hooks:[{type:"command",command:$b,timeout:10}]}]}}}}' ;;
    pi)
      jq -n --arg c "$(code 'export default function (pi) {
  mark("load");
  pi.on("session_start", async () => mark("session_start"));
}')" '{bindings:{pi:{code:$c}}}' ;;
    amp)
      jq -n --arg c "$(code 'export default function (amp) {
  mark("load");
  amp.on("session.start", async () => mark("session.start"));
}')" '{bindings:{amp:{code:$c}}}' ;;
    opencode)
      jq -n --arg c "$(code 'export const SkillshareE2E = async () => {
  mark("load");
  return { event: async ({ event }) => { if (event.type === "session.created") mark("session.created"); } };
}')" '{bindings:{opencode:{code:$c}}}' ;;
    *) printf 'unknown target %s\n' "$t" >&2; return 2 ;;
  esac
}

# isolated runs a command with HOME and XDG directories under $1.
isolated() {
  local h=$1; shift
  env HOME="$h" XDG_CONFIG_HOME="$h/.config" XDG_DATA_HOME="$h/.data" XDG_STATE_HOME="$h/.state" XDG_CACHE_HOME="$h/.cache" "$@"
}

case "${1:-}" in
  install)
    mkdir -p "$AGENTS/home"
    cd "$AGENTS"
    [ -f package.json ] || npm init -y >/dev/null
    npm i --no-fund --no-audit @github/copilot @google/gemini-cli @qwen-code/qwen-code opencode-ai @sourcegraph/amp @earendil-works/pi-coding-agent >/dev/null 2>&1
    [ -x home/.local/bin/droid ] || HOME="$AGENTS/home" bash -c 'curl -fsSL https://app.factory.ai/cli | sh' >/dev/null 2>&1
    for c in claude codex; do printf '%s %s\n' "$c" "$($c --version 2>&1 | head -1)"; done
    for c in copilot gemini qwen opencode amp pi; do printf '%s %s\n' "$c" "$(node_modules/.bin/$c --version 2>&1 | head -1)"; done
    printf 'droid %s\n' "$(HOME="$AGENTS/home" home/.local/bin/droid --version 2>&1 | head -1)"
    ;;
  project)
    t=$2 p=/workspace/tmp/hooks-e2e-$2 s=/tmp/hooks-e2e-state-$2
    dir=${3:-$p}
    rm -rf "$p" "$s"
    mkdir -p "$p/.skillshare" "$s"
    # Its own Git root: Codex and Copilot resolve project layers from the repository root.
    git -C "$p" init -q
    printf 'targets: []\n' > "$p/.skillshare/config.yaml"
    entry "$t" "$dir" > "$p/.skillshare/entry.json"
    cd "$p"
    # Fresh Skillshare state: a ledger from an earlier run would see the deleted
    # fixture as an outside removal and block the sync.
    isolated "$s" "$BIN" hooks add e2e --file .skillshare/entry.json --sync --json -p | jq -ce '{applied} | select(.applied | length > 0)'
    ;;
  global)
    t=$2 h=/tmp/hooks-e2e-home-$2 m=/tmp/hooks-e2e-global-$2
    rm -rf "$h" "$m"
    mkdir -p "$h" "$m"
    entry "$t" "$m" > "$h/entry.json"
    printf 'targets: {}\n' > "$h/skillshare.yaml"
    isolated "$h" env SKILLSHARE_CONFIG="$h/skillshare.yaml" "$BIN" hooks add e2e --file "$h/entry.json" --sync --json -g | jq -e '.applied | length > 0' >/dev/null
    path=$(isolated "$h" env SKILLSHARE_CONFIG="$h/skillshare.yaml" "$BIN" hooks list --json -g | jq -r --arg t "$t" '.paths[$t]')
    if [ -d "$path" ]; then path=$(ls -d "$path"/skillshare-e2e.*); fi
    printf '== %s\n' "$path"
    cat "$path"
    ;;
  fire)
    t=$2 scope=$3
    if [ "$scope" = global ]; then
      h=/tmp/hooks-e2e-home-$t m=/tmp/hooks-e2e-global-$t w=/tmp/hooks-e2e-cwd-$t
      rm -rf "$w" && mkdir -p "$w"
    else
      h=/tmp/hooks-e2e-run-$t m=/workspace/tmp/hooks-e2e-$t w=$m
      rm -rf "$h" && mkdir -p "$h"
    fi
    rm -f "$m"/.hook-fired-*
    cd "$w"
    nm=$AGENTS/node_modules/.bin
    run() { isolated "$h" timeout 60 "$@" </dev/null >"$h/agent.log" 2>&1 || true; }
    case "$t" in
      claude) run env ANTHROPIC_API_KEY=sk-ant-fake claude -p "reply ok" ;;
      codex)
        mkdir -p "$h/.codex"
        # Project layers load only in a trusted project; this throwaway CODEX_HOME holds that trust.
        [ "$scope" = global ] || printf '[projects."%s"]\ntrust_level = "trusted"\n' "$w" > "$h/.codex/config.toml"
        run env CODEX_HOME="$h/.codex" OPENAI_API_KEY=sk-fake codex exec --skip-git-repo-check --ephemeral --dangerously-bypass-hook-trust "reply ok" ;;
      gemini) run env GEMINI_API_KEY=fake GEMINI_CLI_TRUST_WORKSPACE=true "$nm/gemini" -p "reply ok" ;;
      qwen) run env OPENAI_API_KEY=fake OPENAI_BASE_URL=http://127.0.0.1:9/v1 OPENAI_MODEL=fake "$nm/qwen" -p "reply ok" --auth-type openai ;;
      copilot)
        mkdir -p "$h/.copilot"
        # Repository hooks load only in a trusted folder; this throwaway COPILOT_HOME holds that trust.
        [ "$scope" = global ] || jq -n --arg w "$w" '{trustedFolders:[$w]}' > "$h/.copilot/config.json"
        run env COPILOT_HOME="$h/.copilot" COPILOT_OFFLINE=true COPILOT_PROVIDER_BASE_URL=http://127.0.0.1:9/v1 COPILOT_MODEL=gpt-4o "$nm/copilot" -p "reply ok" ;;
      droid) run env FACTORY_API_KEY=fake "$AGENTS/home/.local/bin/droid" exec "reply ok" ;;
      opencode) run env OPENAI_API_KEY=fake "$nm/opencode" run "reply ok" ;;
      amp) run env AMP_API_KEY=fake "$nm/amp" -x "reply ok" ;;
      pi)
        approve=()
        [ "$scope" = global ] || approve=(--approve)
        run env GEMINI_API_KEY=fake "$nm/pi" -p --no-session "${approve[@]}" "reply ok" ;;
      *) printf '%s cannot run here without a login; see the runbook\n' "$t" >&2; exit 2 ;;
    esac
    ls -A "$m" | sed -n 's/^\.hook-fired-//p' | jq -R . | jq -sc --arg t "$t" --arg s "$scope" '{target:$t, scope:$s, markers:.}'
    ;;
  clean)
    rm -rf /workspace/tmp/hooks-e2e-* /tmp/hooks-e2e-*
    ;;
  *) printf 'Usage: real-agents.sh install | project <target> [agent-dir] | global <target> | fire <target> project|global | clean\n' >&2; exit 2 ;;
esac
