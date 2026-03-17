# .skillignore Source Discovery Runbook

Verifies that `DiscoverSourceSkills` and `DiscoverSourceSkillsLite` respect `.skillignore` files inside tracked repos. Covers the fix for [#83](https://github.com/runkids/skillshare/issues/83): vendored `SKILL.md` files (e.g. inside `.venv/`) should not appear in `doctor`, `status`, `list`, `sync`, or any other source-scanning command.

## Scope

- `internal/sync/sync.go` — `DiscoverSourceSkills`, `DiscoverSourceSkillsLite`
- `internal/skillignore/` — shared `.skillignore` parsing and matching
- Commands affected: `doctor`, `status`, `list`, `sync`, `targets`

## Environment

- Devcontainer with `ss` binary
- ssenv-isolated HOME (created with `--init`)
- Tracked repo simulated with `_team-skills/` directory + `.git/` marker

## Steps

### Step 1: Set up tracked repo with .skillignore and vendored SKILL.md

Creates a tracked repo layout matching the issue reporter's scenario: a `.skillignore` excluding `.venv`, with a vendored `SKILL.md` inside `.venv` and a real skill.

```bash
ss extras remove rules --force -g 2>/dev/null || true
rm -rf ~/.claude/rules 2>/dev/null || true

SOURCE=~/.config/skillshare/skills
REPO="$SOURCE/_team-skills"

mkdir -p "$REPO/.git"
printf ".venv\n" > "$REPO/.skillignore"

mkdir -p "$REPO/coding-standards"
cat > "$REPO/coding-standards/SKILL.md" << 'SKILL'
---
name: coding-standards
description: Team coding standards
---
# Coding Standards
SKILL

mkdir -p "$REPO/.venv/lib/python3.13/site-packages/fastapi/.agents/skills/fastapi"
cat > "$REPO/.venv/lib/python3.13/site-packages/fastapi/.agents/skills/fastapi/SKILL.md" << 'SKILL'
not a real skill - this is vendored package metadata
SKILL

echo "SETUP COMPLETE"
ls "$REPO/.skillignore"
ls "$REPO/coding-standards/SKILL.md"
ls "$REPO/.venv/lib/python3.13/site-packages/fastapi/.agents/skills/fastapi/SKILL.md"
```

Expected:
- SETUP COMPLETE
- exit_code: 0

### Step 2: Verify list only shows the real skill (not vendored)

```bash
ss list --json -g | jq -r '[.[] | select(.name | test("_team-skills")) | .name] | sort | join(",")'
```

Expected:
- _team-skills__coding-standards
- Not fastapi
- Not .venv
- exit_code: 0

### Step 3: Verify status shows exactly 1 skill from tracked repo

```bash
ss status --json -g | jq '.skill_count'
```

Expected:
- 1
- exit_code: 0

### Step 4: Verify doctor source count is 1 (vendored not counted)

The doctor may show "unverifiable" for the real skill (no `.meta.json` for manual skills), but the key assertion is: source count is 1, NOT 2.

```bash
ss doctor -g 2>&1 | grep -oP '\d+ skills\)' | head -1
```

Expected:
- 1 skills)
- exit_code: 0

### Step 5: Verify sync completes and links only real skill

```bash
ss sync --json -g
```

Expected:
- jq: .linked > 0
- exit_code: 0

### Step 6: Verify target received only the real skill (not vendored)

```bash
ls ~/.claude/skills/ | grep team-skills
```

Expected:
- _team-skills__coding-standards
- Not fastapi
- Not venv
- exit_code: 0

### Step 7: Add wildcard pattern and more skills

Tests wildcard pattern support in `.skillignore`.

```bash
SOURCE=~/.config/skillshare/skills
REPO="$SOURCE/_team-skills"

printf ".venv\ntest-*\n" > "$REPO/.skillignore"

mkdir -p "$REPO/test-debug"
cat > "$REPO/test-debug/SKILL.md" << 'SKILL'
---
name: test-debug
description: Debug tool
---
# Debug
SKILL

mkdir -p "$REPO/production-tool"
cat > "$REPO/production-tool/SKILL.md" << 'SKILL'
---
name: production-tool
description: Production tool
---
# Production
SKILL

echo "ADDED SKILLS"
cat "$REPO/.skillignore"
```

Expected:
- ADDED SKILLS
- .venv
- test-*
- exit_code: 0

### Step 8: Verify list respects both .skillignore patterns

```bash
ss list --json -g | jq -r '[.[] | select(.name | test("_team-skills")) | .name] | sort | join(",")'
```

Expected:
- _team-skills__coding-standards,_team-skills__production-tool
- Not test-debug
- Not fastapi
- Not venv
- exit_code: 0

### Step 9: Verify sync only creates symlinks for non-ignored skills

```bash
ss sync -g 2>/dev/null
ls ~/.claude/skills/ | grep team-skills | sort
```

Expected:
- _team-skills__coding-standards
- _team-skills__production-tool
- Not test-debug
- Not fastapi
- exit_code: 0

### Step 10: Verify non-tracked repos are unaffected by .skillignore

A `.skillignore` in a non-tracked directory (no `_` prefix) should have no effect — all skills are still discovered.

```bash
SOURCE=~/.config/skillshare/skills

mkdir -p "$SOURCE/my-group/.git"
printf "hidden\n" > "$SOURCE/my-group/.skillignore"

mkdir -p "$SOURCE/my-group/visible"
cat > "$SOURCE/my-group/visible/SKILL.md" << 'SKILL'
---
name: visible
description: Should appear
---
# Visible
SKILL

mkdir -p "$SOURCE/my-group/hidden"
cat > "$SOURCE/my-group/hidden/SKILL.md" << 'SKILL'
---
name: hidden
description: Not tracked repo so skillignore should not apply
---
# Hidden
SKILL

ss list --json -g | jq -r '[.[] | select(.name | test("my-group")) | .name] | sort | join(",")'
```

Expected:
- my-group__hidden,my-group__visible
- exit_code: 0

### Step 11: Final doctor source count includes all non-ignored skills

```bash
ss doctor -g 2>&1 | grep -oP '\d+ skills\)' | head -1
```

Expected:
- 4 skills)
- exit_code: 0

### Step 12: Create root-level .skillignore with wildcard pattern

```bash
SOURCE=~/.config/skillshare/skills

mkdir -p "$SOURCE/draft-wip"
cat > "$SOURCE/draft-wip/SKILL.md" << 'SKILL'
---
name: draft-wip
description: Work in progress
---
# Draft
SKILL

mkdir -p "$SOURCE/draft-experiment"
cat > "$SOURCE/draft-experiment/SKILL.md" << 'SKILL'
---
name: draft-experiment
description: Experiment
---
# Experiment
SKILL

printf "draft-*\n" > "$SOURCE/.skillignore"
echo "ROOT SKILLIGNORE CREATED"
cat "$SOURCE/.skillignore"
```

Expected:
- ROOT SKILLIGNORE CREATED
- draft-*
- exit_code: 0

### Step 13: Verify root .skillignore hides matching skills from list

```bash
ss list --json -g | jq -r '[.[] | select(.name | test("draft")) | .name] | length'
```

Expected:
- 0
- exit_code: 0

### Step 14: Root .skillignore skips entire tracked repo

```bash
SOURCE=~/.config/skillshare/skills

printf "draft-*\n_team-skills\n" > "$SOURCE/.skillignore"

ss list --json -g | jq -r '[.[] | select(.name | test("_team-skills")) | .name] | length'
```

Expected:
- 0
- exit_code: 0

### Step 15: Doctor source count drops with root .skillignore

```bash
ss doctor -g 2>&1 | grep -oP '\d+ skills\)' | head -1
```

Expected:
- regex: \d+ skills\)
- exit_code: 0

### Step 16: Remove root .skillignore — everything comes back

```bash
SOURCE=~/.config/skillshare/skills
rm -f "$SOURCE/.skillignore"

ss list --json -g | jq -r '[.[] | select(.name | test("draft|_team-skills")) | .name] | sort | join(",")'
```

Expected:
- _team-skills__coding-standards
- _team-skills__production-tool
- draft-experiment
- draft-wip
- exit_code: 0

## Pass Criteria

- All 16 steps pass
- `.skillignore` patterns (exact and wildcard) are respected by list, status, doctor, sync
- Vendored SKILL.md files in `.venv/` and `test-*` dirs never appear in any command output
- Non-tracked repos are unaffected by repo-level `.skillignore`
- Root-level `.skillignore` hides skills globally (tracked and non-tracked)
- Removing root `.skillignore` restores all skills
- Target directories only receive symlinks for non-ignored skills
