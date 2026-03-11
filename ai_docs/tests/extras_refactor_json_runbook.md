# CLI E2E Runbook: Extras Refactor — JSON Purity & New Path Layout

Validates the refactored extras sync after consolidating duplicate logic:
`sync --all --json` outputs a single valid JSON object with embedded `extras`,
extras use the `extras/<name>/` directory layout, and shared helpers
(`EffectiveMode`, `CheckSyncStatus`, `ValidateExtraMode`) work correctly.

**Origin**: refactor(extras) commit — consolidate duplicate logic and fix
sync --all --json output.

## Scope

- `sync --all --json` produces a single valid JSON (not two concatenated objects)
- The JSON includes an `extras` array when extras are configured
- Extras source uses `~/.config/skillshare/extras/<name>/` path (not flat)
- `extras init` validates mode correctly (reject invalid, accept valid)
- `extras list --json` returns valid JSON with correct status
- `sync extras --json` returns valid JSON
- Auto-migration from legacy flat path to `extras/<name>/`
- Copy mode creates real files (not symlinks), re-syncs on content change
- Project mode (`-p`): init, sync, list, and sync --all --json

## Environment

Run inside devcontainer.
If `ss` alias is unavailable, replace `ss` with `skillshare`.

## Steps

### 1. Setup: create extras source with new path layout

```bash
mkdir -p ~/.config/skillshare/extras/rules
echo "# TDD rules" > ~/.config/skillshare/extras/rules/tdd.md
echo "# Error handling" > ~/.config/skillshare/extras/rules/errors.md
```

Expected:
- exit_code: 0

### 2. Configure extras in config.yaml

```bash
cat >> ~/.config/skillshare/config.yaml << 'CONF'

extras:
  - name: rules
    targets:
      - path: ~/.claude/rules
CONF
```

Expected:
- exit_code: 0

### 3. sync extras --json: valid JSON output

```bash
ss sync extras --json | python3 -c "import sys,json; d=json.load(sys.stdin); print('valid_json=yes'); print(f'extras_count={len(d[\"extras\"])}')"
```

Expected:
- exit_code: 0
- valid_json=yes
- extras_count=1

### 4. sync --all --json: single valid JSON with embedded extras

```bash
ss sync --all --json 2>/dev/null | python3 -c "
import sys, json
data = sys.stdin.read().strip()
obj = json.loads(data)
print('single_json=yes')
has_targets = 'targets' in obj
has_extras = 'extras' in obj
print(f'has_targets={has_targets}')
print(f'has_extras={has_extras}')
if has_extras:
    print(f'extras_len={len(obj[\"extras\"])}')
    for e in obj['extras']:
        print(f'extra_name={e[\"name\"]}')
"
```

Expected:
- exit_code: 0
- single_json=yes
- has_targets=True
- has_extras=True
- extras_len=1
- extra_name=rules

### 5. Verify extras source path uses extras/<name>/ layout

```bash
test -d ~/.config/skillshare/extras/rules && echo "new_layout=yes" || echo "new_layout=no"
ls ~/.config/skillshare/extras/rules/
```

Expected:
- exit_code: 0
- new_layout=yes
- tdd.md
- errors.md

### 6. Verify symlinks point to extras/<name>/ source

```bash
readlink ~/.claude/rules/tdd.md
```

Expected:
- exit_code: 0
- regex: skillshare/extras/rules/tdd\.md

### 7. extras list --json: valid JSON with sync status

```bash
ss extras list --json | python3 -c "
import sys, json
entries = json.load(sys.stdin)
print('valid_json=yes')
for e in entries:
    print(f'name={e[\"name\"]}')
    print(f'source_exists={e[\"source_exists\"]}')
    print(f'file_count={e[\"file_count\"]}')
    for t in e.get('targets', []):
        print(f'target_status={t[\"status\"]}')
        print(f'target_mode={t[\"mode\"]}')
"
```

Expected:
- exit_code: 0
- valid_json=yes
- name=rules
- source_exists=True
- file_count=2
- target_status=synced
- target_mode=merge

### 8. extras init: rejects invalid mode

```bash
ss extras init test-extra --target /tmp/test --mode invalid 2>&1 || true
```

Expected:
- regex: invalid mode

### 9. extras init: accepts valid modes

```bash
ss extras init copy-test --target /tmp/copy-target --mode copy 2>&1
```

Expected:
- exit_code: 0
- regex: Created|created

```bash
ss extras init symlink-test --target /tmp/sym-target --mode symlink 2>&1
```

Expected:
- exit_code: 0
- regex: Created|created

### 10. sync --all --json includes multiple extras

```bash
echo "# Copy content" > ~/.config/skillshare/extras/copy-test/content.md
ss sync --all --json 2>/dev/null | python3 -c "
import sys, json
obj = json.loads(sys.stdin.read().strip())
extras = obj.get('extras', [])
names = sorted([e['name'] for e in extras])
print(f'extras_count={len(extras)}')
for n in names:
    print(f'extra={n}')
"
```

Expected:
- exit_code: 0
- extras_count=3
- extra=copy-test
- extra=rules
- extra=symlink-test

### 11. No extras field when none configured

```bash
cp ~/.config/skillshare/config.yaml ~/.config/skillshare/config.yaml.bak
sed -i '/^extras:/,$d' ~/.config/skillshare/config.yaml
ss sync --all --json 2>/dev/null | python3 -c "
import sys, json
obj = json.loads(sys.stdin.read().strip())
has_extras = 'extras' in obj and obj['extras'] is not None
print(f'has_extras={has_extras}')
print('valid_json=yes')
"
```

Expected:
- exit_code: 0
- valid_json=yes
- has_extras=False

```bash
cp ~/.config/skillshare/config.yaml.bak ~/.config/skillshare/config.yaml
```

Expected:
- exit_code: 0

### 12. Auto-migration: legacy flat path migrated on sync

Use `extras init` to add config entry properly, then simulate legacy path.

```bash
ss extras init prompts --target ~/.claude/prompts
# Delete the new-style directory that init created
rm -rf ~/.config/skillshare/extras/prompts
# Create files at legacy (flat) path
mkdir -p ~/.config/skillshare/prompts
echo "# System prompt" > ~/.config/skillshare/prompts/system.md
test -d ~/.config/skillshare/prompts && echo "legacy_exists=yes"
test -d ~/.config/skillshare/extras/prompts && echo "new_exists=yes" || echo "new_exists=no"
```

Expected:
- exit_code: 0
- legacy_exists=yes
- new_exists=no

```bash
ss sync extras 2>&1
```

Expected:
- exit_code: 0
- Prompts

```bash
test -d ~/.config/skillshare/extras/prompts && echo "migrated=yes" || echo "migrated=no"
test -d ~/.config/skillshare/prompts && echo "legacy_still=yes" || echo "legacy_still=no"
cat ~/.claude/prompts/system.md
```

Expected:
- exit_code: 0
- migrated=yes
- legacy_still=no
- System prompt

### 13. Copy mode: verify file is a real copy, not symlink

```bash
test -f /tmp/copy-target/content.md && echo "file_exists=yes" || echo "file_exists=no"
test -L /tmp/copy-target/content.md && echo "is_symlink=yes" || echo "is_symlink=no"
cat /tmp/copy-target/content.md
```

Expected:
- exit_code: 0
- file_exists=yes
- is_symlink=no
- Copy content

### 14. Copy mode: content update re-syncs correctly

```bash
echo "# Updated copy content" > ~/.config/skillshare/extras/copy-test/content.md
ss sync extras --force
cat /tmp/copy-target/content.md
```

Expected:
- exit_code: 0
- Updated copy content

### 15. Project mode: extras init -p creates .skillshare/extras/<name>/

```bash
mkdir -p /tmp/test-project
cd /tmp/test-project
ss init -p --targets claude
ss extras init proj-rules --target .claude/rules -p
```

Expected:
- exit_code: 0
- regex: Created|created

```bash
cd /tmp/test-project
test -d .skillshare/extras/proj-rules && echo "proj_extras_dir=yes" || echo "proj_extras_dir=no"
```

Expected:
- exit_code: 0
- proj_extras_dir=yes

### 16. Project mode: sync extras -p syncs project extras

```bash
cd /tmp/test-project
echo "# Project coding rules" > .skillshare/extras/proj-rules/coding.md
ss sync extras -p
```

Expected:
- exit_code: 0
- regex: Proj-rules|proj-rules|Proj_rules

```bash
cd /tmp/test-project
test -L .claude/rules/coding.md && echo "is_symlink=yes" || echo "is_symlink=no"
cat .claude/rules/coding.md
```

Expected:
- exit_code: 0
- is_symlink=yes
- Project coding rules

### 17. Project mode: extras list --json -p returns correct data

```bash
cd /tmp/test-project
ss extras list --json -p | python3 -c "
import sys, json
entries = json.load(sys.stdin)
print('valid_json=yes')
for e in entries:
    print(f'name={e[\"name\"]}')
    print(f'source_exists={e[\"source_exists\"]}')
    print(f'file_count={e[\"file_count\"]}')
    for t in e.get('targets', []):
        print(f'target_status={t[\"status\"]}')
"
```

Expected:
- exit_code: 0
- valid_json=yes
- name=proj-rules
- source_exists=True
- file_count=1
- target_status=synced

### 18. Project mode: sync --all --json -p produces single JSON with extras

```bash
cd /tmp/test-project
ss sync --all --json -p 2>/dev/null | python3 -c "
import sys, json
data = sys.stdin.read().strip()
obj = json.loads(data)
print('single_json=yes')
has_extras = 'extras' in obj
print(f'has_extras={has_extras}')
if has_extras:
    print(f'extras_len={len(obj[\"extras\"])}')
    for e in obj['extras']:
        print(f'extra_name={e[\"name\"]}')
"
```

Expected:
- exit_code: 0
- single_json=yes
- has_extras=True
- extras_len=1
- extra_name=proj-rules

## Pass Criteria

All 18 steps pass. Key behaviors validated:
- `sync --all --json` outputs a single valid JSON object with `extras` field
- Extras source uses `extras/<name>/` directory layout
- `extras list --json` returns correct sync status via shared `CheckSyncStatus`
- `extras init` validates mode via shared `ValidateExtraMode`
- Auto-migration from legacy flat path to `extras/<name>/`
- No `extras` field in JSON when no extras configured (omitempty)
- Copy mode creates real file copies (not symlinks) and re-syncs on content change
- Project mode: init, sync, list, and sync --all --json all work with `-p` flag
- Project extras stored in `.skillshare/extras/<name>/`
