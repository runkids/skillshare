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
# Setup hook already ran ss init
# Batch install from same repo using -s
ss install sickn33/antigravity-awesome-skills -s pdf-official,tdd-workflow
ss install sickn33/antigravity-awesome-skills -s react-best-practices --into frontend --force
ss sync
```

Expected:
- exit_code: 0
- Installed

Verify:

```bash
ls ~/.config/skillshare/skills/
ls -la ~/.claude/skills/
```

Expected:
- exit_code: 0
- pdf-official
- tdd-workflow
- frontend

### 2. --all --dry-run previews without removing

```bash
ss uninstall --all --dry-run
```

Expected:
- exit_code: 0
- dry-run
- regex: would move to trash|would remove

Verify:

```bash
COUNT=$(ls ~/.config/skillshare/skills/ | grep -c .)
echo "Skill count: $COUNT"
test "$COUNT" -ge 3 && echo "PASS" || echo "FAIL: count too low"
```

Expected:
- exit_code: 0
- PASS
- Not FAIL

### 3. --all --force removes all skills

```bash
ss uninstall --all --force
```

Expected:
- exit_code: 0
- Uninstalled

Verify:

```bash
REMAINING=$(ls ~/.config/skillshare/skills/ 2>/dev/null | grep -v '.gitignore' | wc -l | tr -d ' ') || true
echo "Remaining skills: $REMAINING (expected: 0)"

grep -c 'name:' ~/.config/skillshare/registry.yaml 2>/dev/null || echo "no skills in registry"
```

Expected:
- exit_code: 0
- Remaining skills: 0 (expected: 0)

### 4. Sync after --all leaves no orphans

```bash
ss sync
```

Expected:
- exit_code: 0
- pruned

Verify:

```bash
TARGET=~/.claude/skills
for skill in pdf-official tdd-workflow frontend__react-best-practices; do
  [ ! -e "$TARGET/$skill" ] && echo "$skill: cleaned" || echo "$skill: STILL EXISTS (FAIL)"
done
```

Expected:
- exit_code: 0
- pdf-official: cleaned
- tdd-workflow: cleaned
- frontend__react-best-practices: cleaned
- Not STILL EXISTS

### 5. --all mutual exclusion

```bash
ss uninstall --all some-skill 2>&1 || true
ss uninstall --all --group frontend 2>&1 || true
```

Expected:
- --all cannot be used with skill names
- --all cannot be used with --group

### 6. Shell glob detection

Simulate what happens when shell expands `*` in a typical project directory:

```bash
ss uninstall README.md go.mod go.sum Makefile cmd internal 2>&1 || true
```

Expected:
- --all

### 7. --all in project mode

```bash
rm -rf /tmp/e2e-project && mkdir -p /tmp/e2e-project && cd /tmp/e2e-project
ss init -p --targets claude
ss install sickn33/antigravity-awesome-skills -s pdf-official,tdd-workflow -p
ss sync -p
ss uninstall --all --force -p
```

Expected:
- exit_code: 0
- Uninstalled

Verify:

```bash
REMAINING=$(ls /tmp/e2e-project/.skillshare/skills/ 2>/dev/null | grep -v '.gitignore' | wc -l | tr -d ' ') || true
echo "Remaining project skills: $REMAINING (expected: 0)"
```

Expected:
- exit_code: 0
- Remaining project skills: 0 (expected: 0)

## Pass Criteria

- [ ] `--all --dry-run` shows preview without removing skills
- [ ] `--all --force` removes all skills from source
- [ ] Registry `skills:` list cleared after `--all`
- [ ] `sync` after `--all` cleans up target symlinks
- [ ] `--all` + skill names → mutual exclusion error
- [ ] `--all` + `--group` → mutual exclusion error
- [ ] Shell glob detection intercepts file-name args
- [ ] `--all` works in project mode (`-p`)
