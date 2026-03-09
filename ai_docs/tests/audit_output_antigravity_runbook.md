# CLI E2E Runbook: Audit Output for Batch Install & Update

Validates audit output richness and parity between `install --all` and `update --all`
using a large real-world skill repo (`sickn33/antigravity-awesome-skills`).

## Scope

- `install --all` from monorepo subdir produces structured audit output
- Blocked skills section appears with severity breakdown
- Installed skills section shows successful installs
- Audit Warnings section shows finding summaries
- `update --all` after force-install produces audit output with similar richness
- No raw error dumps (structured sections only)
- `--audit-verbose` hint is shown for blocked skills
- Parity: install uses `Audit Warnings` section, update uses `Audit Findings` section; both share `finding(s) across` format

## Environment

Run inside devcontainer with `ssenv` HOME isolation.
Requires network access (GitHub clone).

## Steps

### 1. Create isolated environment

```bash
ENV_NAME="e2e-audit-output-$(date +%Y%m%d-%H%M%S)"
docker exec $CONTAINER ssenv create "$ENV_NAME" --init
```

Expected:
- exit_code: 0

### 2. Install --all (default threshold = CRITICAL)

```bash
docker exec $CONTAINER env SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
  ssenv enter "$ENV_NAME" -- \
  ss install -g sickn33/antigravity-awesome-skills/skills --all 2>&1 | tee /tmp/audit-install.log
```

Expected:
- exit_code: 0
- Blocked / Failed
- blocked by security audit
- CRITICAL
- Installed
- --audit-verbose
- Not Enumerating objects

Verify:

```bash
docker exec $CONTAINER ssenv enter "$ENV_NAME" -- bash -c '
  LOG=/tmp/audit-install.log
  grep -q "Blocked / Failed" "$LOG"          && echo "PASS: blocked section"      || echo "FAIL: no blocked section"
  grep -q "blocked by security audit" "$LOG" && echo "PASS: blocked message"      || echo "FAIL: no blocked message"
  grep -q "CRITICAL" "$LOG"                  && echo "PASS: CRITICAL label"       || echo "FAIL: no CRITICAL label"
  grep -q "Installed" "$LOG"                 && echo "PASS: installed section"    || echo "FAIL: no installed section"
  grep -q "\-\-audit-verbose" "$LOG"         && echo "PASS: verbose hint"         || echo "FAIL: no verbose hint"
  ! grep -qE "(Enumerating objects|Counting objects|Receiving objects)" "$LOG" \
                                             && echo "PASS: no git progress"      || echo "FAIL: git progress leaked"
'
```

Expected:
- exit_code: 0
- PASS: blocked section
- PASS: blocked message
- PASS: CRITICAL label
- PASS: installed section
- PASS: verbose hint
- PASS: no git progress
- Not FAIL

### 3. Count installed vs blocked skills

```bash
docker exec $CONTAINER ssenv enter "$ENV_NAME" -- bash -c '
  SKILLS=$(find ~/.config/skillshare/skills -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l)
  echo "Installed skill dirs: $SKILLS"
  test "$SKILLS" -gt 0 && echo "PASS: skills installed" || echo "FAIL: no skills installed"
'
```

Expected:
- exit_code: 0
- PASS: skills installed
- Not FAIL

### 4. Reinstall with --force (prepare for update test)

```bash
docker exec $CONTAINER env SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
  ssenv enter "$ENV_NAME" -- \
  ss install -g sickn33/antigravity-awesome-skills/skills --all --force 2>&1 | tee /tmp/audit-force-install.log
```

Expected:
- exit_code: 0
- Installed

Verify:

```bash
docker exec $CONTAINER ssenv enter "$ENV_NAME" -- bash -c '
  LOG=/tmp/audit-force-install.log
  grep -q "Installed" "$LOG"                   && echo "PASS: installed section"    || echo "FAIL: no installed section"
  ! grep -q "Blocked / Failed" "$LOG"          && echo "PASS: no blocked (forced)" || echo "FAIL: blocked despite --force"
'
```

Expected:
- exit_code: 0
- PASS: installed section
- PASS: no blocked (forced)
- Not FAIL

### 5. Update --all (nothing changed upstream)

```bash
docker exec $CONTAINER env SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
  ssenv enter "$ENV_NAME" -- \
  ss update -g --all 2>&1 | tee /tmp/audit-update.log
```

Expected:
- exit_code: 0
- regex: (?i)audit
- skipped
- Not Blocked / Failed
- Not Blocked / Rolled Back

Verify:

```bash
docker exec $CONTAINER ssenv enter "$ENV_NAME" -- bash -c '
  LOG=/tmp/audit-update.log
  grep -qi "audit" "$LOG"                       && echo "PASS: audit present"     || echo "FAIL: no audit section"
  grep -q "skipped" "$LOG"                      && echo "PASS: skipped present"   || echo "FAIL: no skipped"
  ! grep -q "Blocked / Failed" "$LOG"           && echo "PASS: no install block"  || echo "FAIL: install block in update"
  ! grep -q "Blocked / Rolled Back" "$LOG"      && echo "PASS: no update block"   || echo "FAIL: update blocked"
'
```

Expected:
- exit_code: 0
- PASS: audit present
- PASS: skipped present
- PASS: no install block
- PASS: no update block
- Not FAIL

### 6. Install --all with --audit-verbose --force

Note: needs `--force` because skills already exist from Steps 2-4.

```bash
docker exec $CONTAINER env SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
  ssenv enter "$ENV_NAME" -- \
  ss install -g sickn33/antigravity-awesome-skills/skills --all --audit-verbose --force 2>&1 | tee /tmp/audit-verbose.log
```

Expected:
- exit_code: 0
- regex: HIGH|CRITICAL
- Audit Warnings

Verify:

```bash
docker exec $CONTAINER ssenv enter "$ENV_NAME" -- bash -c '
  LOG=/tmp/audit-verbose.log
  grep -qE "HIGH|CRITICAL" "$LOG"                  && echo "PASS: severity labels"     || echo "FAIL: no severity labels"
  grep -q "audit HIGH:" "$LOG"                     && echo "PASS: verbose detail"      || echo "FAIL: no verbose detail"
  grep -q "audit finding line(s)" "$LOG"           && echo "PASS: finding line count"  || echo "FAIL: no finding count"
  grep -q "HIGH/CRITICAL detail" "$LOG"            && echo "PASS: expanded section"    || echo "FAIL: no expanded section"
'
```

Expected:
- exit_code: 0
- PASS: severity labels
- PASS: verbose detail
- Not FAIL

### 7. Verify audit output has structured sections (no raw dumps)

```bash
docker exec $CONTAINER ssenv enter "$ENV_NAME" -- bash -c '
  LOG=/tmp/audit-install.log
  # Should have section labels, not raw error text
  grep -q "Next Steps" "$LOG"           && echo "PASS: next steps section"  || echo "FAIL: no next steps"
  grep -qE "finding\(s\)" "$LOG"        && echo "PASS: finding(s) format"  || echo "FAIL: no finding(s) format"
'
```

Expected:
- exit_code: 0
- PASS: next steps section
- regex: PASS: finding\(s\) format
- Not FAIL

## Pass Criteria

- [ ] Step 1: Environment created
- [ ] Step 2: `install --all` produces Blocked/Failed + Installed sections + CRITICAL label + verbose hint
- [ ] Step 3: Skills installed to disk
- [ ] Step 4: `install --all --force` succeeds, no blocked section
- [ ] Step 5: `update --all` has `Audit Findings` section, no blocked sections (nothing changed)
- [ ] Step 6: `--audit-verbose --force` shows per-skill HIGH/CRITICAL detail lines
- [ ] Step 7: Output uses structured sections (Next Steps, finding(s) format)
