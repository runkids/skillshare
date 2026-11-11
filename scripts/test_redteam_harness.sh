#!/bin/bash
# Regression tests for red-team assertions and mutation-run isolation.
# Run inside the devcontainer: bash scripts/test_redteam_harness.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/red_team/_helpers.sh"

check_absence() {
  local label="$1" json="$2" exit_code="$3" expected="$4"
  TESTS_PASSED=0
  TESTS_FAILED=0
  SS_EXIT="$exit_code"
  assert_no_finding "$label" "$json" "prompt-injection" >/dev/null
  if [ "$TESTS_FAILED" -ne "$expected" ]; then
    echo "FAIL: $label — expected $expected failures, got $TESTS_FAILED"
    exit 1
  fi
}

check_absence "empty findings" '{"results":[{"findings":[]}]}' 0 0
check_absence "null findings" '{"results":[{"findings":null}]}' 0 0
check_absence "finding present" '{"results":[{"findings":[{"pattern":"prompt-injection"}]}]}' 0 1
check_absence "empty output" '' 0 1
check_absence "malformed JSON" 'not JSON' 0 1
check_absence "missing result" '{}' 0 1
check_absence "empty results" '{"results":[]}' 0 1
check_absence "multiple documents" '{"results":[{"findings":[]}]} {}' 0 1
check_absence "missing findings" '{"results":[{}]}' 0 1
check_absence "invalid findings" '{"results":[{"findings":"broken"}]}' 0 1
check_absence "CLI failure" '{"results":[{"findings":[]}]}' 2 1

echo "PASS: negative assertions reject invalid output and CLI failures"

# Use a disposable project with fake build/test commands, never mutate real source.
TEST_ROOT="$(mktemp -d)"
trap 'rm -rf "$TEST_ROOT"' EXIT
mkdir -p "$TEST_ROOT/project/scripts" "$TEST_ROOT/project/internal/install" "$TEST_ROOT/project/bin" "$TEST_ROOT/tools"
cp "$SCRIPT_DIR/test_redteam_signal.sh" "$SCRIPT_DIR/test_redteam_rules_signal.sh" "$TEST_ROOT/project/scripts/"
printf '%s\n' 'if !scanResult.HasSeverityAtOrAbove(threshold) {' > "$TEST_ROOT/project/internal/install/install_audit.go"
printf '%s\n' 'original binary' > "$TEST_ROOT/project/bin/skillshare"
cp "$TEST_ROOT/project/internal/install/install_audit.go" "$TEST_ROOT/original.go"
cp "$TEST_ROOT/project/bin/skillshare" "$TEST_ROOT/original.bin"

cat > "$TEST_ROOT/tools/go" <<'EOF'
#!/bin/bash
set -euo pipefail
printf '%s\n' "$*" >> "$BUILD_LOG"
while [ "$#" -gt 0 ]; do
  if [ "$1" = -o ]; then
    printf '%s\n' 'test binary' > "$2"
    break
  fi
  shift
done
EOF
cat > "$TEST_ROOT/tools/make" <<'EOF'
#!/bin/bash
exit 0
EOF
cat > "$TEST_ROOT/project/scripts/red_team_test.sh" <<'EOF'
#!/bin/bash
set -euo pipefail
if [ ! -f "$RUN_MARKER" ]; then
  touch "$RUN_MARKER"
  exit 0
fi
case "$RUN_MODE" in
  expected)
    printf '\033[31mFAIL\033[0m: TC-21a: update exits non-zero\n'
    echo 'FAIL: TC-29: update --all exits non-zero with malicious repo'
    echo '  Red Team Test Results'
    exit 1
    ;;
  unrelated) echo 'tool missing'; exit 2 ;;
  partial) echo 'FAIL: TC-21a: update exits non-zero'; exit 1 ;;
  unexpected-pass) exit 0 ;;
esac
EOF
# Fixture permissions are private to this disposable test directory.
chmod +x "$TEST_ROOT/tools/go" "$TEST_ROOT/tools/make" "$TEST_ROOT/project/scripts/red_team_test.sh" "$TEST_ROOT/project/bin/skillshare"

for script in test_redteam_signal.sh test_redteam_rules_signal.sh; do
for mode in expected unrelated partial unexpected-pass; do
  marker="$TEST_ROOT/$script-$mode.marker"
  if [ "$script" = test_redteam_rules_signal.sh ]; then
    touch "$marker"
  fi
  status=0
  PATH="$TEST_ROOT/tools:$PATH" BUILD_LOG="$TEST_ROOT/build.log" \
    RUN_MARKER="$marker" RUN_MODE="$mode" \
    bash "$TEST_ROOT/project/scripts/$script" > "$TEST_ROOT/$mode.log" 2>&1 || status=$?
  if [ "$mode" = expected ]; then
    if [ "$status" -ne 0 ]; then
      cat "$TEST_ROOT/$mode.log"
      echo 'FAIL: expected mutation signal rejected'
      exit 1
    fi
  elif [ "$status" -eq 0 ]; then
    echo "FAIL: $mode incorrectly accepted as mutation signal"
    exit 1
  fi
  cmp "$TEST_ROOT/original.go" "$TEST_ROOT/project/internal/install/install_audit.go"
  cmp "$TEST_ROOT/original.bin" "$TEST_ROOT/project/bin/skillshare"
done
done

echo 'PASS: mutation checks require gate failures and leave source/shared binary unchanged'
