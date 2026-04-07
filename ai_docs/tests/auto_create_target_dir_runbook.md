# CLI E2E Runbook: Auto-Create Missing Target Directory

Validates that `sync` auto-creates a missing target directory when the parent exists,
fails fast when the parent is also missing, and shows appropriate notifications.

Ref: GitHub Issue #87

## Scope

- Sync auto-creates target `skills/` directory when parent (e.g., `~/.claude/`) exists
- Sync fails fast with typo hint when parent directory is missing
- Dry-run shows "Will create" without actually creating
- Copy mode also auto-creates
- Notification message is visible in output

## Environment

Run inside devcontainer with ssenv isolation.

## Step 1: Setup — create skill and remove target skills dir

```bash
mkdir -p ~/.config/skillshare/skills/test-skill
cat > ~/.config/skillshare/skills/test-skill/SKILL.md << 'SKILL'
---
name: test-skill
description: Test skill for auto-create
---
# Test Skill
SKILL
ls ~/.config/skillshare/skills/test-skill/SKILL.md
```

Expected:
- exit_code: 0
- SKILL.md

## Step 2: Remove claude skills dir but keep parent

The setup from mdproof.json creates `~/.claude/`. We remove `~/.claude/skills/` to simulate a fresh CLI install.

```bash
rm -rf ~/.claude/skills
test -d ~/.claude && echo "parent exists" || echo "parent missing"
test -d ~/.claude/skills && echo "skills exists" || echo "skills missing"
```

Expected:
- exit_code: 0
- parent exists
- skills missing

## Step 3: Sync auto-creates missing target directory

```bash
ss sync -g
```

Expected:
- exit_code: 0
- Created target directory:
- merged

## Step 4: Verify skill was synced

```bash
test -L ~/.claude/skills/test-skill && echo "symlink OK" || echo "symlink MISSING"
```

Expected:
- exit_code: 0
- symlink OK

## Step 5: Auto-creates even when parent is also missing

This handles built-in targets like `universal` (`~/.agents/skills`) where the
entire path tree may not exist yet.

```bash
rm -rf ~/.newcli 2>/dev/null || true
test -d ~/.newcli && echo "parent exists" || echo "parent missing"
# Insert newcli target under the targets: map (sed inserts after the 'targets:' line)
CONFIG="$HOME/.config/skillshare/config.yaml"
sed -i '/^targets:/a\  newcli:\n    skills:\n      path: ~/.newcli/ai/skills' "$CONFIG"
ss sync -g 2>&1
test -d ~/.newcli/ai/skills && echo "deep dir created" || echo "deep dir missing"
```

Expected:
- exit_code: 0
- parent missing
- Created target directory:
- deep dir created

## Step 6: Dry-run shows would-create without creating

```bash
rm -rf ~/.codex/skills 2>/dev/null || true
test -d ~/.codex && echo "codex parent exists" || echo "codex parent missing"
ss sync -g --dry-run 2>&1
test -d ~/.codex/skills && echo "dir created" || echo "dir not created"
```

Expected:
- exit_code: 0
- codex parent exists
- Will create target directory:
- dir not created

## Pass Criteria

- All 6 steps pass
- Auto-create works for merge mode (default)
- Auto-create works even when parent directory is also missing
- Dry-run does not create directories
- Notification is visible in CLI output
