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
Setup hook handles `ss init -g`.

## Steps

### 1. Create bare remote with two skills

```bash
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
```

Expected:
- exit_code: 0
- === Remote ready ===

### 2. Install both skills from the bare repo

```bash
REMOTE=~/remote-multi.git
ss install "file://$REMOTE//skills/keep-skill" -g --skip-audit
ss install "file://$REMOTE//skills/doomed-skill" -g --skip-audit
echo "=== Installed ==="
ls ~/.config/skillshare/skills/
```

Expected:
- exit_code: 0
- === Installed ===
- keep-skill
- doomed-skill

### 3. Delete doomed-skill from remote and push update

```bash
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
```

Expected:
- exit_code: 0
- === Remote updated ===

### 4. check --json reports stale status

```bash
ss check --json -g
```

Expected:
- exit_code: 0
- jq: .skills[] | select(.name == "doomed-skill") | .status == "stale"
- jq: .skills[] | select(.name == "keep-skill") | .status == "update_available"

### 5. check (text) shows stale warning with --prune hint

```bash
ss check -g
```

Expected:
- exit_code: 0
- stale
- --prune

### 6. update --all without --prune shows stale warning

```bash
ss update --all -g --skip-audit
```

Expected:
- exit_code: 0
- stale
- --prune
- doomed-skill

### 7. update --all --prune removes stale skill

```bash
ss update --all -g --prune --skip-audit
```

Expected:
- exit_code: 0
- regex: [Pp]runed
- doomed-skill

### 8. Verify stale skill moved to trash

```bash
ss trash list -g
```

Expected:
- exit_code: 0
- doomed-skill

### 9. Verify registry cleaned up

```bash
cat ~/.config/skillshare/registry.yaml
```

Expected:
- exit_code: 0
- keep-skill
- Not doomed-skill

## Pass Criteria

- Step 4: `check --json` shows `stale` for deleted skill
- Step 5: Text output includes stale warning + `--prune` hint
- Step 6: Without `--prune`, stale skill survives + warning shown
- Step 7: With `--prune`, stale skill removed
- Step 8: Stale skill in trash (not permanently deleted)
- Step 9: Registry cleaned of pruned skill
