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

### Step 1: Verify init — config.yaml has no skills section

```bash
# Setup hook already ran ss init; verify config is clean
cat ~/.config/skillshare/config.yaml
# registry.yaml may not exist yet (created on first install) — that is OK
ls ~/.config/skillshare/registry.yaml 2>/dev/null || echo "registry not yet created (expected)"
```

**Expected:**
- exit_code: 0
- Not skills:

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
- exit_code: 0
- Installed

```bash
cat ~/.config/skillshare/registry.yaml
grep -c "skills:" ~/.config/skillshare/config.yaml && echo "FAIL: config has skills" || echo "PASS: config clean"
```

**Expected:**
- name: test-skill
- PASS: config clean
- Not FAIL

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
- exit_code: 0

```bash
cat ~/.config/skillshare/registry.yaml
grep -c "skills:" ~/.config/skillshare/config.yaml && echo "FAIL: skills still in config" || echo "PASS: migration ok"
```

**Expected:**
- name: legacy-skill
- PASS: migration ok
- Not FAIL

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
- exit_code: 0

```bash
grep "real-skill" ~/.config/skillshare/registry.yaml && echo "PASS" || echo "FAIL: real-skill missing"
grep "stale-skill" ~/.config/skillshare/registry.yaml && echo "FAIL: stale leaked" || echo "PASS: no leak"
```

**Expected:**
- PASS
- PASS: no leak
- Not FAIL stale leaked

### Step 5: Uninstall removes from registry.yaml

```bash
mkdir -p /tmp/remove-me
echo "---
name: remove-me
---
# Remove Me" > /tmp/remove-me/SKILL.md

ss install /tmp/remove-me
ss uninstall remove-me --force
```

**Expected:**
- exit_code: 0

```bash
grep "remove-me" ~/.config/skillshare/registry.yaml && echo "FAIL: still present" || echo "PASS: removed"
```

**Expected:**
- PASS: removed
- Not FAIL still present

### Step 6: Install with --into records group in registry

```bash
mkdir -p /tmp/grouped-skill
echo "---
name: grouped-skill
---
# Grouped" > /tmp/grouped-skill/SKILL.md

ss install /tmp/grouped-skill --into frontend
```

**Expected:**
- exit_code: 0
- Installed

```bash
cat ~/.config/skillshare/registry.yaml
grep "group: frontend" ~/.config/skillshare/registry.yaml && echo "PASS" || echo "FAIL: group missing"
```

**Expected:**
- name: grouped-skill
- group: frontend
- PASS
- Not FAIL

### Step 7: Project mode — registry in .skillshare/

```bash
mkdir -p /tmp/project-test
cd /tmp/project-test
ss init -p --targets claude

mkdir -p /tmp/proj-skill
echo "---
name: proj-skill
---
# Project Skill" > /tmp/proj-skill/SKILL.md

SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 ss install /tmp/proj-skill -p
```

**Expected:**
- exit_code: 0
- Installed

```bash
cat /tmp/project-test/.skillshare/registry.yaml
grep -c "skills:" /tmp/project-test/.skillshare/config.yaml && echo "FAIL: config has skills" || echo "PASS"
```

**Expected:**
- name: proj-skill
- PASS
- Not FAIL

## Pass Criteria

- All 7 steps pass
- Skills never appear in config.yaml after any operation
- Migration works in both directions (old → new, and preserves existing registry)
- Both global and project modes use registry.yaml
