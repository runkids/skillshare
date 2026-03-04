# CLI E2E Runbook: Extras Sync (Merge, Copy, Symlink modes)

Validates `sync extras` syncing file-based resources (rules, commands) across
AI tools using merge (per-file symlink), copy, and symlink (entire directory)
modes.

**Origin**: Feature #59 — extras sync for rules, commands, agents beyond skills.

## Scope

- `sync extras` syncs all configured extras
- Merge mode: per-file symlinks from target to source
- Copy mode: per-file copies
- Symlink mode: entire directory symlink
- Conflict handling: skip without `--force`, overwrite with `--force`
- Orphan pruning: removes files in target that no longer exist in source
- `sync --all` syncs skills + extras together
- No extras configured: prints helpful hint
- Source directory missing: prints friendly message, continues

## Environment

Run inside devcontainer.
If `ss` alias is unavailable, replace `ss` with `skillshare`.

## Steps

### 1. Setup: initialize and create extras source directories

```bash
ss init --no-copy --all-targets --no-git --no-skill
mkdir -p ~/.config/skillshare/rules
mkdir -p ~/.config/skillshare/commands
echo "# Always use TDD" > ~/.config/skillshare/rules/tdd.md
echo "# Error handling" > ~/.config/skillshare/rules/errors.md
echo "# Deploy command" > ~/.config/skillshare/commands/deploy.md
```

**Expected**: Source directories and files created under `~/.config/skillshare/`.

### 2. Configure extras in config.yaml

```bash
cat >> ~/.config/skillshare/config.yaml << 'CONF'

extras:
  - name: rules
    targets:
      - path: ~/.claude/rules
      - path: ~/.continue/rules
        mode: copy
  - name: commands
    targets:
      - path: ~/.claude/commands
        mode: symlink
CONF
```

**Expected**: Config file updated with extras section containing 3 targets across 2 extras.

### 3. Dry run: verify sync extras --dry-run shows plan without changes

```bash
ss sync extras --dry-run
```

**Expected**:
- Output contains "Dry run mode"
- Output contains "Rules" header
- Output contains "files synced" or "files copied" for each target
- No actual symlinks or copies created yet:

```bash
ls ~/.claude/rules/ 2>/dev/null && echo "EXISTS" || echo "NOT YET"
# Should print: NOT YET
```

### 4. Sync extras: merge mode (per-file symlinks)

```bash
ss sync extras
```

**Expected**:
- Output contains "Rules" and "Commands" headers
- `~/.claude/rules` target shows merge mode with 2 files synced
- `~/.continue/rules` target shows copy mode with 2 files synced
- `~/.claude/commands` target shows symlink mode

Verify merge mode (per-file symlinks):

```bash
ls -la ~/.claude/rules/
# Should show tdd.md and errors.md as symlinks
readlink ~/.claude/rules/tdd.md
# Should point to absolute path under ~/.config/skillshare/rules/tdd.md
```

### 5. Verify copy mode (real file copies)

```bash
ls -la ~/.continue/rules/
# Should show tdd.md and errors.md as regular files (not symlinks)
file ~/.continue/rules/tdd.md
# Should NOT contain "symbolic link"
cat ~/.continue/rules/tdd.md
# Should contain "# Always use TDD"
```

### 6. Verify symlink mode (entire directory linked)

```bash
ls -la ~/.claude/ | grep commands
# Should show commands as a symlink to the source directory
readlink ~/.claude/commands
# Should point to absolute path of ~/.config/skillshare/commands/
cat ~/.claude/commands/deploy.md
# Should contain "# Deploy command"
```

### 7. Idempotent re-sync: running again produces no errors

```bash
ss sync extras
```

**Expected**:
- All targets report "files synced" or "up to date"
- No errors or warnings
- File counts remain the same

### 8. Add new source file and re-sync

```bash
echo "# Code review rules" > ~/.config/skillshare/rules/review.md
ss sync extras
```

**Expected**:
- `~/.claude/rules` now has 3 files (tdd.md, errors.md, review.md)
- `~/.continue/rules` now has 3 files (copies)

```bash
ls ~/.claude/rules/ | wc -l
# Should be 3
ls ~/.continue/rules/ | wc -l
# Should be 3
```

### 9. Prune orphans: remove source file and re-sync

```bash
rm ~/.config/skillshare/rules/errors.md
ss sync extras
```

**Expected**:
- Output shows "1 pruned" for merge-mode target (`~/.claude/rules`)
- `~/.claude/rules/errors.md` symlink is removed
- `~/.continue/rules/errors.md` copy is removed

```bash
ls ~/.claude/rules/
# Should only have tdd.md and review.md
ls ~/.continue/rules/
# Should only have tdd.md and review.md
```

### 10. Conflict handling: existing file without --force

Create a conflict — a real file at a target path where merge would create a symlink:

```bash
# Remove the symlink first
rm ~/.claude/rules/tdd.md
# Create a real file (user-created content)
echo "my local notes" > ~/.claude/rules/tdd.md
ss sync extras
```

**Expected**:
- Output warns about skipped file(s) with "use --force to override"
- The user's local file is preserved (not overwritten)

```bash
cat ~/.claude/rules/tdd.md
# Should still contain "my local notes"
```

### 11. Conflict handling: --force overwrites

```bash
ss sync extras --force
```

**Expected**:
- Output shows successful sync (no skipped)
- `~/.claude/rules/tdd.md` is now a symlink again

```bash
readlink ~/.claude/rules/tdd.md
# Should point to source
cat ~/.claude/rules/tdd.md
# Should contain "# Always use TDD"
```

### 12. sync --all: syncs both skills and extras

```bash
ss sync --all
```

**Expected**:
- Output contains skill sync section (targets)
- Output contains extras section (Rules, Commands)
- Both skills and extras synced in a single command

### 13. No extras configured: helpful hint

```bash
# Back up config, remove extras section
cp ~/.config/skillshare/config.yaml ~/.config/skillshare/config.yaml.bak
sed -i '/^extras:/,$d' ~/.config/skillshare/config.yaml
ss sync extras
```

**Expected**:
- Output contains "No extras configured"
- Output shows YAML example for adding extras

```bash
# Restore config
cp ~/.config/skillshare/config.yaml.bak ~/.config/skillshare/config.yaml
```

### 14. Source directory missing: friendly message

```bash
# Add an extra with non-existent source
cat >> ~/.config/skillshare/config.yaml << 'CONF'
  - name: nonexistent
    targets:
      - path: ~/.claude/nonexistent
CONF
ss sync extras
```

**Expected**:
- Output for "nonexistent" extra says source directory does not exist
- Output suggests creating the directory
- Other extras (rules, commands) still sync successfully

### 15. Nested directory structure preserved

```bash
mkdir -p ~/.config/skillshare/rules/lang/go
echo "# Go style" > ~/.config/skillshare/rules/lang/go/style.md
ss sync extras
```

**Expected**:
- Nested structure is preserved in target:

```bash
cat ~/.claude/rules/lang/go/style.md
# Should contain "# Go style"
readlink ~/.claude/rules/lang/go/style.md
# Should point to source
```

## Pass Criteria

All 15 steps pass. Key behaviors validated:
- Three sync modes (merge, copy, symlink) work correctly
- Conflict detection and --force override
- Orphan pruning removes stale files
- Idempotent re-sync
- `sync --all` combines skills + extras
- Nested directories preserved
- Graceful handling of missing source/config
