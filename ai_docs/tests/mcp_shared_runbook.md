# CLI E2E Runbook: Shared MCP Servers

Verify a server defined once with `global: false` loads only in the project roots that list it under `use`.

**Origin**: issue #976 — define an MCP server once and assign it to chosen projects.

## Scope

- A shared server is written to the Agent files of the roots that use it, and nowhere else.
- `mcp list --json` names the projects that use each shared server.
- Editing the server once updates every root that uses it, and the edit keeps `global: false`.
- Sync refuses a `use` name that is not a shared server, and `global: false` in a project's own config.
- Removing the server takes it out of every `use` list and every root's Agent file.

No server is launched and all endpoints are inert example URLs.

## Environment

Run inside the devcontainer with a fresh ssenv HOME. Each step creates its own config directory under the isolated HOME. Use the newly built `ss` binary. Do not run these steps against a real Agent configuration.

## Steps

### Step 1: A shared server reaches only the roots that use it

```bash
set -eu
C=$(mktemp -d "$HOME/mcp-shared.XXXXXX")
export SKILLSHARE_CONFIG="$C/config.yaml"
mkdir -p "$C/api" "$C/blog"
cat > "$SKILLSHARE_CONFIG" <<EOF
targets: {}
mcp:
  targets: [claude]
  servers:
    docs: {url: https://example.com/docs, global: false}
  projects:
    $C/api: {targets: [claude], use: [docs]}
    $C/blog: {targets: [claude]}
EOF
ss sync mcp -g >/dev/null
jq -e '.mcpServers.docs.url == "https://example.com/docs"' "$C/api/.mcp.json" >/dev/null
if [ -e "$C/blog/.mcp.json" ] && jq -e '.mcpServers.docs' "$C/blog/.mcp.json" >/dev/null; then exit 1; fi
if [ -e "$HOME/.claude.json" ] && jq -e '.mcpServers.docs' "$HOME/.claude.json" >/dev/null; then exit 1; fi
ss mcp list --json -g
```

Expected:
- exit_code: 0
- jq: .blocked == false
- jq: .shared[0].name == "docs"
- jq: (.shared[0].projects | length) == 1
- jq: .shared[0].projects[0] | endswith("/api")

### Step 2: One edit updates every root that uses it

```bash
set -eu
C=$(mktemp -d "$HOME/mcp-shared-edit.XXXXXX")
export SKILLSHARE_CONFIG="$C/config.yaml"
mkdir -p "$C/api" "$C/web"
cat > "$SKILLSHARE_CONFIG" <<EOF
targets: {}
mcp:
  targets: [claude]
  servers:
    docs: {url: https://example.com/docs, global: false}
  projects:
    $C/api: {targets: [claude], use: [docs]}
    $C/web: {targets: [claude], use: [docs]}
EOF
ss sync mcp -g >/dev/null
ss mcp edit docs --url https://example.com/docs-v2 --sync -g >/dev/null
grep -q 'global: false' "$SKILLSHARE_CONFIG"
for root in api web; do
  jq -e '.mcpServers.docs.url == "https://example.com/docs-v2"' "$C/$root/.mcp.json" >/dev/null
done
ss mcp list --json -g
```

Expected:
- exit_code: 0
- jq: .blocked == false
- jq: (.shared[0].projects | length) == 2

### Step 3: Sync refuses a use name that is not shared, and global: false in a project

```bash
set -eu
C=$(mktemp -d "$HOME/mcp-shared-rules.XXXXXX")
export SKILLSHARE_CONFIG="$C/config.yaml"
mkdir -p "$C/api"
cat > "$SKILLSHARE_CONFIG" <<EOF
targets: {}
mcp:
  targets: [claude]
  servers:
    everywhere: {url: https://example.com/everywhere}
  projects:
    $C/api: {targets: [claude], use: [everywhere]}
EOF
if ss sync mcp -g > "$C/global.err" 2>&1; then exit 1; fi
grep -q 'everywhere' "$C/global.err"
P="$C/proj"
mkdir -p "$P/.skillshare"
printf 'targets: {}\nmcp:\n  targets: [claude]\n  servers:\n    docs: {url: https://example.com/docs, global: false}\n' > "$P/.skillshare/config.yaml"
unset SKILLSHARE_CONFIG
cd "$P"
if ss sync mcp -p > "$C/project.err" 2>&1; then exit 1; fi
cat "$C/project.err"
```

Expected:
- exit_code: 0
- global

### Step 4: Removing a shared server clears every use

```bash
set -eu
C=$(mktemp -d "$HOME/mcp-shared-remove.XXXXXX")
export SKILLSHARE_CONFIG="$C/config.yaml"
mkdir -p "$C/api" "$C/web"
cat > "$SKILLSHARE_CONFIG" <<EOF
targets: {}
mcp:
  targets: [claude]
  servers:
    docs: {url: https://example.com/docs, global: false}
  projects:
    $C/api: {targets: [claude], use: [docs]}
    $C/web: {targets: [claude], use: [docs]}
EOF
ss sync mcp -g >/dev/null
ss mcp remove docs --sync -g >/dev/null
if grep -q 'docs' "$SKILLSHARE_CONFIG"; then exit 1; fi
for root in api web; do
  if [ -e "$C/$root/.mcp.json" ] && jq -e '.mcpServers.docs' "$C/$root/.mcp.json" >/dev/null; then exit 1; fi
done
ss mcp list --json -g
```

Expected:
- exit_code: 0
- jq: .blocked == false
- jq: has("shared") | not

## Pass Criteria

- All steps marked PASS.
- No Agent file outside the roots that use a shared server ever holds it.
