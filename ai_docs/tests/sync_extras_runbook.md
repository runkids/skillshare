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

Expected:
- exit_code: 0

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

Expected:
- exit_code: 0

### 3. Dry run: verify sync extras --dry-run shows plan without changes

```bash
ss sync extras --dry-run
```

Expected:
- exit_code: 0
- regex: dry.run|Dry run
- Rules

```bash
ls ~/.claude/rules/ 2>/dev/null && echo "EXISTS" || echo "NOT YET"
```

Expected:
- exit_code: 0
- NOT YET

### 4. Sync extras: merge mode (per-file symlinks)

```bash
ss sync extras
```

Expected:
- exit_code: 0
- Rules
- Commands

Verify merge mode (per-file symlinks):

```bash
ls -la ~/.claude/rules/
readlink ~/.claude/rules/tdd.md
```

Expected:
- exit_code: 0
- tdd.md
- errors.md
- regex: skillshare/rules/tdd\.md

### 5. Verify copy mode (real file copies)

```bash
ls -la ~/.continue/rules/
file ~/.continue/rules/tdd.md
cat ~/.continue/rules/tdd.md
```

Expected:
- exit_code: 0
- tdd.md
- errors.md
- Not symbolic link
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
- regex: skillshare/commands
- Deploy command

### 7. Idempotent re-sync: running again produces no errors

```bash
ss sync extras
```

Expected:
- exit_code: 0
- Rules
- Commands

### 8. Add new source file and re-sync

```bash
echo "# Code review rules" > ~/.config/skillshare/rules/review.md
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
rm ~/.config/skillshare/rules/errors.md
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
- regex: skip|conflict|--force

```bash
cat ~/.claude/rules/tdd.md
```

Expected:
- exit_code: 0
- my local notes

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
- regex: skillshare/rules/tdd\.md
- Always use TDD

### 12. sync --all: syncs both skills and extras

```bash
ss sync --all
```

Expected:
- exit_code: 0
- Rules
- Commands

### 13. No extras configured: helpful hint

```bash
# Back up config, remove extras section
cp ~/.config/skillshare/config.yaml ~/.config/skillshare/config.yaml.bak
sed -i '/^extras:/,$d' ~/.config/skillshare/config.yaml
ss sync extras
```

Expected:
- exit_code: 0
- No extras configured

```bash
# Restore config
cp ~/.config/skillshare/config.yaml.bak ~/.config/skillshare/config.yaml
```

Expected:
- exit_code: 0

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

Expected:
- exit_code: 0
- regex: not exist|not found|missing
- Rules

### 15. Nested directory structure preserved

```bash
mkdir -p ~/.config/skillshare/rules/lang/go
echo "# Go style" > ~/.config/skillshare/rules/lang/go/style.md
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
- regex: skillshare/rules/lang/go/style\.md

## Pass Criteria

All 15 steps pass. Key behaviors validated:
- Three sync modes (merge, copy, symlink) work correctly
- Conflict detection and --force override
- Orphan pruning removes stale files
- Idempotent re-sync
- `sync --all` combines skills + extras
- Nested directories preserved
- Graceful handling of missing source/config
