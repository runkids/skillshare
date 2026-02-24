# CLI E2E Runbook: Uninstall --all + Shell Glob Detection

Validates the `--all` flag for batch removal and shell glob detection
that intercepts accidentally expanded `*` arguments.

**Origin**: v0.15.5 — `ss uninstall *` without quotes caused shell expansion,
resulting in many "not found" warnings.

## Scope

- `--all` removes every skill from source (global mode)
- `--all` removes every skill from source (project mode)
- `--all` cannot be combined with skill names or `--group`
- `--all --dry-run` previews without removing
- `--all --force` skips confirmation
- Shell glob detection intercepts file-name-like args and suggests `--all`
- `--all` followed by `sync` leaves no orphans in targets

## Environment

Run inside devcontainer with `ssenv` isolation.

## Steps

### 1. Setup: init and install multiple skills

```bash
ss init --no-copy --all-targets --no-git --no-skill
ss install sickn33/antigravity-awesome-skills/skills/pdf-official --name pdf
ss install sickn33/antigravity-awesome-skills/skills/tdd-workflow
ss install sickn33/antigravity-awesome-skills/skills/react-best-practices --into frontend
ss sync
```

Expected:
- All 3 skills installed and synced
- Symlinks exist in `~/.claude/skills/`

Verify:

```bash
ls ~/.config/skillshare/skills/
ls -la ~/.claude/skills/
```

### 2. --all --dry-run previews without removing

```bash
ss uninstall --all --dry-run
```

Expected:
- Output contains `dry-run` and `would move to trash`
- All skills still exist in source

Verify:

```bash
ls ~/.config/skillshare/skills/ | grep -c .
# Should be >= 3 (pdf, tdd-workflow, frontend)
```

### 3. --all --force removes all skills

```bash
ss uninstall --all --force
```

Expected:
- Output contains `Uninstalled` for each top-level entry
- Source directory is empty (no skill directories remain)
- Registry skills list is cleared

Verify:

```bash
REMAINING=$(ls ~/.config/skillshare/skills/ 2>/dev/null | grep -v '.gitignore' | wc -l | tr -d ' ')
echo "Remaining skills: $REMAINING (expected: 0)"

grep -c 'name:' ~/.config/skillshare/registry.yaml || echo "no skills in registry"
```

### 4. Sync after --all leaves no orphans

```bash
ss sync
```

Expected:
- Symlinks removed from all targets
- No orphan directories remain

Verify:

```bash
TARGET=~/.claude/skills
for skill in pdf tdd-workflow frontend__react-best-practices; do
  [ ! -e "$TARGET/$skill" ] && echo "$skill: cleaned" || echo "$skill: STILL EXISTS (FAIL)"
done
```

### 5. --all mutual exclusion

```bash
ss uninstall --all some-skill 2>&1; echo "exit: $?"
ss uninstall --all --group frontend 2>&1; echo "exit: $?"
```

Expected:
- First command: error containing `--all cannot be used with skill names`
- Second command: error containing `--all cannot be used with --group`
- Both exit non-zero

### 6. Shell glob detection

Simulate what happens when shell expands `*` in a typical project directory:

```bash
ss uninstall README.md go.mod go.sum Makefile cmd internal 2>&1; echo "exit: $?"
```

Expected:
- Output contains suggestion to use `--all`
- Exits non-zero
- No skills are removed

### 7. --all in project mode

```bash
mkdir -p /tmp/e2e-project && cd /tmp/e2e-project
ss init -p --no-copy --no-git --no-skill --target claude
ss install sickn33/antigravity-awesome-skills/skills/pdf-official --name pdf -p
ss install sickn33/antigravity-awesome-skills/skills/tdd-workflow -p
ss sync -p
ss uninstall --all --force -p
```

Expected:
- Both skills removed from `.skillshare/skills/`
- Project registry skills list cleared

Verify:

```bash
REMAINING=$(ls /tmp/e2e-project/.skillshare/skills/ 2>/dev/null | grep -v '.gitignore' | wc -l | tr -d ' ')
echo "Remaining project skills: $REMAINING (expected: 0)"
```

## Pass Criteria

- [ ] `--all --dry-run` shows preview without removing skills
- [ ] `--all --force` removes all skills from source
- [ ] Registry `skills:` list cleared after `--all`
- [ ] `sync` after `--all` cleans up target symlinks
- [ ] `--all` + skill names → mutual exclusion error
- [ ] `--all` + `--group` → mutual exclusion error
- [ ] Shell glob detection intercepts file-name args
- [ ] `--all` works in project mode (`-p`)
