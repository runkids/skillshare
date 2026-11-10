# CLI E2E Runbook: Hermes MCP

## Scope

Verify Hermes YAML destinations, OAuth options, portable environment references,
SSE refusal, preview, conflict protection, backups and managed removal. No server
is launched and no real credentials or OAuth login are used.

## Environment

Run inside the devcontainer with mdproof's per-runbook HOME isolation. Each step
creates its own temporary config and Hermes home. Use the freshly built `ss`.

## Steps

### Step 1: Sync native OAuth settings without touching Hermes credentials

```bash
set -eu
MCP_CASE=$(mktemp -d "$HOME/hermes-oauth.XXXXXX")
export SKILLSHARE_CONFIG="$MCP_CASE/config.yaml"
export HERMES_HOME="$MCP_CASE/hermes"
export CLAUDE_CONFIG_DIR="$MCP_CASE/claude"
export CODEX_HOME="$MCP_CASE/codex"
mkdir -p "$HERMES_HOME/mcp-tokens"
printf 'API_KEY=hermes-only-test-value\n' > "$HERMES_HOME/.env"
printf '{"access_token":"hermes-only-test-token"}\n' > "$HERMES_HOME/mcp-tokens/linear.json"
cp "$HERMES_HOME/.env" "$MCP_CASE/env.before"
cp "$HERMES_HOME/mcp-tokens/linear.json" "$MCP_CASE/token.before"
cat > "$SKILLSHARE_CONFIG" <<'YAML'
targets: {}
mcp:
  servers:
    linear:
      url: https://mcp.linear.app/mcp
      targets: [claude, codex, hermes]
      options:
        hermes:
          auth: oauth
YAML
ss sync mcp --dry-run --json -g | jq -e '.blocked == false and (.changes | length) == 3' >/dev/null
test ! -f "$HERMES_HOME/config.yaml"
ss sync mcp -g >/dev/null
grep -q 'auth: oauth' "$HERMES_HOME/config.yaml"
grep -q 'https://mcp.linear.app/mcp' "$CLAUDE_CONFIG_DIR/.claude.json"
grep -q 'https://mcp.linear.app/mcp' "$CODEX_HOME/config.toml"
cmp "$HERMES_HOME/.env" "$MCP_CASE/env.before"
cmp "$HERMES_HOME/mcp-tokens/linear.json" "$MCP_CASE/token.before"
! grep -q 'hermes-only-test' "$SKILLSHARE_CONFIG"
ss mcp list --json -g
```

Expected:
- exit_code: 0
- jq: .blocked == false
- jq: [.changes[].action] | all(. == "unchanged")

### Step 2: Import environment references and reject SSE

```bash
set -eu
MCP_CASE=$(mktemp -d "$HOME/hermes-import.XXXXXX")
export SKILLSHARE_CONFIG="$MCP_CASE/config.yaml"
export HERMES_HOME="$MCP_CASE/hermes"
mkdir -p "$HERMES_HOME"
printf 'targets: {}\n' > "$SKILLSHARE_CONFIG"
cat > "$HERMES_HOME/config.yaml" <<'YAML'
mcp_servers:
  local:
    command: example-mcp
    env:
      API_KEY: '${env:EXAMPLE_KEY}'
  legacy:
    url: https://example.com/sse
    transport: sse
YAML
ss mcp import local --from hermes --target hermes -g >/dev/null
grep -q 'fromEnv: EXAMPLE_KEY' "$SKILLSHARE_CONFIG"
ss mcp import --from hermes --json -g
```

Expected:
- exit_code: 1
- jq: .[] | select(.name == "local") | .problems == [] and .server.env.API_KEY.fromEnv == "EXAMPLE_KEY"
- jq: .[] | select(.name == "legacy") | (.problems | length) > 0

### Step 3: Preserve unmanaged settings through conflict, backup and removal

```bash
set -eu
MCP_CASE=$(mktemp -d "$HOME/hermes-lifecycle.XXXXXX")
export SKILLSHARE_CONFIG="$MCP_CASE/config.yaml"
export HERMES_HOME="$MCP_CASE/hermes"
mkdir -p "$HERMES_HOME"
printf 'targets: {}\n' > "$SKILLSHARE_CONFIG"
printf '# personal settings\nmodel: example\nmcp_servers:\n  personal:\n    command: mine\n' > "$HERMES_HOME/config.yaml"
ss mcp add docs --url https://example.com/mcp --target hermes -g >/dev/null
ss sync mcp --json -g > "$MCP_CASE/applied.json"
cp "$HERMES_HOME/config.yaml" "$MCP_CASE/synced"
ss sync mcp -g >/dev/null
cmp "$HERMES_HOME/config.yaml" "$MCP_CASE/synced"
sed -i 's,https://example.com/mcp,https://changed.example/mcp,' "$HERMES_HOME/config.yaml"
if ss sync mcp -g > "$MCP_CASE/conflict" 2>&1; then exit 1; fi
grep -q 'https://changed.example/mcp' "$HERMES_HOME/config.yaml"
cp "$MCP_CASE/synced" "$HERMES_HOME/config.yaml"
BACKUP_ID=$(jq -er '.backupIds[0]' "$MCP_CASE/applied.json")
ss mcp restore "$BACKUP_ID" --dry-run --json -g | jq -e '.blocked == false' >/dev/null
ss mcp remove docs --sync -g >/dev/null
grep -q '# personal settings' "$HERMES_HOME/config.yaml"
grep -q 'model: example' "$HERMES_HOME/config.yaml"
grep -q 'command: mine' "$HERMES_HOME/config.yaml"
! grep -q 'https://example.com/mcp' "$HERMES_HOME/config.yaml"
ss mcp list --json -g
```

Expected:
- exit_code: 0
- jq: .blocked == false and .changes == []

## Pass Criteria

- All three steps pass.
- Only named managed entries change; unrelated YAML and Hermes credential files survive.
- OAuth settings remain Hermes-specific, references remain references, and SSE cannot import as HTTP.
