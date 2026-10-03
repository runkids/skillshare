#!/usr/bin/env bash
# Screenshot the full-screen TUIs with vhs, one fresh ssenv per screen,
# each filled by tui-fixture.sh. Run inside the devcontainer.
#
#   scripts/screenshots/record-tui.sh [name...]   # default: every screen
#
# BIN   directory holding the skillshare binary to record (default /tmp/tuibin)
# OUT   where the PNGs go (default /tmp/tui-shots)
# THEME dark or light (default dark)
set -euo pipefail

BIN=${BIN:-/tmp/tuibin}
OUT=${OUT:-/tmp/tui-shots}
THEME=${THEME:-dark}
HERE=$(cd "$(dirname "$0")" && pwd)
"$HERE/../../.devcontainer/ensure-recording-tools.sh"
# Chromium refuses its sandbox as root, which is how docker exec runs here.
[ "$(id -u)" = 0 ] && export VHS_NO_SANDBOX=true

# name | command | keys after it opens (vhs syntax, ";"-separated)
SCREENS='
list|skillshare list|
list-agents|skillshare list|Tab
list-keys|skillshare list|Type `?`
list-confirm|skillshare list|Down;Type `d`
list-filter|skillshare list|Type `/`;Type `re`
list-files|skillshare list|Enter
list-open-tracked|skillshare list|Type `/`;Type `review`;Enter;Enter
list-open-agent|skillshare list|Tab;Enter
target|skillshare target list|
target-edit|skillshare target list|Type `e`
target-confirm|skillshare target list|Type `d`
extras|skillshare extras list|
extras-edit|skillshare extras list|Type `e`
extras-confirm|skillshare extras list|Type `d`
extras-files|skillshare extras list|Enter
extras-init|skillshare extras init|Type `prompts`;Enter;Down;Enter;Type `system.md`;Enter;Enter;Type `~/.pi/agent`;Enter;Type `APPEND_SYSTEM.md`;Enter;Down;Down;Enter
extras-init-error|skillshare extras init|Type `a/b`;Enter
trash|skillshare trash list|
trash-files|skillshare trash list|Enter
trash-confirm|skillshare trash list|Space;Down;Space;Type `d`
restore-pick|skillshare restore|
restore|skillshare restore|Enter
restore-confirm|skillshare restore|Enter;Enter;Type `d`
log|skillshare log|
log-stats|skillshare log|Tab
log-confirm|skillshare log|Down;Space;Down;Space;Type `d`
audit|skillshare audit|
audit-files|skillshare audit|Type `/`;Type `risky`;Enter;Enter
audit-files-next|skillshare audit|Type `/`;Type `risky`;Enter;Enter;Type `n`
audit-rules|skillshare audit rules|
audit-rules-severity|skillshare audit rules|Enter;Down;Type `e`
diff|skillshare diff|
analyze|skillshare analyze|
analyze-files|skillshare analyze|Enter
search|skillshare search --hub ~/hub.json|Enter
mcp|skillshare mcp|
mcp-keys|skillshare mcp|Type `?`
mcp-remove|skillshare mcp|Type `d`;Type `y`
mcp-paste|skillshare mcp|Type `n`;Type `{"command": "npx",`;Alt+Enter;Type `"args": ["-y", "docs-mcp"]}`
install-pick|skillshare install file://$HOME/work/team-skills|
install-folders|skillshare install file://$HOME/work/big-repo|
install-folder-skills|skillshare install file://$HOME/work/big-repo|Down;Enter
new-pick|skillshare new my-skill|
'

vhs_theme="Catppuccin Mocha"
[ "$THEME" = light ] && vhs_theme="Catppuccin Latte"
mkdir -p "$OUT"

record() {
  local name=$1 cmd=$2 keys=$3 env="rec-tui-$1"
  ssenv delete "$env" --force >/dev/null 2>&1 || true
  ssenv create "$env" >/dev/null
  PATH="$BIN:$PATH" ssenv enter "$env" -- "$HERE/tui-fixture.sh"

  local tape="$OUT/$name.tape"
  {
    echo "Output \"$OUT/$name.gif\""
    echo 'Set Shell bash'
    echo 'Set FontSize 16'
    echo 'Set Width 1400'
    echo 'Set Height 860'
    echo 'Set Padding 20'
    echo "Set Theme \"$vhs_theme\""
    echo 'Hide'
    echo "Type \`export PATH=$BIN:\$PATH SKILLSHARE_THEME=$THEME PS1='~ ' && eval \"\$(ssenv --eval use $env)\" && clear\`"
    echo 'Enter'
    echo 'Sleep 1s'
    echo 'Show'
    echo "Type \`$cmd\`"
    echo 'Enter'
    echo 'Sleep 3s'
    local k
    IFS=';' read -ra ks <<<"$keys"
    for k in "${ks[@]}"; do
      [ -n "$k" ] && { echo "$k"; echo 'Sleep 1s'; }
    done
    echo "Screenshot \"$OUT/$name-$THEME.png\""
    echo 'Sleep 500ms'
  } >"$tape"
  if vhs "$tape" >"$OUT/$name.log" 2>&1; then
    echo "ok $name"; rm -f "$OUT/$name.log"
  else
    echo "FAIL $name (see $OUT/$name.log)"; tail -5 "$OUT/$name.log"
    failed=1
  fi
  rm -f "$OUT/$name.gif" "$tape"
  ssenv delete "$env" --force >/dev/null 2>&1 || true
}

failed=0
while IFS='|' read -r name cmd keys; do
  [ -z "$name" ] && continue
  if [ $# -gt 0 ] && [[ " $* " != *" $name "* ]]; then continue; fi
  record "$name" "$cmd" "$keys"
done <<<"$SCREENS"
exit "$failed"
