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
# Chromium refuses its sandbox as root, which is how docker exec runs here.
[ "$(id -u)" = 0 ] && export VHS_NO_SANDBOX=true

# name | command | keys after it opens (vhs syntax, ";"-separated)
SCREENS='
list|skillshare list|
list-agents|skillshare list|Tab
list-keys|skillshare list|Type `?`
list-confirm|skillshare list|Down;Type `d`
list-filter|skillshare list|Type `/`;Type `re`
target|skillshare target list|
extras|skillshare extras list|
trash|skillshare trash list|
trash-confirm|skillshare trash list|Space;Down;Space;Type `d`
restore-pick|skillshare restore|
restore|skillshare restore|Enter
log|skillshare log|
log-stats|skillshare log|Tab
log-confirm|skillshare log|Down;Space;Down;Space;Type `d`
audit|skillshare audit|
audit-rules|skillshare audit rules|
diff|skillshare diff|
analyze|skillshare analyze|
search|skillshare search --hub ~/hub.json|Enter
mcp|skillshare mcp|
install-pick|skillshare install file://$HOME/work/team-skills|
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
  fi
  rm -f "$OUT/$name.gif" "$tape"
  ssenv delete "$env" --force >/dev/null 2>&1 || true
}

while IFS='|' read -r name cmd keys; do
  [ -z "$name" ] && continue
  if [ $# -gt 0 ] && [[ " $* " != *" $name "* ]]; then continue; fi
  record "$name" "$cmd" "$keys"
done <<<"$SCREENS"
