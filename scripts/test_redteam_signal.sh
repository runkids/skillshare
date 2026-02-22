#!/bin/bash
# Verify red_team_test.sh has real detection signal.
#
# Strategy:
# 1) Run baseline red team test (must pass).
# 2) Introduce a temporary mutation that bypasses HIGH/CRITICAL rollback gate.
# 3) Rebuild binary and run red team test again (must fail).
# 4) Restore original source and rebuild.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
UPDATE_GO="$PROJECT_ROOT/cmd/skillshare/update.go"
BACKUP_FILE=""
BACKUP_READY=false
RESTORED=false

backup_source() {
  BACKUP_FILE="$(mktemp "${TMPDIR:-/tmp}/update.go.backup.XXXXXX")"
  cp "$UPDATE_GO" "$BACKUP_FILE"
  BACKUP_READY=true
}

restore_source() {
  if [ "$RESTORED" = false ] && [ "$BACKUP_READY" = true ] && [ -f "$BACKUP_FILE" ]; then
    cp "$BACKUP_FILE" "$UPDATE_GO"
    RESTORED=true
  fi
  if [ -n "$BACKUP_FILE" ]; then
    rm -f "$BACKUP_FILE"
  fi
}

cleanup() {
  restore_source
}
trap cleanup EXIT

echo "==> [1/4] Baseline: running make test-redteam (should pass)"
(cd "$PROJECT_ROOT" && make test-redteam)

echo "==> [2/4] Injecting mutation: bypass rollback on HIGH/CRITICAL findings"
backup_source

perl -0pi -e 's@return fmt\.Errorf\("security audit failed[^"]*rolled back \(use --skip-audit to bypass\): %w", normalizedThreshold, audit\.ErrBlocked\)@return nil // MUTATION: bypass rollback gate@g' "$UPDATE_GO"

if command -v rg >/dev/null 2>&1; then
  MUTATION_COUNT="$(rg -c "MUTATION: bypass rollback gate" "$UPDATE_GO" || true)"
else
  MUTATION_COUNT="$(grep -c "MUTATION: bypass rollback gate" "$UPDATE_GO" || true)"
fi
MUTATION_COUNT="${MUTATION_COUNT:-0}"
if ! [[ "$MUTATION_COUNT" =~ ^[0-9]+$ ]]; then
  echo "ERROR: mutation count is not numeric: $MUTATION_COUNT"
  exit 2
fi
if [ "$MUTATION_COUNT" -lt 2 ]; then
  echo "ERROR: expected to inject 2 mutation points, got $MUTATION_COUNT"
  exit 2
fi

echo "==> [3/4] Rebuild mutated binary and run red team (should fail)"
(cd "$PROJECT_ROOT" && go build -o bin/skillshare ./cmd/skillshare)

set +e
(cd "$PROJECT_ROOT" && ./scripts/red_team_test.sh)
MUTATED_EXIT=$?
set -e

if [ "$MUTATED_EXIT" -eq 0 ]; then
  echo "ERROR: mutated run unexpectedly passed; red team signal check failed"
  exit 1
fi
echo "PASS: mutated run failed as expected (exit=$MUTATED_EXIT)"

echo "==> [4/4] Restoring original source and rebuilding"
restore_source
(cd "$PROJECT_ROOT" && go build -o bin/skillshare ./cmd/skillshare)

echo "PASS: test-redteam signal verified (baseline pass + mutation fail)"
