# CLI E2E Runbook: registry.yaml Split

Verifies that skills are stored in `registry.yaml` (not `config.yaml`) after the refactor, including migration from old format.

## Scope

- Fresh init creates empty registry.yaml
- Install writes skills to registry.yaml, not config.yaml
- Migration: old config.yaml with skills[] → auto-split on any command
- Uninstall removes skills from registry.yaml
- Project mode: same behavior in .skillshare/

## Environment

Run inside devcontainer with ssenv isolation.

## Steps

### Step 1: Fresh init — verify registry.yaml exists

```bash
ss init --no-copy --all-targets --no-git --no-skill
```

**Expected:**
- `~/.config/skillshare/config.yaml` exists and does NOT contain `skills:`
- `~/.config/skillshare/registry.yaml` exists (may be empty or minimal)

```bash
cat ~/.config/skillshare/config.yaml
cat ~/.config/skillshare/registry.yaml 2>/dev/null || echo "MISSING"
```

### Step 2: Install a local skill — verify registry.yaml updated

```bash
mkdir -p /tmp/test-skill
echo "---
name: test-skill
---
# Test Skill" > /tmp/test-skill/SKILL.md

ss install /tmp/test-skill
```

**Expected:**
- `~/.config/skillshare/skills/test-skill/SKILL.md` exists
- `~/.config/skillshare/registry.yaml` contains `name: test-skill`
- `~/.config/skillshare/config.yaml` does NOT contain `skills:` or `test-skill`

```bash
cat ~/.config/skillshare/registry.yaml
grep -c "skills:" ~/.config/skillshare/config.yaml && echo "FAIL: config has skills" || echo "PASS: config clean"
```

### Step 3: Migration — old config with skills[] auto-migrates

```bash
# Manually inject skills[] into config.yaml (simulating old format)
cat > ~/.config/skillshare/config.yaml << 'YAML'
source: ~/.config/skillshare/skills
targets: {}
skills:
  - name: legacy-skill
    source: github.com/example/repo
YAML

# Remove registry to test migration
rm -f ~/.config/skillshare/registry.yaml

# Run any command — triggers migration via config.Load()
ss status
```

**Expected:**
- `~/.config/skillshare/registry.yaml` created with `name: legacy-skill`
- `~/.config/skillshare/config.yaml` no longer contains `skills:`

```bash
cat ~/.config/skillshare/registry.yaml
grep -c "skills:" ~/.config/skillshare/config.yaml && echo "FAIL: skills still in config" || echo "PASS: migration ok"
```

### Step 4: Migration preserves existing registry

```bash
# Write registry with real skill
cat > ~/.config/skillshare/registry.yaml << 'YAML'
skills:
  - name: real-skill
    source: github.com/real/repo
YAML

# Inject stale skills into config (simulating edge case)
cat > ~/.config/skillshare/config.yaml << 'YAML'
source: ~/.config/skillshare/skills
targets: {}
skills:
  - name: stale-skill
    source: github.com/stale/repo
YAML

ss status
```

**Expected:**
- `registry.yaml` still has `real-skill`, NOT `stale-skill`
- `config.yaml` may still have `skills:` (registry existed so migration skipped the write)

```bash
grep "real-skill" ~/.config/skillshare/registry.yaml && echo "PASS" || echo "FAIL"
grep "stale-skill" ~/.config/skillshare/registry.yaml && echo "FAIL: stale leaked" || echo "PASS: no leak"
```

### Step 5: Uninstall removes from registry.yaml

```bash
# Reset clean state
ss init --no-copy --all-targets --no-git --no-skill --force

mkdir -p /tmp/remove-me
echo "---
name: remove-me
---
# Remove Me" > /tmp/remove-me/SKILL.md

ss install /tmp/remove-me
ss uninstall remove-me --yes
```

**Expected:**
- `registry.yaml` does NOT contain `remove-me`

```bash
grep "remove-me" ~/.config/skillshare/registry.yaml && echo "FAIL: still present" || echo "PASS: removed"
```

### Step 6: Install with --into records group in registry

```bash
ss init --no-copy --all-targets --no-git --no-skill --force

mkdir -p /tmp/grouped-skill
echo "---
name: grouped-skill
---
# Grouped" > /tmp/grouped-skill/SKILL.md

ss install /tmp/grouped-skill --into frontend
```

**Expected:**
- `registry.yaml` contains `group: frontend` and `name: grouped-skill`
- `config.yaml` does NOT contain `group:` or `grouped-skill`

```bash
cat ~/.config/skillshare/registry.yaml
grep "group: frontend" ~/.config/skillshare/registry.yaml && echo "PASS" || echo "FAIL"
```

### Step 7: Project mode — registry in .skillshare/

```bash
mkdir -p /tmp/project-test
cd /tmp/project-test
ss init -p --target claude --no-git --no-skill

mkdir -p /tmp/proj-skill
echo "---
name: proj-skill
---
# Project Skill" > /tmp/proj-skill/SKILL.md

SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 ss install /tmp/proj-skill -p
```

**Expected:**
- `.skillshare/registry.yaml` contains `name: proj-skill`
- `.skillshare/config.yaml` does NOT contain `skills:`

```bash
cat /tmp/project-test/.skillshare/registry.yaml
grep -c "skills:" /tmp/project-test/.skillshare/config.yaml && echo "FAIL" || echo "PASS"
```

## Pass Criteria

- All 7 steps pass
- Skills never appear in config.yaml after any operation
- Migration works in both directions (old → new, and preserves existing registry)
- Both global and project modes use registry.yaml
