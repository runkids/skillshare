# CLI E2E Runbook: UI Base Path for Reverse Proxy

Verifies `skillshare ui --base-path` serves the dashboard and API under a sub-path prefix, as required for reverse proxy deployments (issue #85).

## Scope

- `--base-path` flag routes API and SPA under a prefix
- Bare path redirect (`/skillshare` → `/skillshare/`)
- Unprefixed paths return 404
- `__BASE_PATH__` injected into `index.html`
- SPA fallback works under base path
- `-b` short flag accepted
- `SKILLSHARE_UI_BASE_PATH` env var fallback
- Multi-level base path (`/tools/skillshare`)
- No base path = unchanged behavior (regression)

## Environment

Run inside devcontainer. Uses `/tmp/` for isolated HOME and a fake UI dist.
Server uses port **19421** to avoid conflicts with existing UI on 19420.

## Step 0: Setup isolated HOME and fake UI dist

```bash
export E2E_HOME="/tmp/ss-e2e-basepath"
rm -rf "$E2E_HOME"
mkdir -p "$E2E_HOME/.config/skillshare/skills/demo"
mkdir -p "$E2E_HOME/.cache/skillshare/ui/dev"
mkdir -p "$E2E_HOME/.claude/skills"

cat > "$E2E_HOME/.config/skillshare/skills/demo/SKILL.md" <<'SKILL'
---
name: demo
---
# Demo skill
SKILL

cat > "$E2E_HOME/.config/skillshare/config.yaml" <<'YAML'
source: ~/.config/skillshare/skills
mode: merge
targets:
  claude:
    path: ~/.claude/skills
YAML

cat > "$E2E_HOME/.cache/skillshare/ui/dev/index.html" <<'HTML'
<!DOCTYPE html>
<html>
<head><title>Skillshare UI</title></head>
<body><h1>Dashboard</h1></body>
</html>
HTML

echo "setup_ok=yes"
```

Expected:
- exit_code: 0
- setup_ok=yes

## Step 1: Start server with --base-path

```bash
export E2E_HOME="/tmp/ss-e2e-basepath"
export HOME="$E2E_HOME"
export XDG_CONFIG_HOME="$E2E_HOME/.config"
export XDG_DATA_HOME="$E2E_HOME/.local/share"
export XDG_STATE_HOME="$E2E_HOME/.local/state"
export XDG_CACHE_HOME="$E2E_HOME/.cache"

fuser -k 19421/tcp 2>/dev/null || true
sleep 1

cd /workspace
go run ./cmd/skillshare ui --base-path /skillshare --host 0.0.0.0 --port 19421 --no-open -g > /tmp/basepath-server.log 2>&1 &
SERVER_PID=$!
echo "server_pid=$SERVER_PID"

sleep 5
cat /tmp/basepath-server.log
```

Expected:
- exit_code: 0
- regex: server_pid=\d+
- regex: running at http://.*:19421/skillshare/

## Step 2: API health with prefix returns 200

```bash
curl -sf http://localhost:19421/skillshare/api/health
```

Expected:
- exit_code: 0
- jq: .status == "ok"

## Step 3: API health without prefix returns 404

```bash
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:19421/api/health)
echo "status=$HTTP_CODE"
test "$HTTP_CODE" = "404" && echo "correctly_rejected=yes"
```

Expected:
- exit_code: 0
- status=404
- correctly_rejected=yes

## Step 4: Bare path redirects to trailing slash

```bash
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:19421/skillshare)
LOCATION=$(curl -s -o /dev/null -w "%{redirect_url}" http://localhost:19421/skillshare)
echo "status=$HTTP_CODE"
echo "location=$LOCATION"
```

Expected:
- exit_code: 0
- status=301
- regex: location=.*/skillshare/

## Step 5: Root path returns 404

```bash
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:19421/)
echo "status=$HTTP_CODE"
test "$HTTP_CODE" = "404" && echo "root_blocked=yes"
```

Expected:
- exit_code: 0
- status=404
- root_blocked=yes

## Step 6: Index.html served with __BASE_PATH__ injection

```bash
BODY=$(curl -sf http://localhost:19421/skillshare/)
echo "$BODY"
echo "$BODY" | grep -q '__BASE_PATH__' && echo "injection_found=yes"
echo "$BODY" | grep -q '"/skillshare"' && echo "value_correct=yes"
```

Expected:
- exit_code: 0
- injection_found=yes
- value_correct=yes
- __BASE_PATH__

## Step 7: SPA fallback serves index.html for unknown routes

```bash
HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:19421/skillshare/skills/nonexistent)
BODY=$(curl -sf http://localhost:19421/skillshare/skills/nonexistent)
echo "status=$HTTP_CODE"
echo "$BODY" | grep -q '__BASE_PATH__' && echo "spa_fallback_ok=yes"
```

Expected:
- exit_code: 0
- status=200
- spa_fallback_ok=yes

## Step 8: API overview returns valid data

```bash
curl -sf http://localhost:19421/skillshare/api/overview
```

Expected:
- exit_code: 0
- jq: .skillCount >= 0
- jq: .mode == "merge"

## Step 9: Stop server and cleanup

```bash
fuser -k 19421/tcp 2>/dev/null || true
sleep 1
echo "server_stopped=yes"
```

Expected:
- exit_code: 0
- server_stopped=yes

## Step 10: --base-path missing value returns error

```bash
cd /workspace
go run ./cmd/skillshare ui --base-path 2>&1 || true
```

Expected:
- --base-path requires a value

## Step 11: -b short flag missing value returns error

```bash
cd /workspace
go run ./cmd/skillshare ui -b 2>&1 || true
```

Expected:
- --base-path requires a value

## Step 12: Env var SKILLSHARE_UI_BASE_PATH is read

```bash
export E2E_HOME="/tmp/ss-e2e-basepath"
export HOME="$E2E_HOME"
export XDG_CONFIG_HOME="$E2E_HOME/.config"
export XDG_DATA_HOME="$E2E_HOME/.local/share"
export XDG_STATE_HOME="$E2E_HOME/.local/state"
export XDG_CACHE_HOME="$E2E_HOME/.cache"
export SKILLSHARE_UI_BASE_PATH="/from-env"

fuser -k 19422/tcp 2>/dev/null || true
sleep 1

cd /workspace
go run ./cmd/skillshare ui --host 0.0.0.0 --port 19422 --no-open -g > /tmp/basepath-env.log 2>&1 &
sleep 5
cat /tmp/basepath-env.log

curl -sf http://localhost:19422/from-env/api/health
fuser -k 19422/tcp 2>/dev/null || true
```

Expected:
- exit_code: 0
- regex: running at http://.*:19422/from-env/
- jq: .status == "ok"

## Step 13: Final cleanup

```bash
fuser -k 19421/tcp 2>/dev/null || true
fuser -k 19422/tcp 2>/dev/null || true
rm -rf /tmp/ss-e2e-basepath /tmp/basepath-server.log /tmp/basepath-env.log
echo "cleanup_done=yes"
```

Expected:
- exit_code: 0
- cleanup_done=yes

## Pass/Fail Criteria

Pass when all are true:

- API endpoints accessible under `/skillshare/api/...`
- API endpoints return 404 without prefix
- Bare path redirects to trailing slash
- Root path returns 404
- `index.html` contains injected `__BASE_PATH__`
- SPA fallback serves `index.html` for unknown routes
- `--base-path` / `-b` flag validation works
- `SKILLSHARE_UI_BASE_PATH` env var is respected

Fail if any step errors or API is unreachable under the base path.
