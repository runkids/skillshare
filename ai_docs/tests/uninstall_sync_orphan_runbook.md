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

Run inside devcontainer. Setup hook runs `ss init -g`.

## Steps

### 1. Install skills and verify manifest

```bash
ss install sickn33/antigravity-awesome-skills -s pdf-official,debugging-toolkit-smart-debug,tdd-workflow
ss install sickn33/antigravity-awesome-skills -s react-best-practices --into frontend --force
ss sync
ls ~/.config/skillshare/skills/
ls ~/.claude/skills/
```

Expected:
- exit_code: 0
- pdf-official
- debugging-toolkit-smart-debug
- tdd-workflow
- frontend

### 2. Replace symlinks with real directories (simulate copy-mode residue)

```bash
TARGET=~/.claude/skills
for skill in pdf-official debugging-toolkit-smart-debug tdd-workflow; do
  rm "$TARGET/$skill"
  mkdir -p "$TARGET/$skill"
  echo "# Copy residue" > "$TARGET/$skill/SKILL.md"
done
ls -la "$TARGET/pdf-official" "$TARGET/debugging-toolkit-smart-debug" "$TARGET/tdd-workflow"
```

Expected:
- exit_code: 0
- SKILL.md

### 3. Uninstall all and sync

```bash
ss uninstall pdf-official debugging-toolkit-smart-debug tdd-workflow react-best-practices --force
ss sync
```

Expected:
- exit_code: 0
- Uninstalled
- pruned

### 4. Verify orphan directories removed

```bash
TARGET=~/.claude/skills
for skill in pdf-official debugging-toolkit-smart-debug tdd-workflow frontend__react-best-practices; do
  [ ! -e "$TARGET/$skill" ] && echo "$skill: removed" || echo "$skill: STILL EXISTS (FAIL)"
done
```

Expected:
- exit_code: 0
- pdf-official: removed
- debugging-toolkit-smart-debug: removed
- tdd-workflow: removed
- frontend__react-best-practices: removed
- Not STILL EXISTS

### 5. User-created directories preserved

```bash
ss install sickn33/antigravity-awesome-skills -s pdf-official
ss sync
TARGET=~/.claude/skills
mkdir -p "$TARGET/my-custom-skill"
echo "# My custom" > "$TARGET/my-custom-skill/SKILL.md"
ss uninstall pdf-official --force
ss sync
[ -f "$TARGET/my-custom-skill/SKILL.md" ] && echo "user dir preserved" || echo "FAIL: user dir gone"
```

Expected:
- exit_code: 0
- user dir preserved
- Not FAIL

### 6. Dry-run does not modify manifest

```bash
ss install sickn33/antigravity-awesome-skills -s pdf-official
ss sync
TARGET=~/.claude/skills
MANIFEST="$TARGET/.skillshare-manifest.json"
cp "$MANIFEST" /tmp/manifest-before.json
ss sync --dry-run
cmp -s "$MANIFEST" /tmp/manifest-before.json && echo "manifest unchanged in dry-run" || echo "FAIL: manifest changed"
```

Expected:
- exit_code: 0
- manifest unchanged in dry-run
- Not FAIL

### 7. Exclude filter prunes managed real directories

```bash
# Clean up from previous steps
ss uninstall --all --force 2>/dev/null || true
ss sync

# Install two skills fresh
ss install sickn33/antigravity-awesome-skills -s pdf-official,tdd-workflow
ss sync

TARGET=~/.claude/skills
grep '"pdf-official"' "$TARGET/.skillshare-manifest.json" && echo "pdf-official in manifest"
grep '"tdd-workflow"' "$TARGET/.skillshare-manifest.json" && echo "tdd-workflow in manifest"

# Replace pdf-official symlink with real directory
rm "$TARGET/pdf-official"
mkdir -p "$TARGET/pdf-official"
echo "# residue" > "$TARGET/pdf-official/SKILL.md"

# Add exclude for pdf-official in claude target config
CONFIG="$HOME/.config/skillshare/config.yaml"
sed -i '/^    claude:/,/^    [^ ]/{/path:/a\        exclude:\n            - pdf-official
}' "$CONFIG"

ss sync

# pdf-official should be removed (excluded + was in manifest + real dir)
[ ! -e "$TARGET/pdf-official" ] && echo "pdf-official removed by exclude" || echo "FAIL: pdf-official still exists"

# tdd-workflow should still be linked
[ -L "$TARGET/tdd-workflow" ] && echo "tdd-workflow still linked" || echo "FAIL: tdd-workflow missing"

# pdf-official should NOT be in manifest
if ! grep -q '"pdf-official"' "$TARGET/.skillshare-manifest.json"; then
  echo "pdf-official disowned from manifest"
else
  echo "FAIL: pdf-official still in manifest"
fi
```

Expected:
- exit_code: 0
- pdf-official in manifest
- tdd-workflow in manifest
- pdf-official removed by exclude
- tdd-workflow still linked
- pdf-official disowned from manifest
- Not FAIL

### 8. Bulk install/uninstall stress test (original #45 scenario)

```bash
# Clean slate: remove exclude from config, uninstall all
ss uninstall --all --force 2>/dev/null || true
CONFIG="$HOME/.config/skillshare/config.yaml"
sed -i '/exclude:/d; /- pdf-official/d' "$CONFIG"
ss sync

# Remove leftover user dirs from step 5
rm -rf ~/.claude/skills/my-custom-skill

TARGET=~/.claude/skills

# Install a fresh batch
ss install sickn33/antigravity-awesome-skills -s pdf-official,tdd-workflow,debugging-toolkit-smart-debug,code-reviewer,debugger
ss install sickn33/antigravity-awesome-skills -s react-best-practices --into frontend --force
ss sync

# Count managed entries
MANAGED=$(jq '.managed | length' "$TARGET/.skillshare-manifest.json")
echo "Managed skills in manifest: $MANAGED"

# Replace some symlinks with real directories
for skill in pdf-official debugging-toolkit-smart-debug code-reviewer; do
  rm "$TARGET/$skill"
  mkdir -p "$TARGET/$skill"
  echo "# residue" > "$TARGET/$skill/SKILL.md"
done

# Uninstall everything
ss uninstall --all --force
ss sync

# Verify ALL orphans are cleaned up
for skill in pdf-official debugging-toolkit-smart-debug tdd-workflow frontend__react-best-practices code-reviewer debugger; do
  [ ! -e "$TARGET/$skill" ] && echo "$skill: removed" || echo "$skill: STILL EXISTS (FAIL)"
done

# Manifest should be empty
REMAINING=$(jq '.managed | length' "$TARGET/.skillshare-manifest.json")
echo "Remaining managed entries: $REMAINING (expected: 0)"
```

Expected:
- exit_code: 0
- regex: Managed skills in manifest: \d+
- pdf-official: removed
- debugging-toolkit-smart-debug: removed
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
