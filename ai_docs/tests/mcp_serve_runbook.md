# CLI E2E Runbook: `skillshare mcp serve`

Validates the SEP-2640 skills server over stdio and Streamable HTTP, and its opt-in skill editing.

**Origin**: issue #428 — Agents that only reach an MCP endpoint need the skills Skillshare manages. Editing: issue #540. Executor index: issue #574.

## Scope

- `server/discover` declares `resources` and the `io.modelcontextprotocol/skills` extension
- `skills/list` returns one entry per valid, enabled skill with frontmatter and a complete sha256 manifest
- A skill whose `name` differs from its directory is skipped with a stderr warning; a disabled skill is not served
- `--check` lists skipped skills and the served count without serving
- `--target` applies that target's `exclude` filter
- `resources/read` returns a listed file and rejects a URI that escapes the skill
- Over HTTP: bearer token required, a newly added skill appears without a restart
- A non-loopback `--http` address without `SKILLSHARE_MCP_TOKEN` is refused
- A non-loopback `--http` address with a token but no TLS or explicit opt-in is refused
- `--http-allow-insecure` starts with exactly one warning and still requires a token
- With `mcp.serve.edit`, a preview writes nothing, apply saves exactly the preview once, and an unlisted skill is refused
- Over HTTP, the Executor index at `/.well-known/agent-skills/` lists text skills, leaves out one with a binary file (and `--check` says so), serves listed files, and changes a skill's `version` after `apply_skill_edit`

## Environment

Run inside the devcontainer with a fresh ssenv HOME:

```bash
ssenv create mcp-serve-e2e --init
ssenv enter mcp-serve-e2e -- mdproof --report json /workspace/ai_docs/tests/mcp_serve_runbook.md
```

Every step works from `$HOME` and points `SKILLSHARE_CONFIG` at `$HOME/mcp-serve/config.yaml`.
The server drops requests still in flight when stdin closes, so stdio steps keep stdin open
with `sleep 1`. Step 6 listens on port 47941; pick another free port if it is taken.

## Steps

### Step 1: Discover and list over stdio

```bash
set -eu
cd "$HOME"
CASE="$HOME/mcp-serve"
rm -rf "$CASE"
mkdir -p "$CASE/skills"
export SKILLSHARE_CONFIG="$CASE/config.yaml"
cat > "$SKILLSHARE_CONFIG" <<YAML
source: $CASE/skills
targets:
  claude:
    skills:
      path: $CASE/claude-skills
      exclude: [notes]
YAML
for n in pdf notes off; do
  mkdir -p "$CASE/skills/$n"
  printf -- '---\nname: %s\ndescription: Use when testing %s\n---\n# %s\n' "$n" "$n" "$n" > "$CASE/skills/$n/SKILL.md"
done
mkdir -p "$CASE/skills/pdf/references" "$CASE/skills/renamed"
echo forms > "$CASE/skills/pdf/references/FORMS.md"
printf -- '---\nname: original\ndescription: Use when testing\n---\n# Renamed\n' > "$CASE/skills/renamed/SKILL.md"
echo off > "$CASE/skills/.skillignore"
M='"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"e2e","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}'
{ printf '%s\n' "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"server/discover\",\"params\":{$M}}" "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"skills/list\",\"params\":{$M}}"; sleep 1; } \
  | ss mcp serve -g 2>"$CASE/stderr" | jq -sc 'map({(.id|tostring): .result}) | add'
```

Expected:
- exit_code: 0
- jq: .["1"].capabilities.extensions | has("io.modelcontextprotocol/skills")
- jq: .["1"].capabilities.resources == {}
- jq: [.["2"].skills[].uri] == ["skill://notes/SKILL.md", "skill://pdf/SKILL.md"]
- jq: .["2"].resultType == "complete"
- jq: (.["2"].skills[] | select(.uri == "skill://pdf/SKILL.md") | [.resources[].uri] | sort) == ["skill://pdf/SKILL.md", "skill://pdf/references/FORMS.md"]
- jq: [.["2"].skills[].resources[].digest | test("^sha256:[0-9a-f]{64}$")] | all

### Step 2: The renamed skill is reported on stderr

```bash
cat "$HOME/mcp-serve/stderr"
```

Expected:
- exit_code: 0
- skipped renamed: SKILL.md name "original" does not match directory name "renamed"

### Step 3: `--check` lists the skipped skill without serving

```bash
cd "$HOME"
export SKILLSHARE_CONFIG="$HOME/mcp-serve/config.yaml"
NO_COLOR=1 ss mcp serve --check -g < /dev/null
```

Expected:
- exit_code: 0
- SKILL.md name "original" does not match directory name "renamed"
- 2 skills served, 1 skipped

### Step 4: `--target` applies the target's exclude filter

```bash
set -eu
cd "$HOME"
export SKILLSHARE_CONFIG="$HOME/mcp-serve/config.yaml"
M='"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"e2e","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}'
{ printf '%s\n' "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"skills/list\",\"params\":{$M}}"; sleep 1; } \
  | ss mcp serve -g --target claude 2>/dev/null | jq -c .result
```

Expected:
- exit_code: 0
- jq: [.skills[].uri] == ["skill://pdf/SKILL.md"]

### Step 5: Read a listed file; reject an escaping URI

```bash
set -eu
cd "$HOME"
export SKILLSHARE_CONFIG="$HOME/mcp-serve/config.yaml"
M='"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"e2e","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}'
{ printf '%s\n' \
    "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"resources/read\",\"params\":{$M,\"uri\":\"skill://pdf/references/FORMS.md\"}}" \
    "{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"resources/read\",\"params\":{$M,\"uri\":\"skill://pdf/%2e%2e/notes/SKILL.md\"}}"; sleep 1; } \
  | ss mcp serve -g 2>/dev/null | jq -sc 'map({(.id|tostring): .}) | add'
```

Expected:
- exit_code: 0
- jq: .["1"].result.contents[0].text == "forms\n"
- jq: .["2"].error.code == -32602

### Step 6: HTTP needs the token and picks up a new skill without a restart

```bash
set -eu
cd "$HOME"
CASE="$HOME/mcp-serve"
export SKILLSHARE_CONFIG="$CASE/config.yaml"
export SKILLSHARE_MCP_TOKEN=e2e-token
ss mcp serve -g --http 127.0.0.1:47941 2>/dev/null &
PID=$!
trap 'kill $PID 2>/dev/null || true' EXIT
for _ in 1 2 3 4 5 6 7 8 9 10; do curl -s -o /dev/null http://127.0.0.1:47941/ && break; sleep 0.5; done
M='"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"e2e","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}'
list() {
  curl -s -X POST http://127.0.0.1:47941/ -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' -H 'MCP-Protocol-Version: 2026-07-28' \
    -H 'Mcp-Method: skills/list' "$@" \
    -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"skills/list\",\"params\":{$M}}"
}
UNAUTH=$(list -o /dev/null -w '%{http_code}')
BEFORE=$(list -H "Authorization: Bearer $SKILLSHARE_MCP_TOKEN" | sed -n 's/^data: //p' | jq '.result.skills | length')
mkdir -p "$CASE/skills/late"
printf -- '---\nname: late\ndescription: Use when testing late\n---\n# late\n' > "$CASE/skills/late/SKILL.md"
sleep 6
AFTER=$(list -H "Authorization: Bearer $SKILLSHARE_MCP_TOKEN" | sed -n 's/^data: //p' | jq '.result.skills | length')
jq -nc --arg u "$UNAUTH" --argjson b "$BEFORE" --argjson a "$AFTER" '{unauth: $u, before: $b, after: $a}'
```

Expected:
- exit_code: 0
- jq: .unauth == "401"
- jq: .before == 2
- jq: .after == 3

### Step 7: A network listener without a token is refused

```bash
cd "$HOME"
export SKILLSHARE_CONFIG="$HOME/mcp-serve/config.yaml"
unset SKILLSHARE_MCP_TOKEN
ss mcp serve -g --http 0.0.0.0:47942
```

Expected:
- exit_code: 1
- SKILLSHARE_MCP_TOKEN

### Step 8: A network listener without TLS is refused

```bash
cd "$HOME"
export SKILLSHARE_CONFIG="$HOME/mcp-serve/config.yaml"
SKILLSHARE_MCP_TOKEN=change-me ss mcp serve -g --http 0.0.0.0:47943
```

Expected:
- exit_code: 1
- --tls-cert

### Step 8a: Explicit plain HTTP opt-in warns once and keeps bearer authentication

```bash
set -eu
cd "$HOME"
CASE="$HOME/mcp-serve"
export SKILLSHARE_CONFIG="$CASE/config.yaml"
export SKILLSHARE_MCP_TOKEN=e2e-throwaway-token
ss mcp serve -g --http 0.0.0.0:47946 --http-allow-insecure 2>"$CASE/insecure-stderr" &
PID=$!
trap 'kill "$PID" 2>/dev/null || true; wait "$PID" 2>/dev/null || true' EXIT
URL=http://127.0.0.1:47946/.well-known/agent-skills/index.json
for _ in 1 2 3 4 5 6 7 8 9 10; do curl -s -o /dev/null "$URL" && break; sleep 0.5; done
UNAUTH=$(curl -s -o /dev/null -w '%{http_code}' "$URL")
AUTH=$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: Bearer $SKILLSHARE_MCP_TOKEN" "$URL")
kill "$PID"
wait "$PID"
trap - EXIT
WARNINGS=$(grep -c '^warning: --http-allow-insecure' "$CASE/insecure-stderr")
jq -nc --arg u "$UNAUTH" --arg a "$AUTH" --argjson w "$WARNINGS" '{unauth: $u, auth: $a, warnings: $w}'
```

Expected:
- exit_code: 0
- jq: .unauth == "401"
- jq: .auth == "200"
- jq: .warnings == 1

### Step 8b: Plain HTTP opt-in without a token is still refused

```bash
cd "$HOME"
export SKILLSHARE_CONFIG="$HOME/mcp-serve/config.yaml"
unset SKILLSHARE_MCP_TOKEN
ss mcp serve -g --http 0.0.0.0:47946 --http-allow-insecure
```

Expected:
- exit_code: 1
- without authentication
- SKILLSHARE_MCP_TOKEN

### Step 9: Preview and apply an edit over HTTP

```bash
set -eu
cd "$HOME"
CASE="$HOME/mcp-serve"
export SKILLSHARE_CONFIG="$CASE/config.yaml"
printf 'mcp:\n  serve:\n    edit:\n      skills: [pdf]\n' >> "$SKILLSHARE_CONFIG"
ss mcp serve -g --http 127.0.0.1:47944 2>/dev/null &
PID=$!
trap 'kill $PID 2>/dev/null || true' EXIT
for _ in 1 2 3 4 5 6 7 8 9 10; do curl -s -o /dev/null http://127.0.0.1:47944/ && break; sleep 0.5; done
M='"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"e2e","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}'
call() { # tool, arguments JSON → the result
  curl -s -X POST http://127.0.0.1:47944/ -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' -H 'MCP-Protocol-Version: 2026-07-28' \
    -H 'Mcp-Method: tools/call' -H "Mcp-Name: $1" \
    -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"$1\",\"arguments\":$2,$M}}" \
    | sed -n 's/^data: //p' | jq -c '.result'
}
FILE="$CASE/skills/pdf/references/FORMS.md"
REV=$(call read_skill '{"uri":"skill://pdf/references/FORMS.md"}' | jq -r '.content[0].text' | sed -n 's/^Revision: //p')
PREVIEW=$(call preview_skill_edit "$(jq -nc --arg r "$REV" '{uri:"skill://pdf/references/FORMS.md",base_revision:$r,content:"forms v2\n"}')")
UNCHANGED=$(cat "$FILE")
ID=$(echo "$PREVIEW" | jq -r '.content[0].text' | sed -n 's/^Preview \([0-9a-f]*\) .*/\1/p')
APPLY=$(call apply_skill_edit "{\"preview_id\":\"$ID\"}")
AGAIN=$(call apply_skill_edit "{\"preview_id\":\"$ID\"}")
NOTES_REV=$(call read_skill '{"uri":"skill://notes/SKILL.md"}' | jq -r '.content[0].text' | sed -n 's/^Revision: //p')
NOTES=$(call preview_skill_edit "$(jq -nc --arg r "$NOTES_REV" '{uri:"skill://notes/SKILL.md",base_revision:$r,content:"x\n"}')")
jq -nc --arg u "$UNCHANGED" --arg f "$(cat "$FILE")" --argjson p "$PREVIEW" --argjson a "$APPLY" --argjson g "$AGAIN" --argjson n "$NOTES" \
  '{unchanged: $u, file: $f, diff: ($p.content[0].text | contains("+ forms v2")), applied: ($a.isError | not), again: $g.content[0].text, notes: $n.content[0].text}'
```

Expected:
- exit_code: 0
- jq: .unchanged == "forms"
- jq: .diff == true
- jq: .applied == true
- jq: .file == "forms v2"
- jq: .again | startswith("expired:")
- jq: .notes | startswith("not_writable:")

### Step 10: Executor index over HTTP

```bash
set -eu
cd "$HOME"
CASE="$HOME/mcp-serve"
export SKILLSHARE_CONFIG="$CASE/config.yaml"
mkdir -p "$CASE/skills/logo"
printf -- '---\nname: logo\ndescription: Use when testing logo\n---\n# logo\n' > "$CASE/skills/logo/SKILL.md"
printf '\211PNG\377' > "$CASE/skills/logo/logo.png"
CHECK=$(ss mcp serve -g --check 2>&1)
ss mcp serve -g --http 127.0.0.1:47945 2>/dev/null &
PID=$!
trap 'kill $PID 2>/dev/null || true' EXIT
for _ in 1 2 3 4 5 6 7 8 9 10; do curl -s -o /dev/null http://127.0.0.1:47945/ && break; sleep 0.5; done
W=http://127.0.0.1:47945/.well-known/agent-skills
BEFORE=$(curl -sf "$W/index.json")
AGAIN=$(curl -sf "$W/index.json")
FORMS=$(curl -sf "$W/pdf/references/FORMS.md")
M='"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"e2e","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}'
call() { # tool, arguments JSON → the result
  curl -s -X POST http://127.0.0.1:47945/ -H 'Content-Type: application/json' \
    -H 'Accept: application/json, text/event-stream' -H 'MCP-Protocol-Version: 2026-07-28' \
    -H 'Mcp-Method: tools/call' -H "Mcp-Name: $1" \
    -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"$1\",\"arguments\":$2,$M}}" \
    | sed -n 's/^data: //p' | jq -c '.result'
}
REV=$(call read_skill '{"uri":"skill://pdf/references/FORMS.md"}' | jq -r '.content[0].text' | sed -n 's/^Revision: //p')
ID=$(call preview_skill_edit "$(jq -nc --arg r "$REV" '{uri:"skill://pdf/references/FORMS.md",base_revision:$r,content:"forms v3\n"}')" | jq -r '.content[0].text' | sed -n 's/^Preview \([0-9a-f]*\) .*/\1/p')
call apply_skill_edit "{\"preview_id\":\"$ID\"}" >/dev/null
AFTER=$(curl -sf "$W/index.json")
jq -nc --argjson b "$BEFORE" --argjson a "$AFTER" --arg same "$([ "$BEFORE" = "$AGAIN" ] && echo yes)" --arg forms "$FORMS" --arg check "$CHECK" \
  '{names: [$b.skills[].name], pdf: ($b.skills[] | select(.name == "pdf") | .files), same: $same, forms: $forms,
    changed: (($b.skills[] | select(.name == "pdf") | .version) != ($a.skills[] | select(.name == "pdf") | .version)),
    check: ($check | contains("logo") and contains("not UTF-8"))}'
```

Expected:
- exit_code: 0
- jq: .names | index("logo") == null
- jq: .names | index("pdf") != null
- jq: .pdf == ["SKILL.md", "references/FORMS.md"]
- jq: .same == "yes"
- jq: .forms == "forms v2"
- jq: .changed == true
- jq: .check == true

## Pass Criteria

- All steps marked PASS
- No `ss mcp serve` process remains after Steps 6, 8a, 9 and 10
