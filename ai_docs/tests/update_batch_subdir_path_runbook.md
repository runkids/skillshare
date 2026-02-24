# CLI E2E Runbook: update --all batch subdir path duplication fix

Validates that `update --all` in project mode does not write skills to the
wrong path when `meta.Subdir` differs from the local install path.

**Root cause**: `UpdateSkillsFromRepo` used `filepath.Join(sourceDir, meta.Subdir)`
as the destination, but `meta.Subdir` is the repo-internal path (e.g. `skills/foo`),
while the local install path may be just `foo/`. This created leaked copies at
`sourceDir/skills/foo/` alongside the correct `sourceDir/foo/`, doubling the
skill count on the next scan.

## Scope

- Project-mode `update --all` with monorepo skills whose `meta.Subdir` != local path
- No leaked subdirectories created after update
- Skill count stays stable across repeated `update --all` runs
- Global-mode `update --all` with same pattern (same code path)

## Environment

Run inside devcontainer with `ssenv` HOME isolation.

## Steps

### 1. Create isolated environment

```bash
ENV_NAME="e2e-update-subdir-$(date +%Y%m%d-%H%M%S)"
docker exec "$CONTAINER" ssenv create "$ENV_NAME" --init
```

Expected:
- Environment created successfully

### 2. Create a bare monorepo with skills nested under `skills/` subdirectory

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- bash -c '
  set -e
  REPO=~/monorepo-remote.git
  WORK=~/monorepo-work

  git init --bare "$REPO"
  git clone "$REPO" "$WORK"

  for name in alpha beta gamma; do
    mkdir -p "$WORK/skills/$name"
    cat > "$WORK/skills/$name/SKILL.md" <<SKILL
---
name: $name
---
# $name skill v1
SKILL
  done

  cd "$WORK"
  git add -A
  git -c user.name=e2e -c user.email=e2e@test.com commit -m "init 3 skills"
  git push origin HEAD
'
```

Expected:
- Bare repo created with `skills/alpha`, `skills/beta`, `skills/gamma`

### 3. Set up a project with skills installed at flat paths (simulating --into or manual placement)

The key scenario: skills are installed locally at `alpha/`, `beta/`, `gamma/`
(without the `skills/` prefix), but `meta.Subdir` records `skills/alpha` etc.

```bash
docker exec "$CONTAINER" env SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
  ssenv enter "$ENV_NAME" -- bash -c '
  set -e
  PROJECT=~/test-project
  mkdir -p "$PROJECT/.skillshare/skills" "$PROJECT/.claude"
  cat > "$PROJECT/.skillshare/config.yaml" <<CFG
targets:
  - claude
CFG

  SKILLS_DIR="$PROJECT/.skillshare/skills"
  REPO_URL="file://$HOME/monorepo-remote.git"

  for name in alpha beta gamma; do
    LOCAL="$SKILLS_DIR/$name"
    mkdir -p "$LOCAL"
    cp "$HOME/monorepo-work/skills/$name/SKILL.md" "$LOCAL/"

    cat > "$LOCAL/.skillshare-meta.json" <<META
{
  "source": "${REPO_URL}//skills/${name}",
  "type": "git",
  "repo_url": "$REPO_URL",
  "subdir": "skills/${name}",
  "installed_at": "2025-01-01T00:00:00Z"
}
META
  done

  echo "Installed skills:"
  ls -1 "$SKILLS_DIR"
  echo "---"
  echo "No leaked skills/ subdirectory:"
  test ! -d "$SKILLS_DIR/skills" && echo "PASS: no skills/skills/ dir"
'
```

Expected:
- 3 skills at `alpha/`, `beta/`, `gamma/`
- No `skills/` subdirectory exists under `.skillshare/skills/`

### 4. First `update --all` — verify no path leakage

```bash
docker exec "$CONTAINER" env SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
  ssenv enter "$ENV_NAME" -- bash -c '
  set -e
  cd ~/test-project
  ss update --all -p --skip-audit 2>&1 | tee /tmp/update-run1.log

  SKILLS_DIR=~/test-project/.skillshare/skills

  echo "=== Post-update directory listing ==="
  find "$SKILLS_DIR" -maxdepth 2 -type d | sort

  echo "=== Leak check ==="
  if [ -d "$SKILLS_DIR/skills" ]; then
    echo "FAIL: leaked skills/ subdirectory found!"
    ls -la "$SKILLS_DIR/skills/"
    exit 1
  else
    echo "PASS: no leaked skills/ subdirectory"
  fi

  echo "=== Skill count ==="
  COUNT=$(find "$SKILLS_DIR" -name "SKILL.md" | wc -l | tr -d " ")
  echo "SKILL_COUNT=$COUNT"
  test "$COUNT" -eq 3 || { echo "FAIL: expected 3 skills, got $COUNT"; exit 1; }
  echo "PASS: skill count is 3"
'
```

Expected:
- `update --all` succeeds
- No `skills/` subdirectory leaked under `.skillshare/skills/`
- Skill count remains exactly 3

### 5. Second `update --all` — verify count is stable (no doubling)

```bash
docker exec "$CONTAINER" env SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
  ssenv enter "$ENV_NAME" -- bash -c '
  set -e
  cd ~/test-project
  ss update --all -p --skip-audit 2>&1 | tee /tmp/update-run2.log

  SKILLS_DIR=~/test-project/.skillshare/skills

  echo "=== Leak check (run 2) ==="
  if [ -d "$SKILLS_DIR/skills" ]; then
    echo "FAIL: leaked skills/ subdirectory found after second run!"
    find "$SKILLS_DIR/skills" -name "SKILL.md" | wc -l
    exit 1
  else
    echo "PASS: no leaked skills/ subdirectory"
  fi

  echo "=== Skill count (run 2) ==="
  COUNT=$(find "$SKILLS_DIR" -name "SKILL.md" | wc -l | tr -d " ")
  echo "SKILL_COUNT=$COUNT"
  test "$COUNT" -eq 3 || { echo "FAIL: expected 3 skills, got $COUNT (doubling bug!)"; exit 1; }
  echo "PASS: skill count stable at 3 (no doubling)"
'
```

Expected:
- Second run produces same skill count (3, not 6)
- No leaked directory

### 6. Third `update --all` — triple-check stability

```bash
docker exec "$CONTAINER" env SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
  ssenv enter "$ENV_NAME" -- bash -c '
  set -e
  cd ~/test-project
  ss update --all -p --skip-audit 2>&1 | tee /tmp/update-run3.log

  SKILLS_DIR=~/test-project/.skillshare/skills
  COUNT=$(find "$SKILLS_DIR" -name "SKILL.md" | wc -l | tr -d " ")
  echo "SKILL_COUNT=$COUNT"
  test "$COUNT" -eq 3 || { echo "FAIL: expected 3, got $COUNT"; exit 1; }
  test ! -d "$SKILLS_DIR/skills" || { echo "FAIL: leaked dir"; exit 1; }
  echo "PASS: stable after 3 consecutive runs"
'
```

Expected:
- Count remains 3 across all 3 runs

### 7. Verify skills were actually updated (not just skipped)

Push changes to the remote, then update and verify content changed.

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- bash -c '
  set -e
  WORK=~/monorepo-work
  cd "$WORK"

  for name in alpha beta gamma; do
    cat > "skills/$name/SKILL.md" <<SKILL
---
name: $name
---
# $name skill v2 (updated)
SKILL
  done

  git add -A
  git -c user.name=e2e -c user.email=e2e@test.com commit -m "bump to v2"
  git push origin HEAD
'
```

```bash
docker exec "$CONTAINER" env SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
  ssenv enter "$ENV_NAME" -- bash -c '
  set -e
  cd ~/test-project
  ss update --all -p --skip-audit 2>&1

  SKILLS_DIR=~/test-project/.skillshare/skills

  for name in alpha beta gamma; do
    if grep -q "v2 (updated)" "$SKILLS_DIR/$name/SKILL.md"; then
      echo "PASS: $name updated to v2"
    else
      echo "FAIL: $name not updated"
      cat "$SKILLS_DIR/$name/SKILL.md"
      exit 1
    fi
  done

  # Final leak check
  test ! -d "$SKILLS_DIR/skills" || { echo "FAIL: leaked dir after content update"; exit 1; }
  echo "PASS: all skills updated, no leaks"
'
```

Expected:
- All 3 skills show v2 content
- Still no leaked `skills/` directory

### 8. Global mode: same pattern verification

```bash
docker exec "$CONTAINER" ssenv enter "$ENV_NAME" -- bash -c '
  set -e
  SOURCE=~/.config/skillshare/skills
  REPO_URL="file://$HOME/monorepo-remote.git"

  for name in alpha beta gamma; do
    LOCAL="$SOURCE/$name"
    mkdir -p "$LOCAL"
    cat > "$LOCAL/SKILL.md" <<SKILL
---
name: $name
---
# $name global v1
SKILL
    cat > "$LOCAL/.skillshare-meta.json" <<META
{
  "source": "${REPO_URL}//skills/${name}",
  "type": "git",
  "repo_url": "$REPO_URL",
  "subdir": "skills/${name}",
  "installed_at": "2025-01-01T00:00:00Z"
}
META
  done

  COUNT_BEFORE=$(find "$SOURCE" -name "SKILL.md" | wc -l | tr -d " ")
  ss update --all -g --skip-audit 2>&1 | tee /tmp/update-global.log

  if [ -d "$SOURCE/skills" ]; then
    echo "FAIL: leaked skills/ in global source"
    exit 1
  fi

  COUNT_AFTER=$(find "$SOURCE" -name "SKILL.md" | wc -l | tr -d " ")
  echo "Global SKILL_COUNT before=$COUNT_BEFORE after=$COUNT_AFTER"
  test "$COUNT_BEFORE" -eq "$COUNT_AFTER" || { echo "FAIL: count changed ($COUNT_BEFORE -> $COUNT_AFTER)"; exit 1; }
  echo "PASS: global mode no leak, count stable"
'
```

Expected:
- Global mode also has no leaked `skills/` subdirectory
- Skill count is exactly 3

## Pass Criteria

- [ ] Step 1: Environment created
- [ ] Step 2: Bare monorepo with 3 nested skills
- [ ] Step 3: Project skills installed at flat paths with divergent meta.Subdir
- [ ] Step 4: First `update --all` — no leak, count=3
- [ ] Step 5: Second `update --all` — count stable at 3 (no doubling)
- [ ] Step 6: Third `update --all` — still stable
- [ ] Step 7: Skills actually updated to v2, no leaks
- [ ] Step 8: Global mode same pattern — no leak, count=3
