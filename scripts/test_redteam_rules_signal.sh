#!/bin/bash
# Verify red_team_test.sh has rule-level detection signal.
#
# Strategy:
# 1) Run baseline red team test (must pass).
# 2) Re-run red team with critical builtin prompt-injection rules disabled
#    via a temporary audit-rules.yaml injected by a wrapper binary.
# 3) Assert the run fails and includes expected gate-related failures.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
REAL_BIN="$PROJECT_ROOT/bin/skillshare"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/redteam-rules-signal.XXXXXX")"
WRAPPER_BIN="$TMP_DIR/skillshare-wrapper"

cleanup() {
  rm -rf "$TMP_DIR"
}
trap cleanup EXIT

echo "==> [1/3] Baseline: running make test-redteam (should pass)"
(cd "$PROJECT_ROOT" && make test-redteam)

if [ ! -x "$REAL_BIN" ]; then
  echo "ERROR: expected binary at $REAL_BIN after baseline run"
  exit 2
fi

cat >"$WRAPPER_BIN" <<'WRAP_EOF'
#!/bin/bash
set -euo pipefail

if [ -n "${SKILLSHARE_CONFIG:-}" ]; then
  RULES_FILE="$(dirname "$SKILLSHARE_CONFIG")/audit-rules.yaml"
  cat >"$RULES_FILE" <<'RULES_EOF'
rules:
  - id: prompt-injection-0
    enabled: false
  - id: prompt-injection-1
    enabled: false
RULES_EOF
fi

exec "$SKILLSHARE_REAL_BINARY" "$@"
WRAP_EOF
chmod +x "$WRAPPER_BIN"

echo "==> [2/3] Run red team with builtin prompt-injection rules disabled (should fail)"
set +e
MUTATED_OUTPUT="$(
  cd "$PROJECT_ROOT" && \
  SKILLSHARE_REAL_BINARY="$REAL_BIN" \
  SKILLSHARE_TEST_BINARY="$WRAPPER_BIN" \
  ./scripts/red_team_test.sh 2>&1
)"
MUTATED_EXIT=$?
set -e

printf "%s\n" "$MUTATED_OUTPUT"

SANITIZED_OUTPUT="$(printf "%s\n" "$MUTATED_OUTPUT" | perl -pe 's/\e\[[0-9;]*m//g')"

if [ "$MUTATED_EXIT" -eq 0 ]; then
  echo "ERROR: rules-disabled run unexpectedly passed; signal check failed"
  exit 1
fi

if ! printf "%s\n" "$SANITIZED_OUTPUT" | grep -qF "FAIL: TC-21a"; then
  echo "ERROR: expected TC-21a failure was not observed"
  exit 1
fi

if ! printf "%s\n" "$SANITIZED_OUTPUT" | grep -qF "FAIL: TC-29"; then
  echo "ERROR: expected TC-29 failure was not observed"
  exit 1
fi

echo "==> [3/3] PASS: rules-level signal verified (baseline pass + rules-disabled fail)"
