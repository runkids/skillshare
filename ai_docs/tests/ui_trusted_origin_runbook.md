# CLI E2E Runbook: UI Trusted Origins and Host Check

Verifies that the dashboard answers only at `localhost`, an IP address, its `--host` name and each `--trusted-origin`, so a DNS rebinding page cannot reach it, while a reverse proxy domain the user trusts still works (issue #809).

## Scope

- Untrusted domain Host gets 403 on every route, reads included
- The refusal names the origin to trust: a page for a browser, JSON for the API, one log line per Host
- `localhost` and IP Hosts work
- A trusted origin works through a base path, including its default port and MCP writes
- Origin must be the trusted origin itself (scheme included)
- `SKILLSHARE_UI_TRUSTED_ORIGINS` fallback, and the flag wins over it
- An origin without a scheme is refused at startup
- `ui stop` still works when the variable is mistyped
- `ui start` forwards trusted origins to the background server

## Environment

Run inside devcontainer. Uses `/tmp/` for an isolated HOME and a fake UI dist.
Servers use ports **49831** and **49832** to avoid conflicts with an existing UI on 19420.

## Step 0: Setup isolated HOME and fake UI dist

```bash
export E2E_HOME="/tmp/ss-e2e-trusted"
rm -rf "$E2E_HOME"
mkdir -p "$E2E_HOME/.config/skillshare/skills" "$E2E_HOME/.cache/skillshare/ui/dev" "$E2E_HOME/.claude/skills"
cat > "$E2E_HOME/.config/skillshare/config.yaml" <<'YAML'
source: ~/.config/skillshare/skills
mode: merge
targets:
  claude:
    path: ~/.claude/skills
YAML
echo '<!DOCTYPE html><html><body>Dashboard</body></html>' > "$E2E_HOME/.cache/skillshare/ui/dev/index.html"
export HOME="$E2E_HOME" XDG_CONFIG_HOME="$E2E_HOME/.config" XDG_CACHE_HOME="$E2E_HOME/.cache" XDG_DATA_HOME="$E2E_HOME/.data" XDG_STATE_HOME="$E2E_HOME/.state"
SS=/workspace/bin/skillshare
code() { curl -s -o /dev/null -w '%{http_code}' "$@"; }
echo "setup_ok=yes"
```

Expected:
- setup_ok=yes

## Step 1: Start with a trusted origin under a base path

```bash
$SS ui -g --host 0.0.0.0 --port 49831 --base-path /skillshare --no-open --trusted-origin https://tools.example.com > /tmp/trusted-server.log 2>&1 &
echo $! > /tmp/trusted-server.pid
for i in $(seq 1 50); do [ "$(code http://127.0.0.1:49831/skillshare/api/health)" = 200 ] && break; sleep 0.2; done
echo "loopback=$(code http://127.0.0.1:49831/skillshare/api/health)"
echo "localhost=$(code -H 'Host: localhost:49831' http://127.0.0.1:49831/skillshare/api/health)"
```

Expected:
- loopback=200
- localhost=200

## Step 2: Untrusted domain is refused, reads included

```bash
echo "rebind_read=$(code -H 'Host: evil.example:49831' http://127.0.0.1:49831/skillshare/api/health)"
echo "rebind_mcp=$(code -X POST -H 'Host: evil.example:49831' -H 'Origin: http://evil.example:49831' -H 'Sec-Fetch-Site: same-origin' -d '{}' http://127.0.0.1:49831/skillshare/api/mcp/import)"
curl -s -H 'Host: evil.example:49831' http://127.0.0.1:49831/skillshare/api/health; echo
curl -s -H 'Host: skills.example.com' -H 'Accept: text/html' http://127.0.0.1:49831/skillshare/ | grep -o 'skillshare ui start --trusted-origin [^<]*'
echo "log_lines=$(grep -c 'refused requests for Host "evil.example:49831"' /tmp/trusted-server.log)"
```

Expected:
- rebind_read=403
- rebind_mcp=403
- --trusted-origin http://evil.example:49831
- skillshare ui start --trusted-origin https://skills.example.com
- log_lines=1

## Step 3: Trusted origin works as a proxy forwards it

```bash
echo "trusted=$(code -H 'Host: tools.example.com' http://127.0.0.1:49831/skillshare/api/health)"
echo "trusted_port=$(code -H 'Host: tools.example.com:443' http://127.0.0.1:49831/skillshare/api/health)"
echo "trusted_mcp=$(code -X POST -H 'Host: tools.example.com' -H 'Origin: https://tools.example.com' -H 'Sec-Fetch-Site: same-origin' -H 'Content-Type: application/json' -d '{}' http://127.0.0.1:49831/skillshare/api/mcp/import)"
echo "http_origin=$(code -H 'Host: tools.example.com' -H 'Origin: http://tools.example.com' http://127.0.0.1:49831/skillshare/api/health)"
kill $(cat /tmp/trusted-server.pid); sleep 0.5
```

Expected:
- trusted=200
- trusted_port=200
- trusted_mcp=400
- http_origin=403

`trusted_mcp=400` means the request reached the MCP handler, which rejects the empty body; a refused request would be 403.

## Step 4: Environment variable fallback, and the flag wins

```bash
export SKILLSHARE_UI_TRUSTED_ORIGINS="https://env.example, https://env2.example"
$SS ui -g --port 49831 --no-open > /tmp/trusted-server.log 2>&1 &
echo $! > /tmp/trusted-server.pid
for i in $(seq 1 50); do [ "$(code http://127.0.0.1:49831/api/health)" = 200 ] && break; sleep 0.2; done
echo "env2=$(code -H 'Host: env2.example' http://127.0.0.1:49831/api/health)"
kill $(cat /tmp/trusted-server.pid); sleep 0.5
$SS ui -g --port 49831 --no-open --trusted-origin https://flag.example > /tmp/trusted-server.log 2>&1 &
echo $! > /tmp/trusted-server.pid
for i in $(seq 1 50); do [ "$(code http://127.0.0.1:49831/api/health)" = 200 ] && break; sleep 0.2; done
echo "flag=$(code -H 'Host: flag.example' http://127.0.0.1:49831/api/health)"
echo "env_ignored=$(code -H 'Host: env.example' http://127.0.0.1:49831/api/health)"
kill $(cat /tmp/trusted-server.pid); sleep 0.5
unset SKILLSHARE_UI_TRUSTED_ORIGINS
```

Expected:
- env2=200
- flag=200
- env_ignored=403

## Step 5: An origin without a scheme is refused

```bash
$SS ui -g --port 49831 --no-open --trusted-origin tools.example.com 2>&1 | tail -1
```

Expected:
- must be a scheme and host

## Step 6: Background start forwards origins, and a mistyped variable does not block stop

```bash
$SS ui start -g --port 49832 --no-open --trusted-origin https://bg.example > /dev/null 2>&1
for i in $(seq 1 50); do [ "$(code http://127.0.0.1:49832/api/health)" = 200 ] && break; sleep 0.2; done
echo "background=$(code -H 'Host: bg.example' http://127.0.0.1:49832/api/health)"
SKILLSHARE_UI_TRUSTED_ORIGINS=bg.example $SS ui stop -g --port 49832 > /dev/null 2>&1; echo "stop_exit=$?"
echo "after_stop=$(code http://127.0.0.1:49832/api/health)"
```

Expected:
- background=200
- stop_exit=0
- after_stop=000

## Pass Criteria

- An untrusted domain Host gets 403 on reads and writes
- `localhost`, IP addresses and trusted origins work, through a base path and with the default port
- Origin must equal the trusted origin
- The flag, the variable fallback, and startup validation behave as documented
- `ui start` forwards origins; `ui stop` ignores a mistyped variable
