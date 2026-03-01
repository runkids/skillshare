# CLI E2E Runbook: Symlinked Source/Target Directories

Validates that all commands (sync, status, diff, list, collect, update, uninstall,
reconcile) work correctly when source and/or target directories are symlinks —
the "dotfiles manager" scenario.

## Scope

- Source directory is a symlink (dotfiles manager pointing `~/.config/skillshare/skills/` elsewhere)
- Target directory is a symlink (dotfiles manager pointing `~/.claude/skills/` elsewhere)
- Both source and target are symlinks simultaneously
- Chained symlinks (link → link → real dir)
- Merge mode sync through symlinks
- Copy mode sync through symlinks
- `ss status`, `ss diff`, `ss list` through symlinks
- `ss collect` (pull local skills) through symlinked target
- `ss update --all` discovers skills through symlinked source
- `ss uninstall` resolves skills through symlinked source
- `ss uninstall --group` walks group dirs through symlinked source
- Reconcile (global) detects installed skills through symlinked source
- Idempotency: re-sync doesn't break existing symlinks

## Environment

Run inside devcontainer with ssenv isolation.

## Step 0: Setup — Create symlinked source directory

```bash
# Create the REAL skills directory in a "dotfiles" location
REAL_SOURCE="$HOME/dotfiles/skillshare-skills"
mkdir -p "$REAL_SOURCE"

# Create test skills in the REAL location
mkdir -p "$REAL_SOURCE/alpha"
cat > "$REAL_SOURCE/alpha/SKILL.md" << 'SKILLEOF'
---
name: alpha
description: Test skill alpha
---
# Alpha Skill
SKILLEOF

mkdir -p "$REAL_SOURCE/beta"
cat > "$REAL_SOURCE/beta/SKILL.md" << 'SKILLEOF'
---
name: beta
description: Test skill beta
---
# Beta Skill
SKILLEOF

mkdir -p "$REAL_SOURCE/group/nested"
cat > "$REAL_SOURCE/group/nested/SKILL.md" << 'SKILLEOF'
---
name: nested
description: Nested skill for flat-name test
---
# Nested Skill
SKILLEOF

# Symlink the config source dir to the real location
SYMLINK_SOURCE="$HOME/.config/skillshare/skills"
rm -rf "$SYMLINK_SOURCE"
ln -s "$REAL_SOURCE" "$SYMLINK_SOURCE"

# Verify symlink
ls -la "$SYMLINK_SOURCE"
readlink "$SYMLINK_SOURCE"
```

**Expected:**
- `readlink` shows the symlink pointing to `$HOME/dotfiles/skillshare-skills`
- `ls -la` shows the symlink arrow

## Step 1: Merge mode sync with symlinked source

```bash
ss sync --dry-run
```

**Expected:**
- Discovers 3 skills (alpha, beta, group__nested)
- No errors about "failed to walk source directory"
- Dry-run completes successfully

```bash
ss sync
```

**Expected:**
- All 3 skills are synced (linked/updated) to at least one target
- No errors

## Step 2: Verify symlinks resolve correctly through symlinked source

```bash
# Check that skill symlinks in the target resolve to real files
TARGET_DIR="$HOME/.claude/skills"

# The symlink inside target should resolve to a real directory
ls -la "$TARGET_DIR/alpha" 2>/dev/null || echo "alpha not found in $TARGET_DIR"
stat "$TARGET_DIR/alpha/SKILL.md" 2>/dev/null && echo "RESOLVES OK" || echo "BROKEN SYMLINK"
```

**Expected:**
- `alpha` is a symlink inside the target directory
- `stat` succeeds — the symlink resolves correctly (not broken)
- `RESOLVES OK` is printed

## Step 3: Status reports correctly with symlinked source

```bash
ss status
```

**Expected:**
- Shows source directory (the symlink path, not the resolved path)
- Shows targets with linked skill counts
- No errors about unresolvable paths

## Step 4: Diff detects no changes after sync

```bash
ss diff --no-tui
```

**Expected:**
- No pending changes (all skills are already synced)
- No errors

## Step 5: List shows skills through symlinked source

```bash
ss list --no-tui
```

**Expected:**
- Lists alpha, beta, group/nested (or group__nested)
- No errors about walking source directory

## Step 6: Symlinked target directory — merge mode

```bash
# Create a REAL target directory and symlink it
REAL_TARGET="$HOME/dotfiles/claude-skills"
mkdir -p "$REAL_TARGET"

# Remove existing claude target and replace with symlink
CLAUDE_TARGET="$HOME/.claude/skills"
rm -rf "$CLAUDE_TARGET"
ln -s "$REAL_TARGET" "$CLAUDE_TARGET"

# Verify
ls -la "$CLAUDE_TARGET"

# Re-sync — should create skill symlinks INSIDE the symlinked target
ss sync
```

**Expected:**
- Sync completes without errors
- Does NOT delete the target symlink

```bash
# Verify symlinks were created inside the symlinked target
ls -la "$CLAUDE_TARGET/"
ls -la "$REAL_TARGET/"
```

**Expected:**
- Skills (alpha, beta, group__nested) appear in both `$CLAUDE_TARGET/` and `$REAL_TARGET/`
- The target-level symlink (`$CLAUDE_TARGET` → `$REAL_TARGET`) is preserved

## Step 7: Symlinks resolve from both paths

```bash
# Access through symlink path
stat "$CLAUDE_TARGET/alpha/SKILL.md" && echo "VIA SYMLINK: OK" || echo "VIA SYMLINK: BROKEN"

# Access through real path
stat "$REAL_TARGET/alpha/SKILL.md" && echo "VIA REAL: OK" || echo "VIA REAL: BROKEN"
```

**Expected:**
- Both print "OK" — symlinks resolve from both the symlinked and real target paths

## Step 8: Copy mode with symlinked target

```bash
# Set up a copy-mode target
COPY_TARGET="$HOME/dotfiles/agents-skills"
COPY_SYMLINK="$HOME/.agents/skills"
mkdir -p "$COPY_TARGET"
mkdir -p "$(dirname "$COPY_SYMLINK")"
rm -rf "$COPY_SYMLINK"
ln -s "$COPY_TARGET" "$COPY_SYMLINK"

# Add to config as copy mode
ss target add agents-copy "$COPY_SYMLINK" --mode copy

# Sync
ss sync
```

**Expected:**
- Copy sync completes without errors
- Does NOT delete the target symlink
- Skills are copied (not symlinked) into the directory that `$COPY_SYMLINK` points to

```bash
# Verify files exist at real location
ls "$COPY_TARGET/" | head -5
test -f "$COPY_TARGET/alpha/SKILL.md" && echo "COPY OK" || echo "COPY MISSING"
```

**Expected:**
- `COPY OK` — skill files were copied into the real directory through the symlink

## Step 9: Both source AND target are symlinks (double symlink)

```bash
# Both are already symlinks from previous steps
readlink "$HOME/.config/skillshare/skills"
readlink "$CLAUDE_TARGET"

# Sync should still work
ss sync --dry-run
```

**Expected:**
- Both readlink commands show symlink targets
- Dry-run sync completes with discovered skills
- No errors

## Step 10: Chained symlinks (link → link → real dir)

```bash
# Create a chain: link2 → link1 → real_source
CHAIN_DIR="$HOME/chain-test"
mkdir -p "$CHAIN_DIR"
ln -s "$REAL_SOURCE" "$CHAIN_DIR/link1"
ln -s "$CHAIN_DIR/link1" "$CHAIN_DIR/link2"

# Replace source with chained symlink
rm "$SYMLINK_SOURCE"
ln -s "$CHAIN_DIR/link2" "$SYMLINK_SOURCE"

# Verify chain
readlink "$SYMLINK_SOURCE"
readlink "$CHAIN_DIR/link2"
readlink "$CHAIN_DIR/link1"

# Sync should resolve the full chain
ss sync
```

**Expected:**
- Sync completes successfully — discovers 3 skills
- All chained symlinks are followed correctly
- No "not a directory" or "too many levels of symbolic links" errors

## Step 11: Collect (pull) through symlinked target

```bash
# Create a local-only skill in the symlinked target
mkdir -p "$CLAUDE_TARGET/local-only"
cat > "$CLAUDE_TARGET/local-only/SKILL.md" << 'SKILLEOF'
---
name: local-only
description: A skill created directly in the target
---
# Local Only
SKILLEOF

# Collect should detect it (specify target name since multiple targets exist)
ss collect claude --dry-run
```

**Expected:**
- Detects `local-only` as a local skill in the target
- Shows it would be pulled to source
- No errors about scanning symlinked directory

## Step 12: Update --all through symlinked source

This step validates that `ss update --all` can walk the symlinked source
directory to discover updatable skills. We simulate an installed skill with
metadata so update can discover it.

```bash
# Create a skill with install metadata (simulates remote install)
mkdir -p "$REAL_SOURCE/remote-skill"
cat > "$REAL_SOURCE/remote-skill/SKILL.md" << 'SKILLEOF'
---
name: remote-skill
description: Simulated remote skill
---
# Remote Skill
SKILLEOF

cat > "$REAL_SOURCE/remote-skill/.skillshare-meta.json" << 'METAEOF'
{
  "source": "github.com/example/skills/remote-skill",
  "repo_url": "https://github.com/example/skills",
  "installed_at": "2025-01-01T00:00:00Z"
}
METAEOF

# Sync to pick up new skill
ss sync

# Update --all should discover remote-skill through the symlinked source
ss update --all --dry-run 2>&1
```

**Expected:**
- `update --all` scans the symlinked source directory
- Discovers `remote-skill` as an updatable skill (has metadata)
- No errors about walking/scanning the source directory
- Output mentions `remote-skill` (even if update itself fails due to no actual remote — the discovery is what we test)

## Step 13: Update --group through symlinked source

```bash
# The "group" directory contains "nested" — update --group should find it
# Add metadata to nested skill so it's updatable
cat > "$REAL_SOURCE/group/nested/.skillshare-meta.json" << 'METAEOF'
{
  "source": "github.com/example/group-skills/nested",
  "repo_url": "https://github.com/example/group-skills",
  "installed_at": "2025-01-01T00:00:00Z"
}
METAEOF

ss update --group group --dry-run 2>&1
```

**Expected:**
- Walks the `group` directory (which is inside the symlinked source) to find updatable items
- Discovers `group/nested` as updatable
- No "failed to walk group" errors

## Step 14: Uninstall single skill through symlinked source

```bash
# Uninstall beta skill — should resolve through symlinked source
ss uninstall beta --force --dry-run
```

**Expected:**
- Resolves `beta` in the symlinked source directory
- Shows it would remove `beta`
- No errors about path resolution

```bash
# Actually uninstall
ss uninstall beta --force
```

**Expected:**
- Removes `beta` from the real source directory (through symlink)
- No errors
- `beta` no longer in `ss list --no-tui`

```bash
# Verify beta is gone
test -d "$REAL_SOURCE/beta" && echo "STILL EXISTS" || echo "REMOVED OK"
ss list --no-tui
```

**Expected:**
- `REMOVED OK` — beta removed from real source dir
- `ss list` no longer shows beta

## Step 15: Uninstall --group through symlinked source

```bash
# Uninstall group — should walk the symlinked group dir
ss uninstall --group group --force --dry-run
```

**Expected:**
- Walks the `group` directory inside the symlinked source
- Discovers `group/nested` as a skill to uninstall
- No "failed to walk group" errors

```bash
# Actually uninstall the group
ss uninstall --group group --force
```

**Expected:**
- Removes `group/nested` from the real source dir
- No errors

```bash
# Verify group is gone
test -d "$REAL_SOURCE/group/nested" && echo "STILL EXISTS" || echo "REMOVED OK"
```

**Expected:**
- `REMOVED OK`

## Step 16: Uninstall by nested name resolution through symlinked source

```bash
# Re-create a nested skill for this test
mkdir -p "$REAL_SOURCE/mygroup/deepskill"
cat > "$REAL_SOURCE/mygroup/deepskill/SKILL.md" << 'SKILLEOF'
---
name: deepskill
description: Deep nested skill
---
# Deep Skill
SKILLEOF

ss sync

# Uninstall by short name — resolveNestedSkillDir must walk symlinked source
ss uninstall deepskill --force
```

**Expected:**
- Resolves `deepskill` to `mygroup/deepskill` by walking the symlinked source
- Removes it successfully
- No errors about searching for skill

```bash
test -d "$REAL_SOURCE/mygroup/deepskill" && echo "STILL EXISTS" || echo "REMOVED OK"
```

**Expected:**
- `REMOVED OK`

## Step 17: Reconcile discovers skills through symlinked source

Reconcile is triggered by `ss install`. We install from a local `file://` bare
repo so the install writes metadata into the symlinked source, then reconcile
walks it to populate `registry.yaml`.

```bash
# Set up git identity for bare-repo operations
git config --global user.email "test@test.com"
git config --global user.name "Test"

# Create a bare git repo as install source
BARE_REPO="$HOME/test-repos/recon-repo.git"
rm -rf "$HOME/test-repos"
mkdir -p "$BARE_REPO"
git init --bare "$BARE_REPO"

WORK_DIR="$HOME/test-repos/work"
git clone "$BARE_REPO" "$WORK_DIR"
mkdir -p "$WORK_DIR/reconcile-skill"
cat > "$WORK_DIR/reconcile-skill/SKILL.md" << 'SKILLEOF'
---
name: reconcile-skill
description: Test reconcile through symlink
---
# Reconcile Skill
SKILLEOF
cd "$WORK_DIR"
git add -A && git commit -m "add skill"
git push origin HEAD:master

# Install from file:// URL — installs into the symlinked source dir
cd "$HOME"
ss install "file://$BARE_REPO" --skip-audit --yes

# Check registry.yaml — reconcile should have walked the symlinked source
cat "$HOME/.config/skillshare/registry.yaml"
```

**Expected:**
- Install succeeds: `reconcile-skill` installed into the symlinked source dir
- `registry.yaml` lists `reconcile-skill` (and any other metadata skills)
- Reconcile walked the symlinked source dir successfully (no errors)

## Step 18: Containment guard — group symlink outside source is rejected

If a group directory is a symlink pointing outside the source tree,
`uninstall --group` and `update --group` must refuse to operate (to prevent
accidental deletion/modification of external directories).

```bash
# Create a directory OUTSIDE the source tree
EXTERNAL="$HOME/external-danger"
mkdir -p "$EXTERNAL/victim-skill"
cat > "$EXTERNAL/victim-skill/SKILL.md" << 'SKILLEOF'
---
name: victim
description: Should NOT be uninstallable
---
# Victim
SKILLEOF

# Symlink a group inside source → external location
ln -s "$EXTERNAL" "$REAL_SOURCE/evil-group"

# Attempt group uninstall — should fail with containment error
ss uninstall --group evil-group --force 2>&1
echo "EXIT=$?"

# Attempt group update — should also fail
ss update --group evil-group --dry-run 2>&1
echo "EXIT=$?"

# Verify external directory was NOT touched
test -f "$EXTERNAL/victim-skill/SKILL.md" && echo "EXTERNAL SAFE" || echo "EXTERNAL DAMAGED"

# Cleanup
rm -f "$REAL_SOURCE/evil-group"
rm -rf "$EXTERNAL"
```

**Expected:**
- Both commands fail with "resolves outside source directory" error
- `EXTERNAL SAFE` is printed — the external directory was not touched
- This prevents symlink-based path traversal attacks

## Step 19: Idempotency — re-sync preserves everything

```bash
# Sync twice more
ss sync
ss sync
```

**Expected:**
- Both syncs complete without errors
- No "updated" or "pruned" entries (everything already in sync)
- Target symlink (`$CLAUDE_TARGET` → `$REAL_TARGET`) is still intact

```bash
# Final verification
readlink "$CLAUDE_TARGET"
ls "$REAL_TARGET/alpha/SKILL.md" && echo "STILL WORKS" || echo "BROKEN"
```

**Expected:**
- Target symlink still points to the same real directory
- Skills still resolve correctly
- `STILL WORKS` is printed

## Pass Criteria

- All 20 steps (0-19) pass
- No symlinks are incorrectly deleted during sync
- Skills discovered through symlinked source directories in all commands
- `update --all` and `update --group` walk symlinked source correctly
- `uninstall` (single, group, nested-name) resolves through symlinked source
- Group operations reject symlinks pointing outside source tree (containment guard)
- Reconcile discovers installed skills through symlinked source
- Symlinks inside symlinked targets resolve from both logical and physical paths
- Chained symlinks (2+ levels) work correctly
- Copy mode preserves target symlinks (doesn't unconditionally delete)
- Collect detects local skills through symlinked target directories
- Re-sync is idempotent — doesn't break existing links
