# Phase 3 — CONTENT INTEGRITY (hash pinning)
# Sourced by red_team_test.sh; requires _helpers.sh

phase "PHASE 3 — CONTENT INTEGRITY"

INTEGRITY_DIR="$SOURCE_DIR/integrity-skill"

# ── TC-25: Clean skill with correct hashes → no findings ──────

info "TC-25: clean skill passes integrity check"
create_skill "$INTEGRITY_DIR" "# Integrity test
Safe content for hash verification."

SKILL_HASH=$(shasum -a 256 "$INTEGRITY_DIR/SKILL.md" | awk '{print $1}')

cat > "$INTEGRITY_DIR/.skillshare-meta.json" <<META_EOF
{
  "source": "test/integrity",
  "type": "local",
  "installed_at": "2026-01-01T00:00:00Z",
  "file_hashes": {
    "SKILL.md": "sha256:$SKILL_HASH"
  }
}
META_EOF

ss_capture audit integrity-skill --format json
HAS_INTEGRITY=false
echo "$SS_OUTPUT" | jq -e '.results[0].findings[] | select(.pattern | startswith("content-"))' >/dev/null 2>&1 && HAS_INTEGRITY=true
if [ "$HAS_INTEGRITY" = false ]; then
  pass "TC-25: clean skill with correct hashes passes"
else
  fail "TC-25: unexpected integrity finding"
fi

# ── TC-26: Tamper file → content-tampered (MEDIUM) ────────────

info "TC-26: tampered file detected"
cat > "$INTEGRITY_DIR/SKILL.md" <<SKILL_EOF
---
name: integrity-skill
---
# Integrity test
MODIFIED — this content does not match the pinned hash.
SKILL_EOF

ss_capture audit integrity-skill --format json
assert_finding "TC-26: content-tampered detected as MEDIUM" "$SS_OUTPUT" "content-tampered" "MEDIUM"

# ── TC-27: Missing pinned file → content-missing (LOW) ────────

info "TC-27: missing pinned file detected"
create_skill "$INTEGRITY_DIR" "# Integrity test
Safe content for hash verification."
SKILL_HASH=$(shasum -a 256 "$INTEGRITY_DIR/SKILL.md" | awk '{print $1}')

cat > "$INTEGRITY_DIR/.skillshare-meta.json" <<META_EOF
{
  "source": "test/integrity",
  "type": "local",
  "installed_at": "2026-01-01T00:00:00Z",
  "file_hashes": {
    "SKILL.md": "sha256:$SKILL_HASH",
    "extras.md": "sha256:0000000000000000000000000000000000000000000000000000000000000000"
  }
}
META_EOF

ss_capture audit integrity-skill --format json
assert_finding "TC-27: content-missing detected as LOW" "$SS_OUTPUT" "content-missing" "LOW"

# ── TC-28: Unexpected file → content-unexpected (LOW) ─────────

info "TC-28: unexpected file detected"
cat > "$INTEGRITY_DIR/.skillshare-meta.json" <<META_EOF
{
  "source": "test/integrity",
  "type": "local",
  "installed_at": "2026-01-01T00:00:00Z",
  "file_hashes": {
    "SKILL.md": "sha256:$SKILL_HASH"
  }
}
META_EOF

echo "unexpected content" > "$INTEGRITY_DIR/sneaky.sh"

ss_capture audit integrity-skill --format json
assert_finding "TC-28: content-unexpected detected as LOW" "$SS_OUTPUT" "content-unexpected" "LOW"

# Clean up phase 3
rm -rf "$SOURCE_DIR"/integrity-skill
