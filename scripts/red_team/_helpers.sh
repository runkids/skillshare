# Red Team Test — Shared helpers
# Sourced by red_team_test.sh; do not run directly.

# ── Colors & counters ──────────────────────────────────────────────

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0
CURRENT_PHASE=""

pass() {
  printf "  ${GREEN}PASS${NC}: %s\n" "$1"
  TESTS_PASSED=$((TESTS_PASSED + 1))
}

fail() {
  printf "  ${RED}FAIL${NC}: %s\n" "$1"
  TESTS_FAILED=$((TESTS_FAILED + 1))
}

info() {
  printf "  ${YELLOW}INFO${NC}: %s\n" "$1"
}

phase() {
  CURRENT_PHASE="$1"
  printf "\n${BOLD}${CYAN}═══ %s ═══${NC}\n" "$1"
}

# ── Skill helpers ──────────────────────────────────────────────────

# Create a minimal skill directory with given content
# Usage: create_skill <dir> <content>
create_skill() {
  local dir="$1"
  local content="$2"
  mkdir -p "$dir"
  cat > "$dir/SKILL.md" <<SKILL_EOF
---
name: $(basename "$dir")
---
$content
SKILL_EOF
}

# Write or merge a skill entry into the centralized .metadata.json store.
# Usage: write_store_meta <source_dir> <skill_name> <json_entry>
# Example: write_store_meta "$SOURCE_DIR" "my-skill" '{"source":"test","file_hashes":{"SKILL.md":"sha256:abc"}}'
write_store_meta() {
  local source_dir="$1"
  local skill_name="$2"
  local entry_json="$3"
  local store_path="$source_dir/.metadata.json"

  if [ -f "$store_path" ]; then
    # Merge into existing store
    local tmp
    tmp=$(jq --arg name "$skill_name" --argjson entry "$entry_json" \
      '.entries[$name] = $entry' "$store_path")
    echo "$tmp" > "$store_path"
  else
    # Create new store
    jq -n --arg name "$skill_name" --argjson entry "$entry_json" \
      '{version:1, entries:{($name): $entry}}' > "$store_path"
  fi
}

# Run skillshare with isolated config.
# -g is placed after the subcommand to force global mode,
# preventing auto-detection of .skillshare/ in the working directory.
ss() {
  local cmd="$1"; shift
  SKILLSHARE_CONFIG="$CONFIG_PATH" "$BIN" "$cmd" -g "$@" 2>&1
}

# Run skillshare, capture output + exit code
# Sets: SS_OUTPUT, SS_EXIT, SS_STDERR
# stdout and stderr are captured separately so structured output (JSON/SARIF)
# is not polluted by warnings or progress output on stderr.
ss_capture() {
  local cmd="$1"; shift
  SS_OUTPUT=""
  SS_STDERR=""
  SS_EXIT=0
  local stderr_tmp="$TMPDIR_ROOT/ss_stderr.tmp"
  SS_OUTPUT="$(SKILLSHARE_CONFIG="$CONFIG_PATH" "$BIN" "$cmd" -g "$@" 2>"$stderr_tmp")" || SS_EXIT=$?
  SS_STDERR="$(cat "$stderr_tmp" 2>/dev/null)"
}

# ── Assertion helpers ──────────────────────────────────────────────

# Assert output contains a string (case-sensitive)
assert_contains() {
  local label="$1"
  local haystack="$2"
  local needle="$3"
  if echo "$haystack" | grep -qF "$needle"; then
    pass "$label"
  else
    fail "$label — expected to find: $needle"
    info "Got: $(echo "$haystack" | head -20)"
  fi
}

# Assert output does NOT contain a string
assert_not_contains() {
  local label="$1"
  local haystack="$2"
  local needle="$3"
  if echo "$haystack" | grep -qF "$needle"; then
    fail "$label — should NOT contain: $needle"
  else
    pass "$label"
  fi
}

# Assert exit code equals expected
assert_exit() {
  local label="$1"
  local expected="$2"
  local actual="$3"
  if [ "$actual" -eq "$expected" ]; then
    pass "$label"
  else
    fail "$label — expected exit=$expected, got exit=$actual"
  fi
}

# Assert audit JSON output contains a finding with given pattern and severity
# Usage: assert_finding <label> <json_output> <pattern> <severity>
assert_finding() {
  local label="$1"
  local json="$2"
  local pattern="$3"
  local severity="$4"
  if echo "$json" | jq -e ".results[0].findings[] | select(.pattern == \"$pattern\" and .severity == \"$severity\")" >/dev/null 2>&1; then
    pass "$label"
  else
    fail "$label — expected $severity $pattern"
    info "Findings: $(echo "$json" | jq -c '[.results[0].findings[].pattern]' 2>/dev/null || echo '(parse error)')"
  fi
}

# Assert a successful audit has no findings matching a jq predicate.
# Null findings is the CLI's valid representation of an empty findings slice.
assert_no_findings_matching() {
  local label="$1"
  local json="$2"
  local predicate="$3"
  if [ "$SS_EXIT" -ne 0 ]; then
    fail "$label — audit exited $SS_EXIT"
    return
  fi
  if ! jq -se '
    length == 1 and (.[0] |
      (.results | type == "array" and length > 0) and
      (.results[0] | type == "object" and has("findings")) and
      (.results[0].findings | . == null or type == "array"))
  ' <<< "$json" >/dev/null 2>&1; then
    fail "$label — invalid audit JSON or missing findings"
    return
  fi
  if jq -e "[.results[0].findings[]? | select($predicate)] | length == 0" <<< "$json" >/dev/null 2>&1; then
    pass "$label"
  else
    fail "$label — unexpected finding or invalid finding data"
  fi
}

# Assert audit JSON output does NOT contain a finding with given pattern.
# Usage: assert_no_finding <label> <json_output> <pattern>
assert_no_finding() {
  assert_no_findings_matching "$1" "$2" ".pattern == \"$3\""
}

# ── Git repo helpers ───────────────────────────────────────────────

# Create a bare git remote + cloned working copy, push initial clean commit.
# Usage: setup_tracked_repo <repo_name> <skill_content>
# Sets: REPO_REMOTE, REPO_LOCAL, REPO_WORK
setup_tracked_repo() {
  local name="$1"
  local content="$2"

  REPO_REMOTE="$TMPDIR_ROOT/remotes/${name}.git"
  REPO_LOCAL="$SOURCE_DIR/_${name}"
  REPO_WORK="$TMPDIR_ROOT/work/${name}"

  mkdir -p "$(dirname "$REPO_REMOTE")" "$(dirname "$REPO_WORK")"

  git init --bare "$REPO_REMOTE" >/dev/null 2>&1
  git clone "$REPO_REMOTE" "$REPO_LOCAL" >/dev/null 2>&1

  create_skill "$REPO_LOCAL/my-skill" "$content"
  (cd "$REPO_LOCAL" && git add -A && git commit -m "initial clean commit" >/dev/null 2>&1)
  (cd "$REPO_LOCAL" && git push origin HEAD >/dev/null 2>&1)

  git clone "$REPO_REMOTE" "$REPO_WORK" >/dev/null 2>&1
}

# Push a malicious update to the remote via the work clone
push_malicious_update() {
  local name="$1"
  local content="$2"
  local work="$TMPDIR_ROOT/work/${name}"

  cat > "$work/my-skill/SKILL.md" <<SKILL_EOF
---
name: my-skill
---
$content
SKILL_EOF
  (cd "$work" && git add -A && git commit -m "inject malicious content" >/dev/null 2>&1)
  (cd "$work" && git push origin HEAD >/dev/null 2>&1)
}

# Push a clean update to the remote via the work clone
push_clean_update() {
  local name="$1"
  local content="$2"
  local work="$TMPDIR_ROOT/work/${name}"

  cat > "$work/my-skill/SKILL.md" <<SKILL_EOF
---
name: my-skill
---
$content
SKILL_EOF
  (cd "$work" && git add -A && git commit -m "clean update" >/dev/null 2>&1)
  (cd "$work" && git push origin HEAD >/dev/null 2>&1)
}
