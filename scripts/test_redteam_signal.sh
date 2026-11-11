#!/bin/bash
# Verify red_team_test.sh has real detection signal.
#
# Strategy:
# 1) Build a private baseline binary and run red team (must pass).
# 2) Build with a Go overlay that bypasses the HIGH/CRITICAL gate.
# 3) Require the mutated run to fail the single and batch update gate tests.
# Source and bin/skillshare are never modified, even on failure or interruption.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
GATE_GO="$PROJECT_ROOT/internal/install/install_audit.go"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/redteam-signal.XXXXXX")"
TEST_BIN="$TMP_DIR/skillshare"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

echo "==> [1/3] Baseline: build private binary and run red team (should pass)"
(cd "$PROJECT_ROOT" && go build -o "$TEST_BIN" ./cmd/skillshare)
SKILLSHARE_TEST_BINARY="$TEST_BIN" "$SCRIPT_DIR/red_team_test.sh"

echo "==> [2/3] Build mutation: bypass HIGH/CRITICAL gate through a Go overlay"
# Keep the mutation in a private copy, not the working tree.
perl -0pe 's@if !scanResult\.HasSeverityAtOrAbove\(threshold\) \{@if true { // MUTATION: bypass rollback gate@g' \
  "$GATE_GO" > "$TMP_DIR/install_audit.go"
MUTATION_COUNT="$(grep -c 'MUTATION: bypass rollback gate' "$TMP_DIR/install_audit.go" || true)"
if [ "$MUTATION_COUNT" -ne 1 ]; then
  echo "ERROR: expected to inject 1 mutation point, got $MUTATION_COUNT"
  exit 2
fi
jq -n --arg original "$GATE_GO" --arg mutated "$TMP_DIR/install_audit.go" \
  '{Replace: {($original): $mutated}}' > "$TMP_DIR/overlay.json"
(cd "$PROJECT_ROOT" && go build -overlay "$TMP_DIR/overlay.json" -o "$TEST_BIN" ./cmd/skillshare)

echo "==> [3/3] Run mutated red team (must fail the single and batch update gates)"
MUTATED_EXIT=0
SKILLSHARE_TEST_BINARY="$TEST_BIN" "$SCRIPT_DIR/red_team_test.sh" > "$TMP_DIR/output.log" 2>&1 || MUTATED_EXIT=$?
cat "$TMP_DIR/output.log"
perl -pe 's/\e\[[0-9;]*m//g' "$TMP_DIR/output.log" > "$TMP_DIR/plain.log"

if [ "$MUTATED_EXIT" -ne 1 ]; then
  echo "ERROR: expected completed test failure (exit=1), got exit=$MUTATED_EXIT"
  exit 1
fi
for expected in 'FAIL: TC-21a:' 'FAIL: TC-29:' 'Red Team Test Results'; do
  if ! grep -qF "$expected" "$TMP_DIR/plain.log"; then
    echo "ERROR: expected mutation signal not observed: $expected"
    exit 1
  fi
done

echo "PASS: test-redteam signal verified (baseline pass + expected gate failures)"
