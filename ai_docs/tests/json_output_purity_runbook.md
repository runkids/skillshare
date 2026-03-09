# JSON Output Purity E2E Runbook

Validates that `--json` mode outputs pure JSON to stdout (no UI noise) and that non-JSON mode remains unaffected by the `--json` refactoring.

## Scope

- `ss status --json` / `ss status --project --json`
- `ss sync --json` / `ss sync --all --json`
- `ss install --json` (local path)
- `ss uninstall --json` (success + error paths)
- `ss diff --json`
- `ss collect --json`
- `ss update --json --dry-run`
- Non-JSON regression: `ss status`, `ss sync`, `ss list` still produce human-readable output

## Environment

- ssenv isolated environment with `--init`
- Requires: `jq`

## Steps

### Step 1: Create environment and skills

```bash
ssenv create json-purity --init
ssenv enter json-purity -- bash -c '
  mkdir -p ~/.config/skillshare/skills/alpha
  echo "# Alpha skill" > ~/.config/skillshare/skills/alpha/SKILL.md
  mkdir -p ~/.config/skillshare/skills/beta
  echo "# Beta skill" > ~/.config/skillshare/skills/beta/SKILL.md
  ss sync
'
```

Expected:
- exit_code: 0
- Synced

### Step 2: status --json outputs pure JSON

```bash
ssenv enter json-purity -- bash -c '
  OUTPUT=$(ss status --json)
  echo "$OUTPUT" | jq -e ".skill_count >= 2" && echo "FIELD_CHECK=OK" || echo "FIELD_CHECK=FAIL"
  echo "$OUTPUT" | jq -e ".targets | length >= 1" && echo "TARGETS=OK" || echo "TARGETS=FAIL"
  echo "$OUTPUT" | jq -e ".audit" && echo "AUDIT=OK" || echo "AUDIT=FAIL"
  echo "$OUTPUT" | jq -e ".version" && echo "VERSION=OK" || echo "VERSION=FAIL"
  # Verify first char is { (pure JSON, no banner/spinner)
  FIRST=$(echo "$OUTPUT" | head -c1)
  [ "$FIRST" = "{" ] && echo "PURE_JSON=OK" || echo "PURE_JSON=FAIL"
'
```

Expected:
- exit_code: 0
- FIELD_CHECK=OK
- TARGETS=OK
- AUDIT=OK
- VERSION=OK
- PURE_JSON=OK

### Step 3: sync --json outputs pure JSON

```bash
ssenv enter json-purity -- bash -c '
  OUTPUT=$(ss sync --json)
  echo "$OUTPUT" | jq -e ".targets >= 1" && echo "TARGETS=OK" || echo "TARGETS=FAIL"
  echo "$OUTPUT" | jq -e ".linked >= 0" && echo "LINKED=OK" || echo "LINKED=FAIL"
  echo "$OUTPUT" | jq -e ".details | length >= 1" && echo "DETAILS=OK" || echo "DETAILS=FAIL"
  echo "$OUTPUT" | jq -e ".duration" && echo "DURATION=OK" || echo "DURATION=FAIL"
  FIRST=$(echo "$OUTPUT" | head -c1)
  [ "$FIRST" = "{" ] && echo "PURE_JSON=OK" || echo "PURE_JSON=FAIL"
'
```

Expected:
- exit_code: 0
- TARGETS=OK
- LINKED=OK
- DETAILS=OK
- DURATION=OK
- PURE_JSON=OK

### Step 4: sync --all --json outputs pure JSON

```bash
ssenv enter json-purity -- bash -c '
  OUTPUT=$(ss sync --all --json)
  echo "$OUTPUT" | jq -e "." > /dev/null 2>&1 && echo "VALID_JSON=OK" || echo "VALID_JSON=FAIL"
  FIRST=$(echo "$OUTPUT" | head -c1)
  [ "$FIRST" = "{" ] && echo "PURE_JSON=OK" || echo "PURE_JSON=FAIL"
'
```

Expected:
- exit_code: 0
- VALID_JSON=OK
- PURE_JSON=OK

### Step 5: list --json outputs pure JSON

```bash
ssenv enter json-purity -- bash -c '
  OUTPUT=$(ss list --json)
  echo "$OUTPUT" | jq -e "length >= 2" && echo "COUNT=OK" || echo "COUNT=FAIL"
  echo "$OUTPUT" | jq -e ".[0].name" && echo "HAS_NAME=OK" || echo "HAS_NAME=FAIL"
  FIRST=$(echo "$OUTPUT" | head -c1)
  [ "$FIRST" = "[" ] && echo "PURE_JSON=OK" || echo "PURE_JSON=FAIL"
'
```

Expected:
- exit_code: 0
- COUNT=OK
- HAS_NAME=OK
- PURE_JSON=OK

### Step 6: diff --json outputs pure JSON

```bash
ssenv enter json-purity -- bash -c '
  OUTPUT=$(ss diff --json)
  echo "$OUTPUT" | jq -e ".targets" && echo "TARGETS=OK" || echo "TARGETS=FAIL"
  echo "$OUTPUT" | jq -e ".duration" && echo "DURATION=OK" || echo "DURATION=FAIL"
  FIRST=$(echo "$OUTPUT" | head -c1)
  [ "$FIRST" = "{" ] && echo "PURE_JSON=OK" || echo "PURE_JSON=FAIL"
'
```

Expected:
- exit_code: 0
- TARGETS=OK
- DURATION=OK
- PURE_JSON=OK

### Step 7: install --json (local path) outputs pure JSON

Note: `ss install <path>` uses the **directory name** as the skill name, not the `name` field in SKILL.md.

```bash
ssenv enter json-purity -- bash -c '
  mkdir -p /tmp/ext-skill
  echo -e "---\nname: ext-test\n---\n# External" > /tmp/ext-skill/SKILL.md
  OUTPUT=$(ss install /tmp/ext-skill --json --force)
  echo "$OUTPUT" | jq -e ".skills | length >= 1" && echo "SKILLS=OK" || echo "SKILLS=FAIL"
  FIRST=$(echo "$OUTPUT" | head -c1)
  [ "$FIRST" = "{" ] && echo "PURE_JSON=OK" || echo "PURE_JSON=FAIL"
'
```

Expected:
- exit_code: 0
- SKILLS=OK
- PURE_JSON=OK

### Step 8: uninstall --json outputs pure JSON

```bash
ssenv enter json-purity -- bash -c '
  OUTPUT=$(ss uninstall ext-skill --json)
  echo "$OUTPUT" | jq -e ".removed | length >= 1" && echo "REMOVED=OK" || echo "REMOVED=FAIL"
  FIRST=$(echo "$OUTPUT" | head -c1)
  [ "$FIRST" = "{" ] && echo "PURE_JSON=OK" || echo "PURE_JSON=FAIL"
'
```

Expected:
- exit_code: 0
- REMOVED=OK
- PURE_JSON=OK

### Step 9: uninstall --json error path returns JSON error envelope

```bash
ssenv enter json-purity -- bash -c '
  OUTPUT=$(ss uninstall nonexistent-skill --json 2>&1)
  EXIT=$?
  [ "$EXIT" -ne 0 ] && echo "EXIT_CODE=OK" || echo "EXIT_CODE=FAIL"
  echo "$OUTPUT" | jq -e ".error" && echo "ERROR_FIELD=OK" || echo "ERROR_FIELD=FAIL"
  FIRST=$(echo "$OUTPUT" | head -c1)
  [ "$FIRST" = "{" ] && echo "PURE_JSON=OK" || echo "PURE_JSON=FAIL"
'
```

Expected:
- exit_code: 0
- EXIT_CODE=OK
- ERROR_FIELD=OK
- PURE_JSON=OK

### Step 10: collect --json outputs pure JSON (no local skills)

```bash
ssenv enter json-purity -- bash -c '
  OUTPUT=$(ss collect --json --all)
  echo "$OUTPUT" | jq -e "." > /dev/null 2>&1 && echo "VALID_JSON=OK" || echo "VALID_JSON=FAIL"
  echo "$OUTPUT" | jq -e ".pulled" && echo "PULLED=OK" || echo "PULLED=FAIL"
  FIRST=$(echo "$OUTPUT" | head -c1)
  [ "$FIRST" = "{" ] && echo "PURE_JSON=OK" || echo "PURE_JSON=FAIL"
'
```

Expected:
- exit_code: 0
- VALID_JSON=OK
- PULLED=OK
- PURE_JSON=OK

### Step 11: update --json --dry-run outputs pure JSON

```bash
ssenv enter json-purity -- bash -c '
  OUTPUT=$(ss update --all --json --dry-run)
  echo "$OUTPUT" | jq -e ".dry_run == true" && echo "DRY_RUN=OK" || echo "DRY_RUN=FAIL"
  echo "$OUTPUT" | jq -e ".skipped >= 0" && echo "SKIPPED=OK" || echo "SKIPPED=FAIL"
  FIRST=$(echo "$OUTPUT" | head -c1)
  [ "$FIRST" = "{" ] && echo "PURE_JSON=OK" || echo "PURE_JSON=FAIL"
'
```

Expected:
- exit_code: 0
- DRY_RUN=OK
- SKIPPED=OK
- PURE_JSON=OK

### Step 12: Non-JSON regression — status still shows human-readable output

```bash
ssenv enter json-purity -- bash -c '
  OUTPUT=$(ss status 2>&1)
  echo "$OUTPUT" | grep -q "skillshare" && echo "BANNER=OK" || echo "BANNER=FAIL"
  echo "$OUTPUT" | grep -q "source" && echo "SOURCE=OK" || echo "SOURCE=FAIL"
  echo "$OUTPUT" | grep -q "target" && echo "TARGET=OK" || echo "TARGET=FAIL"
  # Must NOT start with { (not JSON)
  FIRST=$(echo "$OUTPUT" | head -c1)
  [ "$FIRST" != "{" ] && echo "NOT_JSON=OK" || echo "NOT_JSON=FAIL"
'
```

Expected:
- exit_code: 0
- BANNER=OK
- SOURCE=OK
- TARGET=OK
- NOT_JSON=OK

### Step 13: Non-JSON regression — sync still shows human-readable output

```bash
ssenv enter json-purity -- bash -c '
  OUTPUT=$(ss sync 2>&1)
  FIRST=$(echo "$OUTPUT" | head -c1)
  [ "$FIRST" != "{" ] && echo "NOT_JSON=OK" || echo "NOT_JSON=FAIL"
  echo "$OUTPUT" | grep -qi "sync\|linked\|target\|skill" && echo "READABLE=OK" || echo "READABLE=FAIL"
'
```

Expected:
- exit_code: 0
- NOT_JSON=OK
- READABLE=OK

### Step 14: Non-JSON regression — list still shows human-readable output

```bash
ssenv enter json-purity -- bash -c '
  OUTPUT=$(ss list 2>&1)
  FIRST=$(echo "$OUTPUT" | head -c1)
  [ "$FIRST" != "[" ] && echo "NOT_JSON=OK" || echo "NOT_JSON=FAIL"
  echo "$OUTPUT" | grep -q "alpha\|beta" && echo "SKILLS_SHOWN=OK" || echo "SKILLS_SHOWN=FAIL"
'
```

Expected:
- exit_code: 0
- NOT_JSON=OK
- SKILLS_SHOWN=OK

### Step 15: status --project --json in project mode

```bash
SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 ssenv enter json-purity -- bash -c '
  mkdir -p /tmp/test-proj/.skillshare/skills/proj-skill
  echo "# Proj Skill" > /tmp/test-proj/.skillshare/skills/proj-skill/SKILL.md
  mkdir -p /tmp/test-proj/.claude/commands
  cat > /tmp/test-proj/.skillshare/config.yaml << EOF
targets:
  - name: claude
    path: /tmp/test-proj/.claude/commands
EOF
  cd /tmp/test-proj
  OUTPUT=$(ss status --project --json)
  echo "$OUTPUT" | jq -e ".skill_count >= 1" && echo "SKILL_COUNT=OK" || echo "SKILL_COUNT=FAIL"
  echo "$OUTPUT" | jq -e ".version" && echo "VERSION=OK" || echo "VERSION=FAIL"
  FIRST=$(echo "$OUTPUT" | head -c1)
  [ "$FIRST" = "{" ] && echo "PURE_JSON=OK" || echo "PURE_JSON=FAIL"
'
```

Expected:
- exit_code: 0
- SKILL_COUNT=OK
- VERSION=OK
- PURE_JSON=OK

### Step 16: target list --json outputs pure JSON

```bash
ssenv enter json-purity -- bash -c '
  OUTPUT=$(ss target list --json)
  echo "$OUTPUT" | jq -e ".targets | length >= 1" && echo "HAS_TARGETS=OK" || echo "HAS_TARGETS=FAIL"
  FIRST=$(echo "$OUTPUT" | head -c1)
  [ "$FIRST" = "{" ] && echo "PURE_JSON=OK" || echo "PURE_JSON=FAIL"
'
```

Expected:
- exit_code: 0
- HAS_TARGETS=OK
- PURE_JSON=OK

## Pass Criteria

- All 16 steps PASS
- All `--json` outputs start with `{` or `[` (pure JSON)
- Error paths return JSON error envelope with non-zero exit code
- Non-JSON commands still produce human-readable output
