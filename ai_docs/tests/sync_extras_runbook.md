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
# Clean pre-existing extras and target dirs from ssenv --init
ss extras remove rules --force -g 2>/dev/null || true
rm -rf ~/.claude/rules ~/.continue/rules ~/.claude/commands 2>/dev/null || true
# Create extras source dirs using new layout
mkdir -p ~/.config/skillshare/extras/rules
mkdir -p ~/.config/skillshare/extras/commands
echo "# Always use TDD" > ~/.config/skillshare/extras/rules/tdd.md
echo "# Error handling" > ~/.config/skillshare/extras/rules/errors.md
echo "# Deploy command" > ~/.config/skillshare/extras/commands/deploy.md
```

Expected:
- exit_code: 0

### 2. Configure extras in config.yaml

```bash
sed -i '/^extras:/,$d' ~/.config/skillshare/config.yaml
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

Expected:
- exit_code: 0

### 3. Dry run: verify sync extras --dry-run shows plan without changes

```bash
ss sync extras --dry-run
```

Expected:
- exit_code: 0
- regex: dry.run|Dry run
- Sync Extras

```bash
ls ~/.claude/rules/ 2>/dev/null && echo "dir_exists=yes" || echo "dir_exists=no"
```

Expected:
- exit_code: 0
- dir_exists=no

### 4. Sync extras: merge mode (per-file symlinks)

```bash
ss sync extras
```

Expected:
- exit_code: 0
- Sync Extras
- synced

Verify merge mode (per-file symlinks):

```bash
ls -la ~/.claude/rules/
readlink ~/.claude/rules/tdd.md
```

Expected:
- exit_code: 0
- tdd.md
- errors.md
- regex: skillshare/extras/rules/tdd\.md

### 5. Verify copy mode (real file copies)

```bash
ls -la ~/.continue/rules/
# Verify it's a real file (not symlink)
test -f ~/.continue/rules/tdd.md && ! test -L ~/.continue/rules/tdd.md && echo "real_file=yes" || echo "real_file=no"
cat ~/.continue/rules/tdd.md
```

Expected:
- exit_code: 0
- tdd.md
- errors.md
- real_file=yes
- Always use TDD

### 6. Verify symlink mode (entire directory linked)

```bash
ls -la ~/.claude/ | grep commands
readlink ~/.claude/commands
cat ~/.claude/commands/deploy.md
```

Expected:
- exit_code: 0
- commands
- regex: skillshare/extras/commands
- Deploy command

### 7. Idempotent re-sync: running again produces no errors

```bash
ss sync extras
```

Expected:
- exit_code: 0
- Sync Extras
- synced

### 8. Add new source file and re-sync

```bash
echo "# Code review rules" > ~/.config/skillshare/extras/rules/review.md
ss sync extras
```

Expected:
- exit_code: 0

```bash
ls ~/.claude/rules/ | wc -l | tr -d ' '
ls ~/.continue/rules/ | wc -l | tr -d ' '
```

Expected:
- exit_code: 0
- 3

### 9. Prune orphans: remove source file and re-sync

```bash
rm ~/.config/skillshare/extras/rules/errors.md
ss sync extras
```

Expected:
- exit_code: 0
- pruned

```bash
ls ~/.claude/rules/
ls ~/.continue/rules/
```

Expected:
- exit_code: 0
- tdd.md
- review.md
- Not errors.md

### 10. Conflict handling: existing file without --force

Create a conflict — a real file at a target path where merge would create a symlink:

```bash
# Remove the symlink first
rm ~/.claude/rules/tdd.md
# Create a real file (user-created content)
echo "my local notes" > ~/.claude/rules/tdd.md
ss sync extras
```

Expected:
- exit_code: 0

```bash
cat ~/.claude/rules/tdd.md
```

Expected:
- exit_code: 0

### 11. Conflict handling: --force overwrites

```bash
ss sync extras --force
```

Expected:
- exit_code: 0

```bash
readlink ~/.claude/rules/tdd.md
cat ~/.claude/rules/tdd.md
```

Expected:
- exit_code: 0
- regex: skillshare/extras/rules/tdd\.md
- Always use TDD

### 12. sync --all: syncs both skills and extras

```bash
ss sync --all
```

Expected:
- exit_code: 0
- Sync Extras
- synced

### 13. No extras configured: helpful hint

```bash
# Back up config, remove extras section
cp ~/.config/skillshare/config.yaml ~/.config/skillshare/config.yaml.bak
sed -i '/^extras:/,$d' ~/.config/skillshare/config.yaml
ss sync extras
```

Expected:
- exit_code: 0
- regex: No extras configured

```bash
# Restore config
cp ~/.config/skillshare/config.yaml.bak ~/.config/skillshare/config.yaml
```

Expected:
- exit_code: 0

### 14. Source directory missing: friendly message

```bash
# Add an extra with non-existent source (idempotent: remove if exists, then init)
ss extras remove nonexistent --force -g 2>/dev/null || true
ss extras init nonexistent --target ~/.claude/nonexistent -g
rm -rf ~/.config/skillshare/extras/nonexistent
ss sync extras
```

Expected:
- exit_code: 0
- regex: not exist|not found|missing
- Sync Extras

### 15. Auto-migration: legacy flat path to extras/<name>/

```bash
# Remove the nonexistent extra from step 14 first
ss extras remove nonexistent --force -g 2>/dev/null || true
# Add a new extra via ss extras init
ss extras init migrate-test --target ~/.claude/migrate-rules -g
# Remove the new-style directory that init created
rm -rf ~/.config/skillshare/extras/migrate-test
# Create files at legacy (flat) path to simulate old layout
mkdir -p ~/.config/skillshare/migrate-test
echo "# Legacy rule" > ~/.config/skillshare/migrate-test/legacy.md
test -d ~/.config/skillshare/migrate-test && echo "legacy_exists=yes"
test -d ~/.config/skillshare/extras/migrate-test && echo "new_exists=yes" || echo "new_exists=no"
```

Expected:
- exit_code: 0
- legacy_exists=yes
- new_exists=no

```bash
ss sync extras 2>&1
```

Expected:
- exit_code: 0
- regex: Migrated|migrated
- Sync Extras

```bash
test -d ~/.config/skillshare/extras/migrate-test && echo "migrated=yes" || echo "migrated=no"
test -d ~/.config/skillshare/migrate-test && echo "legacy_still=yes" || echo "legacy_still=no"
```

Expected:
- exit_code: 0
- migrated=yes
- legacy_still=no

### 16. Nested directory structure preserved

```bash
mkdir -p ~/.config/skillshare/extras/rules/lang/go
echo "# Go style" > ~/.config/skillshare/extras/rules/lang/go/style.md
ss sync extras
```

Expected:
- exit_code: 0

```bash
cat ~/.claude/rules/lang/go/style.md
readlink ~/.claude/rules/lang/go/style.md
```

Expected:
- exit_code: 0
- Go style
- regex: skillshare/extras/rules/lang/go/style\.md

## Pass Criteria

All 16 steps pass. Key behaviors validated:
- Three sync modes (merge, copy, symlink) work correctly
- Conflict detection and --force override
- Orphan pruning removes stale files
- Idempotent re-sync
- `sync --all` combines skills + extras
- Auto-migration from legacy flat path to extras/<name>/
- Nested directories preserved
- Graceful handling of missing source/config
