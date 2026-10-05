# CLI E2E Runbook: Native Oh My Pi Support

## Scope

Validate issue #409's dry run, native OMP skills and MCP paths, global/project
sync, import, setting preservation, removal, backup restoration, project switches
and explicit account directories. No MCP server or OMP process is launched.

## Environment

Run in the devcontainer with mdproof's isolated HOME, or a fresh ssenv for manual
execution. Every step creates its own config and working directory. Never run
against real Agent configuration. Automatic named-profile routing is out of scope.

## Steps

### Step 1: Issue #409 dry run recognizes OMP without writing

```bash
set -eu
CASE=$(mktemp -d "$HOME/omp-dry.XXXXXX")
export SKILLSHARE_CONFIG="$CASE/config.yaml"
export PI_CODING_AGENT_DIR="$CASE/omp/agent"
printf 'targets: {}\n' > "$SKILLSHARE_CONFIG"
cp "$SKILLSHARE_CONFIG" "$CASE/before.yaml"
ss mcp add omp-probe --url https://example.invalid/mcp --target omp -g --dry-run --json > "$CASE/plan.json"
cmp "$SKILLSHARE_CONFIG" "$CASE/before.yaml"
test ! -e "$PI_CODING_AGENT_DIR/mcp.json"
jq . "$CASE/plan.json"
```

Expected:
- exit_code: 0
- jq: .blocked == false
- jq: .changes | length == 1
- jq: .changes[0].target == "omp" and .changes[0].action == "add" and .changes[0].name == "omp-probe"

### Step 2: Preserve native settings through update, restore and removal

```bash
set -eu
CASE=$(mktemp -d "$HOME/omp-global.XXXXXX")
export SKILLSHARE_CONFIG="$CASE/config.yaml"
export PI_CODING_AGENT_DIR="$CASE/omp/agent"
mkdir -p "$PI_CODING_AGENT_DIR"
printf 'targets: {}\n' > "$SKILLSHARE_CONFIG"
printf '%s\n' '{"$schema":"https://example.com/schema","disabledServers":["external"],"enabledServers":["manual"],"mcpServers":{"docs":{"type":"http","url":"https://example.com/mcp","enabled":false,"timeout":0,"instructions":false,"requestIdFormat":"string","oauth":{"scope":"read"}},"manual":{"command":"manual"}}}' | jq . > "$PI_CODING_AGENT_DIR/mcp.json"
ss mcp add docs --url https://example.com/mcp --target omp -g --sync --no-tui >/dev/null
ss sync mcp -g --json | jq -e '.applied == [] and .plan.changes[0].action == "unchanged"' >/dev/null
ss mcp edit docs --url https://example.com/updated -g --sync --json > "$CASE/update.json"
jq -e '.mcpServers.docs.url == "https://example.com/updated" and .mcpServers.docs.enabled == false and .mcpServers.docs.timeout == 0 and .mcpServers.docs.instructions == false and .mcpServers.docs.requestIdFormat == "string" and .mcpServers.docs.oauth.scope == "read"' "$PI_CODING_AGENT_DIR/mcp.json" >/dev/null
BACKUP=$(jq -r '.backupIds[0]' "$CASE/update.json")
ss mcp restore "$BACKUP" -g --json >/dev/null
jq -e '.mcpServers.docs.url == "https://example.com/mcp"' "$PI_CODING_AGENT_DIR/mcp.json" >/dev/null
ss sync mcp -g >/dev/null
ss mcp remove docs --sync -g --no-tui >/dev/null
jq . "$PI_CODING_AGENT_DIR/mcp.json"
```

Expected:
- exit_code: 0
- jq: .mcpServers.docs == null and .mcpServers.manual.command == "manual"
- jq: .disabledServers == ["external"] and .enabledServers == ["manual"]
- jq: .["$schema"] == "https://example.com/schema"

### Step 3: Project skills, MCP import and a global-server switch

```bash
set -eu
CASE=$(mktemp -d "$HOME/omp-project.XXXXXX")
cd "$CASE"
mkdir -p .skillshare/skills/omp-demo
printf 'targets:\n  - omp\n' > .skillshare/config.yaml
printf '%s\n' '---' 'name: omp-demo' 'description: OMP discovery probe' '---' '# OMP demo' > .skillshare/skills/omp-demo/SKILL.md
ss sync -p >/dev/null
test -f .omp/skills/omp-demo/SKILL.md
ss mcp add docs --url https://example.com/mcp --target omp -p --sync --no-tui >/dev/null
ss mcp import docs --from omp --target omp --replace --dry-run -p --json | jq -e '.blocked == false' >/dev/null
ss mcp add global-docs --disabled --target omp -p --sync --no-tui >/dev/null
jq -e '.mcpServers["global-docs"] == {"enabled":false}' .omp/mcp.json >/dev/null
ss mcp remove global-docs --sync -p --no-tui >/dev/null
jq . .omp/mcp.json
```

Expected:
- exit_code: 0
- jq: .mcpServers.docs.type == "http" and .mcpServers.docs.url == "https://example.com/mcp"
- jq: .mcpServers["global-docs"] == null

### Step 4: Explicit OMP account manages skills and MCP together

```bash
set -eu
CASE=$(mktemp -d "$HOME/omp-account.XXXXXX")
export SKILLSHARE_CONFIG="$CASE/config.yaml"
mkdir -p "$CASE/skills/omp-demo"
printf '%s\n' '---' 'name: omp-demo' 'description: OMP account probe' '---' '# OMP demo' > "$CASE/skills/omp-demo/SKILL.md"
printf 'source: %s/skills\ntargets:\n  omp-work:\n    agent: omp\n    config_dir: %s/profile/agent\n' "$CASE" "$CASE" > "$SKILLSHARE_CONFIG"
ss sync -g >/dev/null
test -f "$CASE/profile/agent/skills/omp-demo/SKILL.md"
ss mcp add docs --url https://example.com/mcp --target omp-work -g --sync --no-tui >/dev/null
ss mcp import docs --from omp-work --target omp-work --replace --dry-run -g --json | jq -e '.blocked == false' >/dev/null
jq . "$CASE/profile/agent/mcp.json"
```

Expected:
- exit_code: 0
- jq: .mcpServers.docs.type == "http" and .mcpServers.docs.url == "https://example.com/mcp"

## Pass Criteria

- All four steps pass without network access or launching MCP servers.
- Dry runs leave the source and native files unchanged.
- OMP settings and unrelated servers survive update, restore and removal.
- Native project and explicit account paths contain both skills and MCP configuration.
