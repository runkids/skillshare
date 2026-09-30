# CLI E2E Runbook: Portable MCP Configuration

## Scope

Verify inline and external declarations, global/project destinations, preview,
idempotence, removal, import/adoption, conflict protection, entry restoration,
Pi built-in MCP and its 0.22 migration, and the per-Agent tool policy.
No server is launched and all endpoints are inert example URLs.

## Environment

Run inside the devcontainer with a fresh ssenv HOME. Each step creates its own
config directory under the isolated HOME. Use the newly built `ss` binary.
Do not run these steps against a real Agent configuration.

## Steps

### Step 1: Inline lifecycle with preview and removal

```bash
set -eu
MCP_CASE=$(mktemp -d "$HOME/mcp-inline.XXXXXX")
export SKILLSHARE_CONFIG="$MCP_CASE/config.yaml"
printf 'targets: {}\n' > "$SKILLSHARE_CONFIG"
ss mcp add e2e-inline --url https://example.com/mcp --target claude -g >/dev/null
ss sync mcp --dry-run --json -g | jq -e '.changes[0].action == "add"' >/dev/null
ss sync mcp --json -g > "$MCP_CASE/applied.json"
jq -e '.applied | length == 1' "$MCP_CASE/applied.json" >/dev/null
ss sync mcp --json -g | jq -e '.applied == [] and .plan.changes[0].action == "unchanged"' >/dev/null
ss mcp remove e2e-inline --sync -g >/dev/null
ss mcp list --json -g
```

Expected:
- exit_code: 0
- jq: .changes == []
- jq: .blocked == false

### Step 2: External source edits and missing-file protection

```bash
set -eu
MCP_CASE=$(mktemp -d "$HOME/mcp-external.XXXXXX")
export SKILLSHARE_CONFIG="$MCP_CASE/config.yaml"
printf 'sources:\n  mcp: ./mcp.yaml\nmcp:\n  targets: [claude]\n' > "$SKILLSHARE_CONFIG"
printf 'servers: {}\n' > "$MCP_CASE/mcp.yaml"
cp "$SKILLSHARE_CONFIG" "$MCP_CASE/config.before"
ss mcp add e2e-external --url https://example.com/external --sync -g >/dev/null
cmp "$SKILLSHARE_CONFIG" "$MCP_CASE/config.before"
cp "$HOME/.claude.json" "$MCP_CASE/native.before"
mv "$MCP_CASE/mcp.yaml" "$MCP_CASE/mcp.saved"
if ss sync mcp -g > "$MCP_CASE/error" 2>&1; then exit 1; fi
cmp "$HOME/.claude.json" "$MCP_CASE/native.before"
mv "$MCP_CASE/mcp.saved" "$MCP_CASE/mcp.yaml"
ss mcp list --json -g
```

Expected:
- exit_code: 0
- jq: .blocked == false
- jq: .changes[0].action == "unchanged"

### Step 3: Import refusal, explicit replace, save-only and later drift conflict

```bash
set -eu
MCP_CASE=$(mktemp -d "$HOME/mcp-import.XXXXXX")
export SKILLSHARE_CONFIG="$MCP_CASE/config.yaml"
export CLAUDE_CONFIG_DIR="$MCP_CASE/client"
mkdir -p "$CLAUDE_CONFIG_DIR"
printf 'targets: {}\n' > "$SKILLSHARE_CONFIG"
printf '{"mcpServers":{"imported":{"command":"example-mcp","env":{"API_TOKEN":"literal-secret"}}},"personal":true}\n' > "$CLAUDE_CONFIG_DIR/.claude.json"
cp "$CLAUDE_CONFIG_DIR/.claude.json" "$MCP_CASE/before"
if ss mcp import imported --from claude --target claude -g > "$MCP_CASE/refused" 2>&1; then exit 1; fi
cmp "$CLAUDE_CONFIG_DIR/.claude.json" "$MCP_CASE/before"
ss mcp import imported --from claude --target claude --replace -g >/dev/null
cmp "$CLAUDE_CONFIG_DIR/.claude.json" "$MCP_CASE/before"
ss sync mcp -g >/dev/null
sed -i 's/example-mcp/changed-mcp/' "$CLAUDE_CONFIG_DIR/.claude.json"
if ss sync mcp -g > "$MCP_CASE/error" 2>&1; then exit 1; fi
ss mcp list --json -g
```

Expected:
- exit_code: 0
- jq: .blocked == true
- jq: .changes[0].action == "conflict"

### Step 4: Project native destinations and restore

```bash
set -eu
MCP_CASE=$(mktemp -d "$HOME/mcp-project.XXXXXX")
mkdir -p "$MCP_CASE/.skillshare"
printf 'targets: []\n' > "$MCP_CASE/.skillshare/config.yaml"
cd "$MCP_CASE"
ss mcp add project-docs --url https://example.com/project --target claude --target codex --sync -p >/dev/null
test -f .mcp.json
test -f .codex/config.toml
ss mcp remove project-docs --sync --json -p > removal.json
MCP_BACKUP=$(jq -r '.backupIds[0]' removal.json)
ss mcp restore "$MCP_BACKUP" --dry-run --json -p | jq -e '.blocked == false' >/dev/null
ss mcp restore "$MCP_BACKUP" --json -p
```

Expected:
- exit_code: 0
- jq: .applied | length == 1
- jq: .plan.changes[0].action == "restore"

### Step 5: OpenCode, Grok and Antigravity project sync

```bash
set -eu
MCP_CASE=$(mktemp -d "$HOME/mcp-extra-clients.XXXXXX")
mkdir -p "$MCP_CASE/.skillshare"
printf 'targets: []\n' > "$MCP_CASE/.skillshare/config.yaml"
cd "$MCP_CASE"
printf '{\n// keep\n"model":"example"\n}\n' > opencode.jsonc
ss mcp add company-docs --url https://example.com/mcp --target opencode --target grok --target antigravity --sync -p >/dev/null
test ! -f opencode.json
grep -q '// keep' opencode.jsonc
grep -q 'remote' opencode.jsonc
grep -q 'mcp_servers.company-docs' .grok/config.toml
jq -e '.mcpServers["company-docs"].serverUrl == "https://example.com/mcp"' .agents/mcp_config.json >/dev/null
ss mcp remove company-docs --sync --json -p > removal.json
MCP_BACKUP=$(jq -r '.backupIds[0]' removal.json)
ss mcp restore "$MCP_BACKUP" --json -p
```

Expected:
- exit_code: 0
- jq: .applied | length == 1
- jq: .plan.changes[0].action == "restore"

### Step 6: Agent-specific fields survive and a pulled change converges

```bash
set -eu
MCP_CASE=$(mktemp -d "$HOME/mcp-converge.XXXXXX")
export SKILLSHARE_CONFIG="$MCP_CASE/config.yaml"
export CODEX_HOME="$MCP_CASE/codex"
mkdir -p "$CODEX_HOME"
printf 'targets: {}\n' > "$SKILLSHARE_CONFIG"
printf '[mcp_servers.docs]\nurl = "https://example.com/mcp"\nstartup_timeout_sec = 30\n' > "$CODEX_HOME/config.toml"
ss mcp import docs --from codex --target codex --sync -g >/dev/null
ss mcp add docs --url https://changed.example/mcp --target codex --replace --sync -g >/dev/null
grep -q 'startup_timeout_sec = 30' "$CODEX_HOME/config.toml"
sed -i 's/changed.example/pulled.example/' "$SKILLSHARE_CONFIG" "$CODEX_HOME/config.toml"
ss mcp list --json -g
```

Expected:
- exit_code: 0
- jq: .blocked == false
- jq: .changes[0].action == "unchanged"

### Step 7: Expanded project clients and Goose YAML import

```bash
set -eu
MCP_CASE=$(mktemp -d "$HOME/mcp-expanded.XXXXXX")
mkdir -p "$MCP_CASE/.skillshare"
printf 'targets: []\n' > "$MCP_CASE/.skillshare/config.yaml"
cd "$MCP_CASE"
ss mcp add browser --target amp --target copilot --target factory --target gemini --target junie --target kiro --target warp --sync -p -- npx -y @playwright/mcp@latest >/dev/null
jq -e '."amp.mcpServers".browser.command == "npx"' .amp/settings.json >/dev/null
jq -e '.mcpServers.browser.type == "local" and .mcpServers.browser.tools == ["*"]' .github/mcp.json >/dev/null
ss sync mcp --json -p | jq -e '.applied == [] and (.plan.changes | length == 7)' >/dev/null
ss mcp add remote --url https://example.com/mcp --target gemini --sync -p >/dev/null
jq -e '.mcpServers.remote.httpUrl == "https://example.com/mcp" and .mcpServers.remote.url == null' .gemini/settings.json >/dev/null
printf 'extensions:\n  imported:\n    type: stdio\n    name: imported\n    enabled: true\n    cmd: echo\n    args: [ready]\n' > goose.yaml
ss mcp import imported --from goose --file goose.yaml --target amp --sync -p >/dev/null
jq -e '."amp.mcpServers".imported.command == "echo"' .amp/settings.json >/dev/null
ss mcp list --json -p
```

Expected:
- exit_code: 0
- jq: .blocked == false
- jq: .changes | length == 9

### Step 8: Scripted editing and TUI opt-out

```bash
set -eu
MCP_CASE=$(mktemp -d "$HOME/mcp-edit.XXXXXX")
export SKILLSHARE_CONFIG="$MCP_CASE/config.yaml"
printf 'targets: {}\n' > "$SKILLSHARE_CONFIG"
ss mcp add editable --url https://example.com/mcp --target claude --no-tui -g >/dev/null
ss mcp edit editable --url https://updated.example/mcp --no-tui -g >/dev/null
grep -q 'updated.example' "$SKILLSHARE_CONFIG"
ss mcp edit editable --url https://preview.example/mcp --dry-run --json -g >/dev/null
! grep -q 'preview.example' "$SKILLSHARE_CONFIG"
ss mcp edit editable --no-tui -g -- echo ready >/dev/null
! grep -q 'url:' "$SKILLSHARE_CONFIG"
ss mcp --no-tui -g >/dev/null
ss mcp --json -g
```

Expected:
- exit_code: 0
- jq: .blocked == false
- jq: .changes[0].name == "editable"

### Step 9: Pi built-in MCP and cleared options

```bash
set -eu
mkdir -p "$HOME/.config/skillshare/skills"
MCP_CASE=$(mktemp -d "$HOME/mcp-pi-builtin.XXXXXX")
export SKILLSHARE_CONFIG="$MCP_CASE/config.yaml"
export PI_CODING_AGENT_DIR="$MCP_CASE/pi"
printf 'targets: {}\n' > "$SKILLSHARE_CONFIG"
ss mcp add docs --target pi --url https://example.com/mcp \
  --pi-options '{"exposure":"deferred","timeout":120,"custom":{"flag":true}}' --no-tui -g >/dev/null
ss sync mcp -g >/dev/null
jq -e '.mcpServers.docs.exposure == "deferred" and .mcpServers.docs.transport == null' "$PI_CODING_AGENT_DIR/mcp.json" >/dev/null
test ! -e "$PI_CODING_AGENT_DIR/mcp-adapter.json"
ss mcp edit docs --pi-options '{"custom":{"flag":true}}' --no-tui -g >/dev/null
ss sync mcp -g >/dev/null
jq -e '.mcpServers.docs.exposure == null and .mcpServers.docs.timeout == null and .mcpServers.docs.custom.flag == true' "$PI_CODING_AGENT_DIR/mcp.json" >/dev/null
ss sync mcp --dry-run --json -g
```

Expected:
- exit_code: 0
- jq: .blocked == false
- jq: .changes[0].action == "unchanged"

### Step 10: A 0.22 Pi config converts on the first sync

The owned `mcp-adapter.json` entry of a real 0.22 sync needs a 0.22 ledger; Go tests
(`TestPiAdapterServerMovesToBuiltin`, `TestMCPPiAdapterServerSyncsToBuiltin`) cover its
removal. This step covers the config, the notices, the backup and a hand-written
adapter entry.

```bash
set -eu
mkdir -p "$HOME/.config/skillshare/skills"
MCP_CASE=$(mktemp -d "$HOME/mcp-pi-migrate.XXXXXX")
export SKILLSHARE_CONFIG="$MCP_CASE/config.yaml"
export PI_CODING_AGENT_DIR="$MCP_CASE/pi"
mkdir -p "$PI_CODING_AGENT_DIR"
printf '{"mcpServers":{"mine":{"command":"my-mcp"}}}\n' > "$PI_CODING_AGENT_DIR/mcp-adapter.json"
cp "$PI_CODING_AGENT_DIR/mcp-adapter.json" "$MCP_CASE/adapter.before"
cat > "$SKILLSHARE_CONFIG" <<'YAML'
targets: {}
mcp:
  directTools: search
  servers:
    docs:
      url: https://example.com/mcp
      targets: [pi]
      piExtension: pi-mcp-adapter
      piOptionsPrune: true
      piOptions:
        lifecycle: lazy
        excludeTools: [delete_repo]
    lister:
      command: lister-mcp
      targets: [pi]
      piExtension: builtin
      directTools: [list_items]
YAML
cp "$SKILLSHARE_CONFIG" "$MCP_CASE/config.before"
ss sync mcp --dry-run --json -g > "$MCP_CASE/plan.json"
jq -e '.migrates == true and (.notices | length == 4)' "$MCP_CASE/plan.json" >/dev/null
jq -e '.notices[0] == "Pi now uses its built-in MCP; the next sync updates the config: docs, lister"' "$MCP_CASE/plan.json" >/dev/null
cmp "$SKILLSHARE_CONFIG" "$MCP_CASE/config.before"
ss sync mcp -g > "$MCP_CASE/sync.out"
grep -q 'Updated config.yaml for 0.23.0 (backup: ' "$MCP_CASE/sync.out"
! grep -Eq 'piExtension|piOptionsPrune|directTools|lifecycle|excludeTools' "$SKILLSHARE_CONFIG"
jq -e '.mcpServers.docs.exposure == "deferred" and .mcpServers.docs.toolExposure.delete_repo == "hidden"' "$PI_CODING_AGENT_DIR/mcp.json" >/dev/null
jq -e '.mcpServers.lister.toolExposure.list_items == "direct"' "$PI_CODING_AGENT_DIR/mcp.json" >/dev/null
cmp "$PI_CODING_AGENT_DIR/mcp-adapter.json" "$MCP_CASE/adapter.before"
ss backup files show "$SKILLSHARE_CONFIG" -g | grep -q 'history/migrate'
ss mcp import --from pi --json -g | jq -e 'any(.[]; .name == "mine")' >/dev/null
ss sync mcp --dry-run --json -g
```

Expected:
- exit_code: 0
- jq: .notices == null
- jq: .migrates == null
- jq: .changes | length == 2

### Step 11: Removed Pi flags explain the change

```bash
set -eu
mkdir -p "$HOME/.config/skillshare/skills"
MCP_CASE=$(mktemp -d "$HOME/mcp-pi-flags.XXXXXX")
export SKILLSHARE_CONFIG="$MCP_CASE/config.yaml"
export PI_CODING_AGENT_DIR="$MCP_CASE/pi"
printf 'targets: {}\n' > "$SKILLSHARE_CONFIG"
ss mcp add docs --url https://example.com/mcp --target pi --no-tui -g >/dev/null
for flag in '--pi-extension builtin' '--pi-options-prune' '--direct-tools true'; do
  if ss mcp edit docs $flag --no-tui -g > "$MCP_CASE/out" 2>&1; then exit 1; fi
  grep -q 'was removed in 0.23.0' "$MCP_CASE/out"
done
grep -q 'tools-expose direct' "$MCP_CASE/out"
ss mcp list --json -g
```

Expected:
- exit_code: 0
- jq: .blocked == false

### Step 12: Tool policy per Agent

```bash
set -eu
mkdir -p "$HOME/.config/skillshare/skills"
unset SKILLSHARE_CONFIG PI_CODING_AGENT_DIR
MCP_CASE=$(mktemp -d "$HOME/mcp-tools.XXXXXX")
mkdir -p "$MCP_CASE/.skillshare"
printf 'targets: []\n' > "$MCP_CASE/.skillshare/config.yaml"
cd "$MCP_CASE"
ss mcp add github --target pi --target codex --target copilot --target opencode \
  --tools-expose deferred --tools-allow 'get_*,search_code' --tools-deny get_secret --no-tui -p -- github-mcp >/dev/null
ss sync mcp --dry-run --json -p > plan.json
jq -e '.notices == ["tool policy not applied for codex: expose, allow patterns (github)", "tool policy not applied for copilot: expose, allow patterns, deny (github)", "tool policy not applied for opencode: expose, allow, deny (github)"]' plan.json >/dev/null
ss sync mcp -p >/dev/null
jq -e '.mcpServers.github.exposure == "deferred" and (.mcpServers.github.toolExposure | keys_unsorted) == ["get_secret", "get_*", "search_code", "*"]' .pi/mcp.json >/dev/null
grep -q "disabled_tools = \['get_secret'\]" .codex/config.toml
! grep -q enabled_tools .codex/config.toml
jq -e '.mcpServers.github.tools == ["*"]' .github/mcp.json >/dev/null
jq -e '.mcp.github | has("tools") | not' opencode.json >/dev/null
# github-mcp is not installed, so check exits 1 for the missing command.
ss mcp check github --json --no-dns -p > check.json || test $? -eq 1
jq -e '[.servers[].findings[] | select(.check == "tools") | .target] == ["codex", "copilot", "opencode"]' check.json >/dev/null
ss mcp edit github --tools-expose '' --tools-allow 'search_code,list_issues' --tools-deny '' --no-tui -p >/dev/null
ss sync mcp -p >/dev/null
grep -q "enabled_tools = \['search_code', 'list_issues'\]" .codex/config.toml
jq -e '.mcpServers.github.tools == ["search_code", "list_issues"]' .github/mcp.json >/dev/null
ss mcp edit github --tools-allow '' --no-tui -p >/dev/null
ss sync mcp -p >/dev/null
! grep -q _tools .codex/config.toml
if ss mcp edit github --tools-allow x --tools-deny x --no-tui -p > err 2>&1; then exit 1; fi
grep -q 'tools.deny removes every tool tools.allow keeps' err
printf '[mcp_servers.imp]\ncommand = "imp-mcp"\nenabled_tools = ["a"]\n' > imp.toml
ss mcp import imp --file imp.toml --from codex --target codex --no-tui -p >/dev/null
grep -A6 '  imp:' .skillshare/config.yaml | grep -q -- '- a'
ss sync mcp --dry-run --json -p
```

Expected:
- exit_code: 0
- jq: .blocked == false
- jq: .notices == null

## Pass Criteria

All twelve steps pass. Core Go tests additionally cover multi-file recovery,
stale revisions, Agent-specific fields, credential references and foreign ownership.
TUI model tests cover search shortcuts, hidden credentials, edit cancellation and
batch import cancellation. In a real terminal, also verify `mcp` search/detail,
edit and remove previews, multi-selection import, and the client/backup picker.
