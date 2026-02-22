# Phase 5 - ADVANCED ADVERSARIAL CASES
# Sourced by red_team_test.sh; requires _helpers.sh

phase "PHASE 5 - ADVANCED ADVERSARIAL CASES"

# -- TC-31: file_hashes path traversal keys are ignored ----------

info "TC-31: file_hashes traversal keys should be ignored"
TRAVERSAL_DIR="$SOURCE_DIR/traversal-skill"
create_skill "$TRAVERSAL_DIR" "# Traversal test
Safe content for traversal hardening checks."

TRAVERSAL_HASH=$(shasum -a 256 "$TRAVERSAL_DIR/SKILL.md" | awk '{print $1}')
echo "TOP SECRET" > "$TMPDIR_ROOT/secret.txt"

cat > "$TRAVERSAL_DIR/.skillshare-meta.json" <<META_EOF
{
  "source": "test/traversal",
  "type": "local",
  "installed_at": "2026-01-01T00:00:00Z",
  "file_hashes": {
    "SKILL.md": "sha256:$TRAVERSAL_HASH",
    "../../../secret.txt": "sha256:0000",
    "../../secret.txt": "sha256:0000",
    "/etc/passwd": "sha256:0000"
  }
}
META_EOF

ss_capture audit traversal-skill --json
if echo "$SS_OUTPUT" | jq -e '
  .results[0].findings[] |
  select(.file == "../../../secret.txt" or .file == "../../secret.txt" or .file == "/etc/passwd")
' >/dev/null 2>&1; then
  fail "TC-31: traversal/absolute keys should not produce findings"
else
  pass "TC-31: traversal/absolute hash keys ignored"
fi

# -- TC-32: symlink to external file is surfaced -----------------

info "TC-32: external symlink should be surfaced as unexpected content"
SYMLINK_DIR="$SOURCE_DIR/symlink-skill"
create_skill "$SYMLINK_DIR" "# Symlink test
Safe content only."

SYMLINK_HASH=$(shasum -a 256 "$SYMLINK_DIR/SKILL.md" | awk '{print $1}')
cat > "$SYMLINK_DIR/.skillshare-meta.json" <<META_EOF
{
  "source": "test/symlink",
  "type": "local",
  "installed_at": "2026-01-01T00:00:00Z",
  "file_hashes": {
    "SKILL.md": "sha256:$SYMLINK_HASH"
  }
}
META_EOF

echo "outside file content" > "$TMPDIR_ROOT/outside.txt"
ln -s "$TMPDIR_ROOT/outside.txt" "$SYMLINK_DIR/external-link.txt"

ss_capture audit symlink-skill --json
if echo "$SS_OUTPUT" | jq -e '
  .results[0].findings[] |
  select(.pattern == "content-unexpected" and .file == "external-link.txt" and .severity == "LOW")
' >/dev/null 2>&1; then
  pass "TC-32: external symlink flagged as content-unexpected"
else
  fail "TC-32: expected LOW content-unexpected for external-link.txt"
fi

# -- TC-33: weird filename should not break --diff parsing -------

info "TC-33: weird filename should not break update --diff summary"
setup_tracked_repo "diff-weird" "# Diff parser baseline
Safe initial content."

WEIRD_WORK="$TMPDIR_ROOT/work/diff-weird"
WEIRD_FILE=$'weird\tname.sh'
cat > "$WEIRD_WORK/$WEIRD_FILE" <<FILE_EOF
#!/bin/sh
echo "safe"
FILE_EOF
(cd "$WEIRD_WORK" && git add -A && git commit -m "add weird filename" >/dev/null 2>&1)
(cd "$WEIRD_WORK" && git push origin HEAD >/dev/null 2>&1)

ss_capture update _diff-weird --diff --skip-audit
assert_exit "TC-33a: update with weird filename exits zero" 0 "$SS_EXIT"
assert_contains "TC-33b: --diff box rendered" "$SS_OUTPUT" "Files Changed"
assert_contains "TC-33c: weird filename appears in summary" "$SS_OUTPUT" "weird"

# Clean up phase 5
rm -rf "$SOURCE_DIR"/traversal-skill "$SOURCE_DIR"/symlink-skill "$SOURCE_DIR"/_diff-weird
rm -rf "$TMPDIR_ROOT/remotes/diff-weird.git" "$TMPDIR_ROOT/work/diff-weird"
