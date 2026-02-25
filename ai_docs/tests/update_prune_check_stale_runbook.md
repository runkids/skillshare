# CLI E2E Runbook: update --prune + check stale detection

Validates that `skillshare check` reports `stale` for skills deleted upstream,
and that `skillshare update --prune` removes stale skills (moved to trash).

**Scenario**: A multi-skill repo removes one skill. `check` should show `stale`
(not `update_available`). `update --all --prune` should clean it up.

## Scope

- Install two skills from a local `file://` bare repo
- Delete one skill from the remote, push new commit
- `check --json` reports `stale` for deleted skill
- `check` (text) shows stale warning with `--prune` hint
- `update --all` (without `--prune`) warns about stale skills
- `update --all --prune` removes stale skill to trash
- Surviving skill is updated normally

## Environment

Run inside devcontainer with `ssenv` HOME isolation.
Offline test — uses `file://` bare repo, no network required.

## Steps

### 1. Create isolated environment

```bash
ssenv create "$ENV_NAME" --init
```

Expected:
- Environment created successfully

### 2. Create bare remote with two skills

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  REMOTE=~/remote-multi.git
  WORK=~/work-clone

  git init --bare "$REMOTE"
  git clone "$REMOTE" "$WORK"
  cd "$WORK"
  git config user.email "test@test.com"
  git config user.name "test"

  mkdir -p skills/keep-skill skills/doomed-skill
  echo "---
name: keep-skill
---
# Keep Skill v1" > skills/keep-skill/SKILL.md
  echo "---
name: doomed-skill
---
# Doomed Skill" > skills/doomed-skill/SKILL.md

  git add -A
  git commit -m "add two skills"
  git push origin HEAD
  echo "=== Remote ready ==="
'
```

Expected:
- Bare repo created with two skill subdirectories
- Output includes "=== Remote ready ==="

### 3. Install both skills from the bare repo

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  REMOTE=~/remote-multi.git
  ss install "file://$REMOTE//skills/keep-skill" -g --skip-audit
  ss install "file://$REMOTE//skills/doomed-skill" -g --skip-audit
  echo "=== Installed ==="
  ls ~/.config/skillshare/skills/
'
```

Expected:
- Both skills installed successfully
- `ls` shows `keep-skill` and `doomed-skill` in source directory

### 4. Delete doomed-skill from remote and push update

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  WORK=~/work-clone
  cd "$WORK"
  rm -rf skills/doomed-skill
  echo "---
name: keep-skill
---
# Keep Skill v2 — updated" > skills/keep-skill/SKILL.md
  git add -A
  git commit -m "remove doomed-skill, update keep-skill"
  git push origin HEAD
  echo "=== Remote updated ==="
'
```

Expected:
- Commit pushed successfully
- `doomed-skill` no longer in remote

### 5. check --json reports stale status

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  ss check --json -g
'
```

Expected:
- JSON output has a skill with `"status": "stale"` for `doomed-skill`
- `keep-skill` has `"status": "update_available"` (content changed)
- Exit code 0

### 6. check (text) shows stale warning with --prune hint

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  ss check -g
'
```

Expected:
- Output contains "stale"
- Output contains "--prune"

### 7. update --all without --prune shows stale warning

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  ss update --all -g --skip-audit
'
```

Expected:
- Output contains "stale" warning
- Output contains "--prune" suggestion
- `doomed-skill` still exists in `~/.config/skillshare/skills/`
- `keep-skill` is updated (content has "v2")

### 8. update --all --prune removes stale skill

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  ss update --all -g --prune --skip-audit
'
```

Expected:
- Output contains "pruned" or "Pruned"
- Output mentions `doomed-skill`
- `doomed-skill` is gone from `~/.config/skillshare/skills/`
- `keep-skill` still exists

### 9. Verify stale skill moved to trash

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  ss trash list -g
'
```

Expected:
- Trash contains `doomed-skill`

### 10. Verify registry cleaned up

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  cat ~/.config/skillshare/registry.yaml
'
```

Expected:
- Registry does NOT contain `doomed-skill`
- Registry still contains `keep-skill`

## Pass Criteria

- Step 5: `check --json` shows `stale` for deleted skill
- Step 6: Text output includes stale warning + `--prune` hint
- Step 7: Without `--prune`, stale skill survives + warning shown
- Step 8: With `--prune`, stale skill removed
- Step 9: Stale skill in trash (not permanently deleted)
- Step 10: Registry cleaned of pruned skill
