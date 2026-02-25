# CLI E2E Runbook: check per-skill tree hash precision

Validates that `skillshare check` uses per-skill git tree hashes to avoid
false "update_available" when a monorepo has new commits in unrelated subdirs.

**Root cause fixed**: `check` compared repo-level commit hashes. When a
registry repo (many skills, 1 URL) had any new commit, all skills showed
"update_available" even if their files were unchanged. Now `meta.tree_hash`
(subdir-level SHA) is stored at install time and compared via blobless fetch.

## Scope

- Install subdir skills (`ab-test-setup`, `3d-web-experience`) from `sickn33/antigravity-awesome-skills`
- Verify `meta.tree_hash` is written during install
- `check --json` reports correct status after unrelated remote changes
- Backward compat: skill without `tree_hash` falls back to commit comparison

## Environment

Run inside devcontainer with `ssenv` HOME isolation.
Requires network access to GitHub (online test).

## Steps

### 1. Create isolated environment

```bash
ssenv create "$ENV_NAME" --init
```

Expected:
- Environment created successfully

### 2. Install a subdir skill and verify tree_hash in meta

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  ss install github.com/sickn33/antigravity-awesome-skills//skills/active-directory-attacks -g
'
```

Expected:
- Skill installed successfully
- `.skillshare-meta.json` contains `tree_hash` field (non-empty 40-char hex)
- `subdir` field is `skills/active-directory-attacks`
- `repo_url` is set

### 3. Verify meta fields

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  cat ~/.config/skillshare/skills/active-directory-attacks/.skillshare-meta.json
'
```

Expected:
- JSON contains `"tree_hash"` with a 40-character hex string
- JSON contains `"subdir": "skills/active-directory-attacks"`
- JSON contains `"version"` (short commit hash)

### 4. Run check — should be up_to_date (no remote changes since install)

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  ss check active-directory-attacks -g --json
'
```

Expected:
- JSON output has `skills[0].status == "up_to_date"`
- Exit code 0

### 5. Install a second skill from the same repo

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  ss install github.com/sickn33/antigravity-awesome-skills//skills/ab-test-setup -g
'
```

Expected:
- Installed successfully
- Both skills share the same `repo_url`

### 6. Check both skills — both should be up_to_date

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  ss check active-directory-attacks ab-test-setup -g --json
'
```

Expected:
- Both skills have `status: "up_to_date"`
- Progress bar shows skill count (not URL count)

### 7. Simulate stale meta (remove tree_hash) — fallback to commit comparison

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  # Backup original meta
  META=~/.config/skillshare/skills/active-directory-attacks/.skillshare-meta.json
  # Remove tree_hash field from meta to simulate old-format install
  python3 -c "
import json, sys
with open(sys.argv[1]) as f: d = json.load(f)
d.pop(\"tree_hash\", None)
# Also set version to a fake old hash to trigger commit mismatch
d[\"version\"] = \"0000000\"
with open(sys.argv[1], \"w\") as f: json.dump(d, f, indent=2)
" "$META"
  cat "$META"
'
```

Expected:
- Meta JSON has no `tree_hash` field
- `version` is set to fake `0000000`

### 8. Check with stale meta — should fallback to update_available

```bash
ssenv enter "$ENV_NAME" -- bash -c '
  ss check active-directory-attacks -g --json
'
```

Expected:
- `skills[0].status == "update_available"` (commit hash mismatch, no tree_hash to save it)
- This confirms backward compatibility fallback

## Pass Criteria

- Step 2: `tree_hash` written during subdir install
- Step 4: Freshly installed skill is `up_to_date`
- Step 6: Multiple skills from same repo all `up_to_date` in one check
- Step 7-8: Without `tree_hash`, falls back to commit comparison (`update_available`)
