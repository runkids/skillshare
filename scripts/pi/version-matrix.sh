#!/usr/bin/env bash
# Runs the Pi package contract against exact, pinned Pi versions and records what
# ran in scripts/pi/version-evidence.json, which must include PiMinVersion.
#
# Run inside the devcontainer only, from the repository root:
#   scripts/pi/version-matrix.sh 0.99.2 1.0.0 1.0.1
#
# Each version is installed with npm into $PI_VERSIONS_DIR/<version> (default
# /tmp/pi-versions), with its own lockfile and npm cache and without install
# scripts. Nothing global is installed and no extension is ever imported: the
# contract only resolves paths, and its fixture extensions throw if loaded.
# Per version it runs the contract against dist/core and against the bundle the
# pi CLI executes, the Go test that holds Skillshare's lock against Pi's, and the
# Go test that has Pi resolve project settings this editor wrote.
set -euo pipefail

PACKAGE=@earendil-works/pi-coding-agent
VERSIONS_DIR=${PI_VERSIONS_DIR:-/tmp/pi-versions}
EVIDENCE=scripts/pi/version-evidence.json
[ $# -gt 0 ] || { echo "usage: $0 <version>..." >&2; exit 2; }
[ -f scripts/pi/phase0-contract.mjs ] || { echo "run from the repository root" >&2; exit 2; }

results=()
for version in "$@"; do
  [[ $version =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "not an exact version: $version" >&2; exit 2; }
  dir=$VERSIONS_DIR/$version
  root=$dir/node_modules/$PACKAGE
  if [ ! -f "$root/package.json" ]; then
    mkdir -p "$dir"
    [ -f "$dir/package.json" ] || printf '{"name":"pi-version-%s","private":true}\n' "$version" >"$dir/package.json"
    npm install --prefix "$dir" --cache "$VERSIONS_DIR/.npm-cache" --ignore-scripts --no-audit --no-fund --save-exact "$PACKAGE@$version" >/dev/null
  fi
  installed=$(node -p "require('$root/package.json').version")
  [ "$installed" = "$version" ] || { echo "$dir holds Pi $installed, not $version" >&2; exit 1; }
  integrity=$(node -p "require('$dir/package-lock.json').packages['node_modules/$PACKAGE'].integrity")

  checks=()
  for entry in core bundle; do
    out=$(PI_ENTRY=$entry PI_ROOT=$root node scripts/pi/phase0-contract.mjs 2>&1) && status=0 || status=$?
    summary=$(printf '%s\n' "$out" | tail -1)
    echo "pi $version contract/$entry: exit $status, $summary"
    checks+=("{\"name\":\"contract/$entry\",\"exit\":$status,\"summary\":$(node -p 'JSON.stringify(process.argv[1])' "$summary")}")
  done
  for check in native-lock:TestPiNativeLockHoldsAgainstPi project-native:TestPiProjectOverridesResolveInPi; do
    name=${check%%:*} test=${check#*:}
    out=$(PI_ROOT=$root go test ./internal/plugin -run "^$test\$" -count=1 -v 2>&1) && status=0 || status=$?
    # A skip (no PI_ROOT seen) is not a pass.
    if ! printf '%s\n' "$out" | grep -q -- "--- PASS: $test" && [ "$status" -eq 0 ]; then status=1; fi
    echo "pi $version $name: exit $status"
    checks+=("{\"name\":\"$name\",\"exit\":$status}")
  done

  passed=true
  for c in "${checks[@]}"; do [[ $c == *'"exit":0'* ]] || passed=false; done
  results+=("{\"version\":\"$version\",\"integrity\":\"$integrity\",\"passed\":$passed,\"checks\":[$(IFS=,; echo "${checks[*]}")]}")
done

node -e '
const [file, node, date, ...results] = process.argv.slice(1);
const doc = { package: "@earendil-works/pi-coding-agent", node, date, versions: results.map((r) => JSON.parse(r)) };
require("fs").writeFileSync(file, JSON.stringify(doc, null, 2) + "\n");
' "$EVIDENCE" "$(node --version)" "$(date -u +%Y-%m-%d)" "${results[@]}"
echo "wrote $EVIDENCE"
for r in "${results[@]}"; do [[ $r == *'"passed":true'* ]] || exit 1; done
