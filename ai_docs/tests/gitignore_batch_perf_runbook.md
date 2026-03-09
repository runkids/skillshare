# CLI E2E Runbook: Gitignore Batch Update Performance

Validates that project-mode install completes within a reasonable time
even when `.skillshare/.gitignore` is very large (100K+ lines).

Root cause: `ReconcileProjectSkills` previously called `UpdateGitIgnore`
per-skill inside WalkDir, re-reading the entire .gitignore each time.
Fix: batch-collect entries, then call `UpdateGitIgnoreBatch` once.

## Scope

- Project-mode repeated install (all skills already exist → skip path)
  completes within 10 seconds, not hanging
- .gitignore managed block is correctly maintained after batch update
- New entries are added when installing to a fresh project
- Existing entries are not duplicated on re-install

## Environment

Run inside devcontainer with `ssenv` HOME isolation.

## Steps

### Step 1: Create isolated project environment

```bash
ssenv create "$ENV_NAME" --init
ssenv enter "$ENV_NAME" -- bash -c '
  mkdir -p ~/test-project
  cd ~/test-project
  ss init -p --no-copy --all-targets --no-git --no-skill
'
```

**Expected**:
- exit_code: 0
- .skillshare

### Step 2: First install (populates .gitignore)

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  cd ~/test-project
  ss install runkids/feature-radar -y -p
'
```

**Expected**:
- exit_code: 0
- Installed

### Step 3: Verify .gitignore content

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  cd ~/test-project
  grep -c "^skills/" .skillshare/.gitignore
  grep "BEGIN SKILLSHARE" .skillshare/.gitignore
  grep "END SKILLSHARE" .skillshare/.gitignore
'
```

**Expected**:
- exit_code: 0
- regex: ^[5-9]\d*$
- BEGIN SKILLSHARE
- END SKILLSHARE

### Step 4: Re-install with large .gitignore (performance test)

Inflate `.gitignore` to simulate a large project, then re-install.

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  cd ~/test-project
  # Inject 100K dummy lines into managed block to simulate large .gitignore
  python3 -c "
for i in range(100000):
    print(f\"skills/dummy-skill-{i}/\")
" > /tmp/dummy_lines.txt
  # Insert dummy lines before END marker
  sed -i "/^# END SKILLSHARE/e cat /tmp/dummy_lines.txt" .skillshare/.gitignore
  wc -l .skillshare/.gitignore
'
```

**Expected**:
- exit_code: 0
- regex: \d{6,}

### Step 5: Timed re-install (must not hang)

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  cd ~/test-project
  timeout 10 ss install runkids/feature-radar -y -p
  echo "EXIT_CODE: $?"
'
```

**Expected**:
- exit_code: 0
- skipped
- EXIT_CODE: 0

### Step 6: No duplicate entries after re-install

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  cd ~/test-project
  # Count occurrences of each real skill entry (should be exactly 1 each)
  for s in feature-radar feature-radar-archive feature-radar-learn feature-radar-ref feature-radar-scan; do
    count=$(grep -c "^skills/$s/$" .skillshare/.gitignore)
    echo "$s: $count"
  done
'
```

**Expected**:
- exit_code: 0
- feature-radar: 1
- feature-radar-archive: 1
- feature-radar-learn: 1
- feature-radar-ref: 1
- feature-radar-scan: 1

## Pass Criteria

- All 6 steps pass
- Step 5 is the critical performance test: must exit 0 within 10s
