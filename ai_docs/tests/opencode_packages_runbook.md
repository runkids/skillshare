# OpenCode directory package regression

## Scope

Verify issue #913 through the CLI: a hook bound to OpenCode becomes a directory
package and a directory registration on OpenCode v2, stays a flat file on v1, and
an OpenCode version that cannot be detected holds back only the OpenCode binding.

A shell script named `opencode` that answers `--version` stands in for the client.
No OpenCode session, plugin code, dependency installation or real user
configuration is used.

## Environment

Run inside the devcontainer in a fresh `ssenv`. Every step writes only under
`$HOME` and the environment's config directory. Shell variables do not persist between steps, so each step sets its own.

## Steps

### Step 1: OpenCode v2 receives a directory package

```bash
set -e
unset OPENCODE_CONFIG OPENCODE_CONFIG_DIR OPENCODE_CONFIG_CONTENT
C="${XDG_CONFIG_HOME:-$HOME/.config}"
rm -rf "$HOME/oc-v1" "$HOME/oc-v2" "$HOME/oc-broken" "$C/opencode"
ss hooks remove native --sync -g >/dev/null 2>&1 || true
mkdir -p "$HOME/oc-v1" "$HOME/oc-v2" "$HOME/oc-broken"
printf '#!/bin/sh\n[ "$#" = 1 ] && [ "$1" = --version ] || exit 91\necho 1.18.31\n' > "$HOME/oc-v1/opencode"
printf '#!/bin/sh\n[ "$#" = 1 ] && [ "$1" = --version ] || exit 91\necho "opencode v2.3.0"\n' > "$HOME/oc-v2/opencode"
printf '#!/bin/sh\nexit 127\n' > "$HOME/oc-broken/opencode"
chmod +x "$HOME/oc-v1/opencode" "$HOME/oc-v2/opencode" "$HOME/oc-broken/opencode"
cat > "$HOME/opencode-entry.json" <<'JSON'
{"bindings":{
  "opencode":{"code":"export default {}\n"},
  "claude":{"events":{"Stop":[{"hooks":[{"type":"command","command":"echo done"}]}]}}
}}
JSON
PATH="$HOME/oc-v2:$PATH" ss hooks add native --file "$HOME/opencode-entry.json" --sync -g --json
```

Expected:
- exit_code: 0
- jq: .plan.blocked == false
- jq: [.applied[] | select(endswith("/opencode/plugins/skillshare-native/package.json"))] | length == 1
- jq: [.applied[] | select(endswith("/opencode/plugins/skillshare-native/index.ts"))] | length == 1
- jq: [.applied[] | select(endswith("skillshare-native.ts"))] | length == 0
- jq: [.plan.warnings[] | select(startswith("OpenCode v2:"))] | length == 1

### Step 2: The registration is the package directory

```bash
set -e
C="${XDG_CONFIG_HOME:-$HOME/.config}"
jq -c '{plugins, manifest: input}' "$C/opencode/opencode.json" "$C/opencode/plugins/skillshare-native/package.json"
```

Expected:
- exit_code: 0
- jq: .plugins | length == 1
- jq: .plugins[0] | startswith("file://") and endswith("/opencode/plugins/skillshare-native")
- jq: .manifest.exports == "./index.ts"

### Step 3: A second sync on v2 changes nothing and reports nothing

```bash
set -e
unset OPENCODE_CONFIG OPENCODE_CONFIG_DIR OPENCODE_CONFIG_CONTENT
PATH="$HOME/oc-v2:$PATH" ss hooks sync -g --json
```

Expected:
- exit_code: 0
- jq: .applied | length == 0
- jq: .plan.warnings | length == 0
- jq: [.plan.changes[] | select(.action != "unchanged")] | length == 0

### Step 4: An undetectable OpenCode version holds back only OpenCode

```bash
set -e
unset OPENCODE_CONFIG OPENCODE_CONFIG_DIR OPENCODE_CONFIG_CONTENT
C="${XDG_CONFIG_HOME:-$HOME/.config}"
PATH="$HOME/oc-broken:$PATH" ss hooks sync -g --json
test -f "$C/opencode/plugins/skillshare-native/package.json"
```

Expected:
- exit_code: 0
- jq: .plan.blocked == false
- jq: [.plan.changes[] | select(.target == "claude" and .action == "unchanged")] | length == 1
- jq: [.plan.changes[] | select(.target == "opencode")] | length == 0
- jq: [.plan.warnings[] | select(contains("its hook bindings are skipped"))] | length == 1
- jq: [.plan.warnings[] | select(contains("are left as they are"))] | length == 1

### Step 5: OpenCode v1 receives the flat file and retires the package

```bash
set -e
unset OPENCODE_CONFIG OPENCODE_CONFIG_DIR OPENCODE_CONFIG_CONTENT
C="${XDG_CONFIG_HOME:-$HOME/.config}"
PATH="$HOME/oc-v1:$PATH" ss hooks sync -g >/dev/null
P="$C/opencode/plugins"
REGISTERED=$(grep -c skillshare-native "$C/opencode/opencode.json" 2>/dev/null || true)
jq -n --argjson flat "$(test -f "$P/skillshare-native.ts" && echo true || echo false)" \
  --argjson package "$(test -e "$P/skillshare-native" && echo true || echo false)" \
  --argjson registered "${REGISTERED:-0}" \
  '{flat: $flat, package: $package, registered: $registered}'
```

Expected:
- exit_code: 0
- jq: .flat == true
- jq: .package == false
- jq: .registered == 0

### Step 6: Removing the hook removes its OpenCode output

```bash
set -e
unset OPENCODE_CONFIG OPENCODE_CONFIG_DIR OPENCODE_CONFIG_CONTENT
C="${XDG_CONFIG_HOME:-$HOME/.config}"
PATH="$HOME/oc-v1:$PATH" ss hooks remove native --sync -g >/dev/null
ls "$C/opencode/plugins" 2>/dev/null | grep -c skillshare- || true
```

Expected:
- exit_code: 0
- regex: ^0$

## Pass Criteria

All steps pass. Together they show the v2 directory layout and registration, a
no-op second sync without warnings, that a missing or failing `opencode` binary
neither blocks other Agents nor touches existing OpenCode output, the v1 flat
file, and cleanup. A passing run is not evidence that a real OpenCode runtime
loaded or executed a plugin.
