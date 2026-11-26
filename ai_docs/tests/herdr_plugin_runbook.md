# CLI E2E Runbook: Herdr plugin target

Validate mod and launcher installation in an isolated environment.

**Origin**: Herdr app plugin target — exercise the managed folder lifecycle.

## Scope

- Root mod discovery, update, and removal.
- Launcher installation at a stable managed path and removal.

## Environment

Run in the devcontainer with mdproof isolation. Use only the isolated home;
never point `HERDR_APP_HOME` at an existing Herdr installation.

## Steps

### 1. Install, update, and remove

```bash
set -eu
export XDG_CONFIG_HOME="$HOME/.config" XDG_DATA_HOME="$HOME/.local/share" XDG_STATE_HOME="$HOME/.local/state" XDG_CACHE_HOME="$HOME/.cache"
export HERDR_APP_HOME="$HOME/herdr-fixture"
mkdir -p "$HERDR_APP_HOME" "$HOME/plugin-fixture/mods/demo" "$HOME/launcher-fixture/bin"
printf 'export default {}\n' > "$HOME/plugin-fixture/mods/demo/main.js"
printf '#!/bin/sh\nprintf "fixture\\n"\n' > "$HOME/launcher-fixture/bin/demo-app"
chmod 755 "$HOME/launcher-fixture/bin/demo-app"
/workspace/bin/skillshare plugin add "$HOME/plugin-fixture" --target herdr-app --no-tui -g
/workspace/bin/skillshare plugin add "$HOME/launcher-fixture" --entry bin/demo-app --target herdr-app --no-tui -g | tee "$HOME/launcher-output"
test -f "$HERDR_APP_HOME/mods/demo/main.js"
launcher=$(find "$HOME" -path "*/plugins/*/herdr-app/demo-app/bin/demo-app" -type f -print -quit)
test -n "$launcher"
grep -F "$launcher" "$HOME/launcher-output"
test -x "$launcher"
test ! -e "$HOME/.local/bin/demo-app"
printf 'export default { updated: true }\n' > "$HOME/plugin-fixture/mods/demo/main.js"
/workspace/bin/skillshare plugin check demo -g
/workspace/bin/skillshare plugin update demo --no-tui -g
/workspace/bin/skillshare plugin remove demo --no-tui -g
/workspace/bin/skillshare plugin remove demo-app --no-tui -g
test ! -e "$HERDR_APP_HOME/mods/demo"
test ! -e "$launcher"
printf 'Herdr lifecycle passed\n'
```

**Expected**:

- exit_code: 0
- Herdr lifecycle passed

## Pass Criteria

- Both installations are discovered and removed in the isolated home.
- No pre-existing Herdr configuration is touched.
