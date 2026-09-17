# CLI E2E Runbook: Portable MCP Configuration

## Scope

Verify inline and external declarations, global/project destinations, preview,
idempotence, removal, import/adoption, conflict protection and entry restoration.
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

### Step 5: OpenCode and Grok project sync

```bash
set -eu
MCP_CASE=$(mktemp -d "$HOME/mcp-extra-clients.XXXXXX")
mkdir -p "$MCP_CASE/.skillshare"
printf 'targets: []\n' > "$MCP_CASE/.skillshare/config.yaml"
cd "$MCP_CASE"
printf '{\n// keep\n"model":"example"\n}\n' > opencode.jsonc
ss mcp add company-docs --url https://example.com/mcp --target opencode --target grok --sync -p >/dev/null
test ! -f opencode.json
grep -q '// keep' opencode.jsonc
grep -q 'remote' opencode.jsonc
grep -q 'mcp_servers.company-docs' .grok/config.toml
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

## Pass Criteria

All six steps pass. Core Go tests additionally cover multi-file recovery,
stale revisions, Agent-specific fields, credential references and foreign ownership.
