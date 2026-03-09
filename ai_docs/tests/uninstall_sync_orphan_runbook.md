# CLI E2E Runbook: Uninstall → Sync Orphan Directory Cleanup

Validates that `sync` correctly prunes orphan real directories after `uninstall`,
using the merge-mode manifest to identify skillshare-managed entries.

**Origin**: Issue #45 — reported after installing all 879 skills from
`sickn33/antigravity-awesome-skills`, then uninstalling.

## Scope

- Merge-mode manifest written on sync
- Orphan real directories (non-symlinks) pruned via manifest
- User-created directories preserved (not in manifest)
- Copy-mode residue directories pruned after uninstall
- Include/exclude filter changes prune managed real directories

## Environment

Run inside devcontainer.
If `ss` alias is unavailable, replace `ss` with `skillshare`.

## Optional: use `ssenv` for isolated HOME

```bash
ssnew orphan-test
```

## Steps

### 1. Setup: install from antigravity repo and sync

Install a batch of skills (the same repo that triggered #45):

```bash
ss init
ss install sickn33/antigravity-awesome-skills/skills/pdf-official --name pdf
ss install sickn33/antigravity-awesome-skills/skills/debugging-toolkit-smart-debug --name smart-debug
ss install sickn33/antigravity-awesome-skills/skills/tdd-workflow
ss install sickn33/antigravity-awesome-skills/skills/react-best-practices --into frontend
ss sync
```

Expected:
- exit_code: 0

Verify manifest exists in at least one target:

```bash
cat ~/.claude/skills/.skillshare-manifest.json
```

Expected:
- exit_code: 0
- "pdf"
- "smart-debug"
- "tdd-workflow"
- "frontend__react-best-practices"

### 2. Replace symlinks with real directories (simulate copy-mode residue)

This simulates what happens when a target previously used copy mode,
or when symlinks were replaced by real directories for any reason:

```bash
TARGET=~/.claude/skills

for skill in pdf smart-debug tdd-workflow; do
  rm "$TARGET/$skill"
  mkdir -p "$TARGET/$skill"
  echo "# Copy residue" > "$TARGET/$skill/SKILL.md"
done

# Verify they are real directories, not symlinks
ls -la "$TARGET/pdf" "$TARGET/smart-debug" "$TARGET/tdd-workflow"
```

Expected:
- exit_code: 0
- SKILL.md

### 3. Uninstall all skills

```bash
ss uninstall pdf smart-debug tdd-workflow react-best-practices
```

Expected:
- exit_code: 0
- Uninstalled

### 4. Sync and verify orphan cleanup

```bash
ss sync
```

Expected:
- exit_code: 0
- pruned
- Not unknown directory (not from skillshare), kept

Verify:

```bash
TARGET=~/.claude/skills
for skill in pdf smart-debug tdd-workflow frontend__react-best-practices; do
  [ ! -e "$TARGET/$skill" ] && echo "$skill: removed" || echo "$skill: STILL EXISTS (FAIL)"
done
```

Expected:
- exit_code: 0
- pdf: removed
- smart-debug: removed
- tdd-workflow: removed
- frontend__react-best-practices: removed
- Not STILL EXISTS

```bash
cat ~/.claude/skills/.skillshare-manifest.json
```

Expected:
- exit_code: 0

### 5. Verify user-created directories are preserved

```bash
# Re-install one skill
ss install sickn33/antigravity-awesome-skills/skills/pdf-official --name pdf
ss sync

# Create a user directory (never managed by skillshare)
TARGET=~/.claude/skills
mkdir -p "$TARGET/my-custom-skill"
echo "# My custom" > "$TARGET/my-custom-skill/SKILL.md"

# Uninstall and sync
ss uninstall pdf
ss sync
```

Expected:
- exit_code: 0
- my-custom-skill

Verify:

```bash
[ -f ~/.claude/skills/my-custom-skill/SKILL.md ] && echo "user dir preserved" || echo "FAIL"
```

Expected:
- exit_code: 0
- user dir preserved
- Not FAIL

### 6. Verify dry-run does not write/modify manifest

```bash
TARGET=~/.claude/skills
ss install sickn33/antigravity-awesome-skills/skills/pdf-official --name pdf
ss sync

MANIFEST="$TARGET/.skillshare-manifest.json"
cp "$MANIFEST" /tmp/manifest-before.json

ss sync --dry-run

cmp -s "$MANIFEST" /tmp/manifest-before.json && echo "manifest unchanged in dry-run" || echo "FAIL"
```

Expected:
- exit_code: 0
- manifest unchanged in dry-run
- Not FAIL

### 7. Verify exclude filter prunes managed real directories

This tests the edge case where a filter change should clean up
previously-managed entries even if they are real directories:

```bash
TARGET=~/.claude/skills

# Start clean
ss uninstall pdf 2>/dev/null; ss sync

# Install two skills and sync
ss install sickn33/antigravity-awesome-skills/skills/pdf-official --name pdf
ss install sickn33/antigravity-awesome-skills/skills/tdd-workflow
ss sync

# Confirm both in manifest
grep '"pdf"' "$TARGET/.skillshare-manifest.json" && echo "pdf in manifest"
grep '"tdd-workflow"' "$TARGET/.skillshare-manifest.json" && echo "tdd-workflow in manifest"

# Replace pdf symlink with real directory
rm "$TARGET/pdf"
mkdir -p "$TARGET/pdf"
echo "# residue" > "$TARGET/pdf/SKILL.md"

# Add exclude for pdf in config (edit the claude target):
CONFIG=$(skillshare doctor 2>/dev/null | grep "Config:" | awk '{print $2}')
# Add under targets → claude → exclude: ["pdf"]
# (manual edit or use sed)

ss sync

# pdf should be removed (excluded + was in manifest + real dir)
[ ! -e "$TARGET/pdf" ] && echo "pdf removed by exclude" || echo "FAIL: pdf still exists"

# tdd-workflow should still be linked
[ -L "$TARGET/tdd-workflow" ] && echo "tdd-workflow still linked" || echo "FAIL"

# pdf should NOT be in manifest
if ! grep -q '"pdf"' "$TARGET/.skillshare-manifest.json"; then
  echo "pdf disowned from manifest"
else
  echo "FAIL: pdf still in manifest"
fi
```

Expected:
- exit_code: 0
- pdf in manifest
- tdd-workflow in manifest
- pdf removed by exclude
- tdd-workflow still linked
- pdf disowned from manifest
- Not FAIL

### 8. Bulk install/uninstall stress test (original #45 scenario)

This approximates the original issue: many skills installed then removed.

```bash
TARGET=~/.claude/skills

# Install a larger batch
ss install sickn33/antigravity-awesome-skills/skills/pdf-official --name pdf
ss install sickn33/antigravity-awesome-skills/skills/tdd-workflow
ss install sickn33/antigravity-awesome-skills/skills/debugging-toolkit-smart-debug --name smart-debug
ss install sickn33/antigravity-awesome-skills/skills/react-best-practices --into frontend
ss install sickn33/antigravity-awesome-skills/skills/code-reviewer
ss install sickn33/antigravity-awesome-skills/skills/debugger
ss sync

# Count managed entries in manifest
MANAGED=$(jq '.managed | length' "$TARGET/.skillshare-manifest.json")
echo "Managed skills in manifest: $MANAGED"

# Replace some symlinks with real directories
for skill in pdf smart-debug code-reviewer; do
  rm "$TARGET/$skill"
  mkdir -p "$TARGET/$skill"
  echo "# residue" > "$TARGET/$skill/SKILL.md"
done

# Uninstall everything
ss uninstall pdf smart-debug tdd-workflow react-best-practices code-reviewer debugger
ss sync

# Verify ALL orphans are cleaned up — no "unknown directory, kept" warnings
for skill in pdf smart-debug tdd-workflow frontend__react-best-practices code-reviewer debugger; do
  [ ! -e "$TARGET/$skill" ] && echo "$skill: removed" || echo "$skill: STILL EXISTS (FAIL)"
done

# Manifest should be empty
REMAINING=$(jq '.managed | length' "$TARGET/.skillshare-manifest.json")
echo "Remaining managed entries: $REMAINING (expected: 0)"
```

Expected:
- exit_code: 0
- regex: Managed skills in manifest: \d+
- pdf: removed
- smart-debug: removed
- tdd-workflow: removed
- frontend__react-best-practices: removed
- code-reviewer: removed
- debugger: removed
- Not STILL EXISTS
- Remaining managed entries: 0 (expected: 0)

## Pass Criteria

- [ ] Manifest written after merge sync (contains synced skill names)
- [ ] Orphan real directories in manifest → removed by sync
- [ ] Nested skill orphans (e.g. `frontend__react-best-practices`) → removed
- [ ] User-created directories (not in manifest) → preserved with warning
- [ ] Manifest cleaned after prune (removed entries deleted from manifest)
- [ ] `--dry-run` does not write manifest
- [ ] Exclude filter prunes managed real directories (not just symlinks)
- [ ] Bulk install → bulk uninstall → sync leaves no orphan directories
