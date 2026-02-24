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
- Environment created with all targets

### 2. Install --all (default threshold = CRITICAL)

```bash
docker exec $CONTAINER env SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
  ssenv enter "$ENV_NAME" -- \
  ss install -g sickn33/antigravity-awesome-skills/skills --all 2>&1 | tee /tmp/audit-install.log
```

Expected:
- Exit code 0 (batch succeeds when some skills install; blocked count shown as warning)
- Output contains `Blocked / Failed` section
- Output contains `blocked by security audit`
- Output contains `CRITICAL` severity label
- Output contains `Installed` section (many skills pass audit)
- Output contains `--audit-verbose` hint
- No raw git progress lines (Enumerating objects, etc.)

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

### 3. Count installed vs blocked skills

```bash
docker exec $CONTAINER ssenv enter "$ENV_NAME" -- bash -c '
  SKILLS=$(find ~/.config/skillshare/skills -mindepth 1 -maxdepth 1 -type d 2>/dev/null | wc -l)
  echo "Installed skill dirs: $SKILLS"
  test "$SKILLS" -gt 0 && echo "PASS: skills installed" || echo "FAIL: no skills installed"
'
```

Expected:
- At least some skills installed (hundreds expected from ~920 in repo)

### 4. Reinstall with --force (prepare for update test)

```bash
docker exec $CONTAINER env SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
  ssenv enter "$ENV_NAME" -- \
  ss install -g sickn33/antigravity-awesome-skills/skills --all --force 2>&1 | tee /tmp/audit-force-install.log
```

Expected:
- Exit code 0 (--force bypasses audit blocks)
- Output contains `Installed`
- Output may contain `Audit Warnings` section

Verify:

```bash
docker exec $CONTAINER ssenv enter "$ENV_NAME" -- bash -c '
  LOG=/tmp/audit-force-install.log
  grep -q "Installed" "$LOG"                   && echo "PASS: installed section"    || echo "FAIL: no installed section"
  ! grep -q "Blocked / Failed" "$LOG"          && echo "PASS: no blocked (forced)" || echo "FAIL: blocked despite --force"
'
```

### 5. Update --all (nothing changed upstream)

```bash
docker exec $CONTAINER env SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
  ssenv enter "$ENV_NAME" -- \
  ss update -g --all 2>&1 | tee /tmp/audit-update.log
```

Expected:
- Exit code 0 (nothing changed, no new findings)
- Output contains `Audit` (audit section present)
- Output contains `skipped` (already up to date)
- No `Blocked / Failed` section (nothing changed)
- No `Blocked / Rolled Back` section (nothing changed)

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

### 6. Install --all with --audit-verbose --force

Note: needs `--force` because skills already exist from Steps 2-4.

```bash
docker exec $CONTAINER env SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
  ssenv enter "$ENV_NAME" -- \
  ss install -g sickn33/antigravity-awesome-skills/skills --all --audit-verbose --force 2>&1 | tee /tmp/audit-verbose.log
```

Expected:
- Verbose output expands HIGH/CRITICAL detail per skill
- Finding lines contain severity + message + file:line format (e.g., `audit HIGH: Disk overwrite command (CHANGELOG.md:737)`)
- HIGH findings appear (not just CRITICAL)
- Audit section header: `Audit Warnings`

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
- `Next Steps` section present
- `finding(s)` structured format present

## Pass Criteria

- [ ] Step 1: Environment created
- [ ] Step 2: `install --all` produces Blocked/Failed + Installed sections + CRITICAL label + verbose hint
- [ ] Step 3: Skills installed to disk
- [ ] Step 4: `install --all --force` succeeds, no blocked section
- [ ] Step 5: `update --all` has `Audit Findings` section, no blocked sections (nothing changed)
- [ ] Step 6: `--audit-verbose --force` shows per-skill HIGH/CRITICAL detail lines
- [ ] Step 7: Output uses structured sections (Next Steps, finding(s) format)
