# CLI E2E Runbook: check per-skill tree hash precision

Validates that `skillshare check` correctly reports `up_to_date` for freshly
installed subdir skills from a monorepo, and falls back to commit comparison
when `tree_hash` is absent (simulated stale/old-format meta).

**Background**: `check` uses per-skill tree hashes (when available) to avoid
false "update_available". When the GitHub API download path is used (no git
repo on disk), `tree_hash` is omitted and `check` falls back to commit-level
comparison. Both paths are covered.

## Scope

- Install subdir skills (`active-directory-attacks`, `ab-test-setup`) from `sickn33/antigravity-awesome-skills`
- Verify meta fields (`subdir`, `version`, `file_hashes`) are written during install
- `check --json` reports correct status for freshly installed skills
- Backward compat: skill with stale `version` correctly reports `update_available`

## Environment

Run inside devcontainer. Setup hook in `runbook.json` handles `ss init -g`.
Requires network access to GitHub (online test).

## Steps

### 1. Install a subdir skill

```bash
ss install github.com/sickn33/antigravity-awesome-skills//skills/active-directory-attacks -g
```

Expected:
- exit_code: 0
- Installed

### 2. Verify meta fields

```bash
META=~/.config/skillshare/skills/active-directory-attacks/.skillshare-meta.json
python3 -c "
import json, sys
with open(sys.argv[1]) as f: d = json.load(f)
checks = []
# subdir must be present and contain the skill path
if 'active-directory-attacks' in d.get('subdir', ''):
    checks.append('SUBDIR_OK')
else:
    checks.append('SUBDIR_FAIL: ' + d.get('subdir', '<missing>'))
# version must be a hex string (may be abbreviated)
import re
v = d.get('version', '')
if re.fullmatch(r'[0-9a-f]+', v):
    checks.append('VERSION_OK')
else:
    checks.append('VERSION_FAIL: ' + v)
# file_hashes must be present
if d.get('file_hashes') and len(d['file_hashes']) > 0:
    checks.append('HASHES_OK')
else:
    checks.append('HASHES_FAIL')
# tree_hash is optional (absent for GitHub API downloads)
if d.get('tree_hash'):
    checks.append('TREE_HASH_PRESENT')
else:
    checks.append('TREE_HASH_ABSENT')
for c in checks:
    print(c)
" "$META"
```

Expected:
- exit_code: 0
- SUBDIR_OK
- VERSION_OK
- HASHES_OK

### 3. Run check — should be up_to_date (no remote changes since install)

```bash
ss check active-directory-attacks -g --json
```

Expected:
- exit_code: 0
- jq: .skills[0].status == "up_to_date"

### 4. Install a second skill from the same repo

```bash
ss install github.com/sickn33/antigravity-awesome-skills//skills/ab-test-setup -g
```

Expected:
- exit_code: 0
- Installed

### 5. Check both skills — both should be up_to_date

```bash
ss check active-directory-attacks ab-test-setup -g --json
```

Expected:
- exit_code: 0
- jq: [.skills[] | select(.status == "up_to_date")] | length == 2

### 6. Simulate stale meta (set fake version) — fallback to commit comparison

```bash
META=~/.config/skillshare/skills/active-directory-attacks/.skillshare-meta.json
python3 -c "
import json, sys
with open(sys.argv[1]) as f: d = json.load(f)
d.pop('tree_hash', None)
d['version'] = '0000000'
with open(sys.argv[1], 'w') as f: json.dump(d, f, indent=2)
# Verify the write
with open(sys.argv[1]) as f: d2 = json.load(f)
if d2.get('version') == '0000000' and 'tree_hash' not in d2:
    print('META_MODIFIED_OK')
else:
    print('META_MODIFIED_FAIL')
    print(json.dumps(d2, indent=2))
" "$META"
```

Expected:
- exit_code: 0
- META_MODIFIED_OK

### 7. Check with stale meta — should fallback to update_available

```bash
ss check active-directory-attacks -g --json
```

Expected:
- exit_code: 0
- jq: .skills[0].status == "update_available"

## Pass Criteria

- Step 1-2: Meta fields (`subdir`, `version`, `file_hashes`) written during subdir install
- Step 3: Freshly installed skill is `up_to_date`
- Step 5: Multiple skills from same repo all `up_to_date` in one check
- Step 6-7: With stale version hash, falls back to commit comparison (`update_available`)
