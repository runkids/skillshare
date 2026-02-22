#!/bin/bash
# Red Team E2E Test — Supply Chain Security
#
# Validates that skillshare detects and blocks supply-chain attacks:
#   - All audit patterns detected at correct severity (CRITICAL → INFO)
#   - Risk floor prevents severity downgrading
#   - Update auto-blocks + rolls back malicious repo updates
#   - Content hash integrity catches post-install tampering
#   - Batch update fails non-zero when any repo is blocked
#
# Run locally:
#   make test-redteam
#
# Run in devcontainer:
#   make build && ./scripts/red_team_test.sh
#
# Run in playground (after `make playground`):
#   ./scripts/sandbox.sh shell
#   ./scripts/red_team_test.sh
#
# Requires: git, jq, go (or pre-built skillshare binary)
# Override binary: SKILLSHARE_TEST_BINARY=/path/to/skillshare ./scripts/red_team_test.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
RED_TEAM_DIR="$SCRIPT_DIR/red_team"

# Resolve binary: env override > bin/skillshare > PATH lookup
if [ -n "${SKILLSHARE_TEST_BINARY:-}" ]; then
  BIN="$SKILLSHARE_TEST_BINARY"
elif [ -f "$PROJECT_ROOT/bin/skillshare" ]; then
  BIN="$PROJECT_ROOT/bin/skillshare"
elif command -v skillshare &>/dev/null; then
  BIN="$(command -v skillshare)"
else
  BIN="$PROJECT_ROOT/bin/skillshare"
fi

# Load shared helpers (colors, pass/fail, assert_*, git repo helpers)
source "$RED_TEAM_DIR/_helpers.sh"

# ── Temp dir + cleanup ─────────────────────────────────────────────

TMPDIR_ROOT=""
cleanup() {
  if [ -n "$TMPDIR_ROOT" ] && [ -d "$TMPDIR_ROOT" ]; then
    rm -rf "$TMPDIR_ROOT"
  fi
}
trap cleanup EXIT

# ══════════════════════════════════════════════════════════════════
# PHASE 0 — SETUP
# ══════════════════════════════════════════════════════════════════

phase "PHASE 0 — SETUP"

# Build binary if not present (skip in read-only environments like playground)
if [ ! -f "$BIN" ]; then
  if [ -w "$PROJECT_ROOT" ]; then
    info "Building skillshare binary..."
    (cd "$PROJECT_ROOT" && make build >/dev/null 2>&1)
  fi
fi

if [ ! -f "$BIN" ]; then
  printf "${RED}ERROR${NC}: Binary not found at $BIN\n"
  printf "  Either build first (make build) or set SKILLSHARE_TEST_BINARY=...\n"
  exit 2
fi
pass "Binary exists at $BIN"

# Verify jq is available
if ! command -v jq &>/dev/null; then
  printf "${RED}ERROR${NC}: jq is required but not installed\n"
  exit 2
fi
pass "jq available"

# Create isolated temp environment
TMPDIR_ROOT="$(mktemp -d)"
SOURCE_DIR="$TMPDIR_ROOT/source"
CONFIG_PATH="$TMPDIR_ROOT/config.yaml"
mkdir -p "$SOURCE_DIR"

# Use an isolated git global config to avoid mutating user-level settings.
# This ensures commit identity is available in clean containers and CI.
GIT_CONFIG_GLOBAL="$TMPDIR_ROOT/gitconfig"
export GIT_CONFIG_GLOBAL
git config --global user.name "Red Team Test"
git config --global user.email "redteam@test.local"
info "Using isolated git config: $GIT_CONFIG_GLOBAL"

cat > "$CONFIG_PATH" <<EOF
source: $SOURCE_DIR
targets: {}
EOF

pass "Isolated environment created at $TMPDIR_ROOT"

# ══════════════════════════════════════════════════════════════════
# RUN PHASES
# ══════════════════════════════════════════════════════════════════

source "$RED_TEAM_DIR/phase1_recon.sh"
source "$RED_TEAM_DIR/phase2_attack.sh"
source "$RED_TEAM_DIR/phase3_integrity.sh"
source "$RED_TEAM_DIR/phase4_batch.sh"
source "$RED_TEAM_DIR/phase5_advanced.sh"

# ══════════════════════════════════════════════════════════════════
# SUMMARY
# ══════════════════════════════════════════════════════════════════

echo ""
printf "${BOLD}════════════════════════════════════════════${NC}\n"
printf "${BOLD}  Red Team Test Results${NC}\n"
printf "${BOLD}════════════════════════════════════════════${NC}\n"
printf "  ${GREEN}Passed: %d${NC}\n" "$TESTS_PASSED"
printf "  ${RED}Failed: %d${NC}\n" "$TESTS_FAILED"
printf "${BOLD}════════════════════════════════════════════${NC}\n"

if [ "$TESTS_FAILED" -gt 0 ]; then
  exit 1
fi
