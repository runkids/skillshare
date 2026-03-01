# CLI E2E Runbook: Symlinked Source/Target Directories (vercel/skills#456)

Validates that sync, status, diff, list, and collect work correctly when
source and/or target directories are symlinks — the "dotfiles manager" scenario.

## Scope

- Source directory is a symlink (dotfiles manager pointing `~/.config/skillshare/skills/` elsewhere)
- Target directory is a symlink (dotfiles manager pointing `~/.claude/skills/` elsewhere)
- Both source and target are symlinks simultaneously
- Chained symlinks (link → link → real dir)
- Merge mode sync through symlinks
- Copy mode sync through symlinks
- `ss status`, `ss diff`, `ss list` through symlinks
- `ss collect` (pull local skills) through symlinked target
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
TARGET_DIR=$(ss status --json 2>/dev/null | head -1 | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    for t in d.get('targets', []):
        if t.get('mode') in ('merge', ''):
            print(t['path'])
            break
except: pass
" 2>/dev/null)

if [ -z "$TARGET_DIR" ]; then
  # Fallback: find a merge-mode target from config
  TARGET_DIR="$HOME/.claude/skills"
fi

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
- Lists alpha, beta, group__nested (or group/nested)
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
- Does NOT delete the target symlink (critical — this is the #456 fix)

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
- This is exactly what vercel/skills#456 breaks (relative symlinks resolve from wrong location)

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

# Collect should detect it
ss collect --dry-run
```

**Expected:**
- Detects `local-only` as a local skill in the target
- Shows it would be pulled to source
- No errors about scanning symlinked directory

## Step 12: Idempotency — re-sync preserves everything

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

- All 13 steps (0-12) pass
- No symlinks are incorrectly deleted during sync
- Skills discovered through symlinked source directories
- Symlinks inside symlinked targets resolve from both logical and physical paths
- Chained symlinks (2+ levels) work correctly
- Copy mode preserves target symlinks (doesn't unconditionally delete)
- Collect detects local skills through symlinked target directories
- Re-sync is idempotent — doesn't break existing links
