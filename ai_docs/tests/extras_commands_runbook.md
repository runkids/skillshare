# CLI E2E Runbook: Extras Commands — init, list, remove, collect, doctor

Validates all `extras` subcommands end-to-end: init (with flags and validation),
list (pretty-print and JSON), remove (with/without --force), collect (from
target back to source), tilde path expansion, and doctor extras checks.

**Origin**: v0.17.0 — extras output redesign, tilde expansion bug fix,
doctor per-target warning.

## Scope

- `extras init` creates source dir and adds config entry
- `extras init` rejects invalid names, duplicate names, invalid modes
- `extras init` supports multiple `--target` flags
- `extras list` shows unified header "Extras" with status icons
- `extras list --json` returns valid JSON with sync status
- `extras remove` without `--force` shows warning, does not remove
- `extras remove --force` removes config entry, preserves source files
- `extras collect` copies local files from target into source
- `extras collect --dry-run` shows plan without changes
- Tilde `~` paths resolve correctly (no literal `~/` directory creation)
- `doctor` reports unreachable extras targets with expanded path

## Environment

Run inside devcontainer with `ssenv` isolation.
If `ss` alias is unavailable, replace `ss` with `skillshare`.

## Steps

### 1. Setup: clean environment with no extras

```bash
ss extras list -g --json
```

Expected:
- exit_code: 0
- []

### 2. extras init: basic creation with single target

```bash
ss extras init rules --target ~/.claude/rules -g
```

Expected:
- exit_code: 0
- regex: Created|created
- 1 target

```bash
test -d ~/.config/skillshare/extras/rules && echo "source_dir=yes" || echo "source_dir=no"
```

Expected:
- exit_code: 0
- source_dir=yes

### 3. extras init: multiple targets

```bash
ss extras init commands --target ~/.claude/commands --target ~/.cursor/commands --mode copy -g
```

Expected:
- exit_code: 0
- regex: Created|created
- 2 target

### 4. extras init: rejects invalid name

```bash
ss extras init "invalid name!" --target /tmp/test -g 2>&1 || true
```

Expected:
- regex: invalid

### 5. extras init: rejects duplicate name

```bash
ss extras init rules --target /tmp/test -g 2>&1 || true
```

Expected:
- regex: already exists|duplicate

### 6. extras init: rejects invalid mode

```bash
ss extras init test-mode --target /tmp/test --mode badmode -g 2>&1 || true
```

Expected:
- regex: invalid mode

### 7. extras list: pretty-print with unified header

```bash
ss extras list -g
```

Expected:
- exit_code: 0
- Extras
- rules
- commands

### 8. extras list: JSON output with sync status

```bash
ss extras list --json -g | jq -e 'length == 2'
```

Expected:
- exit_code: 0

```bash
ss extras list --json -g | jq -e '.[0].name == "rules"'
```

Expected:
- exit_code: 0

```bash
ss extras list --json -g | jq -r '.[0].source_exists'
```

Expected:
- exit_code: 0
- true

### 9. Add files and sync, then verify list shows correct status

```bash
echo "# TDD rule" > ~/.config/skillshare/extras/rules/tdd.md
echo "# Build cmd" > ~/.config/skillshare/extras/commands/build.md
ss sync extras -g
```

Expected:
- exit_code: 0
- Sync Extras

```bash
ss extras list --json -g | jq -r '.[0].file_count'
```

Expected:
- exit_code: 0
- 1

### 10. Tilde expansion: verify sync creates real path, not literal ~/

```bash
test -d ~/.claude/rules && echo "real_path=yes" || echo "real_path=no"
ls ~/.claude/rules/
```

Expected:
- exit_code: 0
- real_path=yes
- tdd.md

### 11. extras collect: gather local files from target to source

```bash
echo "# Local coding style" > ~/.claude/rules/local-style.md
ss extras collect rules -g
```

Expected:
- exit_code: 0
- regex: collected|1 file

```bash
test -f ~/.config/skillshare/extras/rules/local-style.md && echo "collected=yes" || echo "collected=no"
```

Expected:
- exit_code: 0
- collected=yes

### 12. extras collect: dry-run shows plan without changes

```bash
echo "# Another local file" > ~/.claude/rules/another.md
ss extras collect rules --dry-run -g
```

Expected:
- exit_code: 0
- regex: would collect|dry run|Dry run

```bash
test -f ~/.config/skillshare/extras/rules/another.md && echo "exists=yes" || echo "exists=no"
```

Expected:
- exit_code: 0
- exists=no

### 13. extras remove: without --force shows warning, no removal

```bash
ss extras remove commands -g 2>&1
```

Expected:
- exit_code: 0
- regex: will remove|warning|--force

```bash
ss extras list --json -g | jq -e 'length == 2'
```

Expected:
- exit_code: 0

### 14. extras remove: with --force removes from config

```bash
ss extras remove commands --force -g
```

Expected:
- exit_code: 0
- regex: Removed|removed

```bash
ss extras list --json -g | jq -e 'length == 1'
```

Expected:
- exit_code: 0

```bash
test -d ~/.config/skillshare/extras/commands && echo "source_preserved=yes" || echo "source_preserved=no"
```

Expected:
- exit_code: 0
- source_preserved=yes

### 15. doctor: extras checks show reachable targets

```bash
ss doctor -g 2>&1 | grep -A3 "Extras"
```

Expected:
- exit_code: 0
- Extras
- rules
- regex: reachable|source exists

### 16. doctor: unreachable target shows warning with expanded path

```bash
ss extras init ghost --target ~/.nonexistent-tool/ghost-rules -g
ss doctor -g 2>&1 | grep -A8 "Extras"
```

Expected:
- exit_code: 0
- regex: not reachable|unreachable
- regex: nonexistent-tool

```bash
ss extras remove ghost --force -g
```

Expected:
- exit_code: 0

### 17. Project mode: extras init -p

```bash
rm -rf /tmp/extras-proj
mkdir -p /tmp/extras-proj
cd /tmp/extras-proj
ss init -p --targets claude
ss extras init proj-rules --target .claude/rules -p
```

Expected:
- exit_code: 0
- regex: Created|created

```bash
cd /tmp/extras-proj
test -d .skillshare/extras/proj-rules && echo "proj_dir=yes" || echo "proj_dir=no"
```

Expected:
- exit_code: 0
- proj_dir=yes

### 18. Project mode: sync, list, collect, remove

```bash
cd /tmp/extras-proj
echo "# Project rule" > .skillshare/extras/proj-rules/rule.md
ss sync extras -p
```

Expected:
- exit_code: 0
- Sync Extras

```bash
cd /tmp/extras-proj
ss extras list -p
```

Expected:
- exit_code: 0
- Extras
- proj-rules

```bash
cd /tmp/extras-proj
echo "# Local addition" > .claude/rules/local.md
ss extras collect proj-rules -p 2>&1
echo "collect_exit=$?"
```

Expected:
- exit_code: 0
- collected

```bash
cd /tmp/extras-proj
ss extras remove proj-rules --force -p
```

Expected:
- exit_code: 0
- regex: Removed|removed

```bash
cd /tmp/extras-proj
ss extras list --json -p
```

Expected:
- exit_code: 0
- []

## Pass Criteria

All 18 steps pass. Key behaviors validated:
- `extras init` creates source dir, validates name/mode, rejects duplicates
- `extras list` uses unified "Extras" header with status icons
- `extras list --json` returns correct sync status and file counts
- `extras remove` requires `--force` to actually remove
- `extras collect` gathers local files; `--dry-run` is non-destructive
- Tilde `~` paths expand correctly (no literal `~/` directory)
- `doctor` reports per-target reachability with expanded paths
- All commands work in both global and project mode
