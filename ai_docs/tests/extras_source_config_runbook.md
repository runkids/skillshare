# CLI E2E Runbook: Extras Source Config — extras_source and per-extra source

Validates configurable extras source paths: top-level `extras_source` in
config.yaml, per-extra `source` field, three-level priority resolution,
`--source` flag in `extras init`, `source_type` in JSON output, and
sync/collect/doctor correctness with custom source paths.

**Origin**: extras source config feature — replaces hardcoded extras path
derivation with configurable three-level priority.

## Scope

- `extras_source` in config.yaml sets global default extras source directory
- Per-extra `source` field overrides `extras_source` and default
- `extras init --source <path>` writes per-extra source to config
- `extras list --json` includes `source_type` (per-extra / extras_source / default)
- `sync extras` uses resolved source path (not hardcoded default)
- `extras collect` collects to resolved source path
- `doctor` reports correct custom source path when missing
- Priority chain: per-extra source > extras_source > default
- Backward compatibility: no custom config = identical behavior

## Environment

Run inside devcontainer with `ssenv` isolation.
If `ss` alias is unavailable, replace `ss` with `skillshare`.

## Steps

### 1. Setup: clean environment

```bash
ss extras remove rules --force -g 2>/dev/null || true
ss extras remove commands --force -g 2>/dev/null || true
rm -rf ~/.claude/rules 2>/dev/null || true
rm -rf ~/custom-extras 2>/dev/null || true
rm -rf ~/per-extra-rules 2>/dev/null || true
ss extras list -g --json
```

Expected:
- exit_code: 0
- `[]`

### 2. Init with --source flag writes per-extra source to config

```bash
mkdir -p ~/per-extra-rules
ss extras init rules --target ~/.claude/rules --source ~/per-extra-rules -g
cat ~/.config/skillshare/config.yaml
```

Expected:
- exit_code: 0
- rules
- source:

### 3. Verify per-extra source in JSON output

```bash
ss extras list -g --json
```

Expected:
- exit_code: 0
- jq: .[0].source_type == "per-extra"
- jq: .[0].name == "rules"
- jq: .[0].source_exists == true

### 4. Sync uses per-extra source path

```bash
# Create a file in the per-extra source
echo "# TDD Guide" > ~/per-extra-rules/tdd.md
ss sync extras -g
# Verify symlink points to per-extra source
readlink ~/.claude/rules/tdd.md
```

Expected:
- exit_code: 0
- per-extra-rules/tdd.md

### 5. Collect goes to per-extra source

```bash
# Create a local file in target (not a symlink)
echo "# Local Rule" > ~/.claude/rules/local-rule.md
ss extras collect rules -g
# Verify file ended up in per-extra source, not default
ls ~/per-extra-rules/local-rule.md
```

Expected:
- exit_code: 0
- local-rule.md

### 6. Remove with per-extra source and re-init without source

```bash
ss extras remove rules --force -g >/dev/null 2>&1
rm -rf ~/.claude/rules 2>/dev/null || true
ss extras init rules --target ~/.claude/rules -g >/dev/null 2>&1
ss extras list -g --json
```

Expected:
- exit_code: 0
- jq: .[0].source_type == "extras_source"
- Not per-extra

### 7. Setup extras_source in config

```bash
# Remove current extras and set up extras_source
ss extras remove rules --force -g
rm -rf ~/.claude/rules 2>/dev/null || true
rm -rf ~/custom-extras 2>/dev/null || true
# Add extras_source to config
sed -i '/^extras_source:/d' ~/.config/skillshare/config.yaml
sed -i '/^extras:/,$d' ~/.config/skillshare/config.yaml
echo 'extras_source: ~/custom-extras' >> ~/.config/skillshare/config.yaml
cat ~/.config/skillshare/config.yaml
```

Expected:
- exit_code: 0
- extras_source:

### 8. Init with extras_source creates dir under custom path

```bash
ss extras init rules --target ~/.claude/rules -g
# Verify source dir created under extras_source, not default
test -d ~/custom-extras/rules && echo "CUSTOM_DIR_EXISTS"
# Verify default location was NOT created
test -d ~/.config/skillshare/extras/rules && echo "DEFAULT_EXISTS" || echo "DEFAULT_NOT_CREATED"
```

Expected:
- exit_code: 0
- CUSTOM_DIR_EXISTS
- DEFAULT_NOT_CREATED

### 9. Sync auto-creates missing source directory

```bash
# Remove the source dir that init created, then sync should recreate it
rm -rf ~/custom-extras/rules
ss sync extras -g 2>&1
test -d ~/custom-extras/rules && echo "AUTO_CREATED"
```

Expected:
- exit_code: 0
- AUTO_CREATED

### 10. extras_source shows correct source_type in JSON

```bash
ss extras list -g --json
```

Expected:
- exit_code: 0
- jq: .[0].source_type == "extras_source"
- jq: .[0].name == "rules"

### 11. Sync with extras_source

```bash
echo "# Custom Rule" > ~/custom-extras/rules/custom.md
ss sync extras -g
readlink ~/.claude/rules/custom.md
```

Expected:
- exit_code: 0
- custom-extras/rules/custom.md

### 12. Priority chain: per-extra > extras_source > default

```bash
# Add a second extra with per-extra source (overrides extras_source)
mkdir -p ~/override-commands
echo "# Override" > ~/override-commands/cmd.md
ss extras init commands --target ~/.claude/commands --source ~/override-commands -g >/dev/null 2>&1
ss extras list -g --json
```

Expected:
- exit_code: 0
- jq: [.[] | select(.name == "rules")] | .[0].source_type == "extras_source"
- jq: [.[] | select(.name == "commands")] | .[0].source_type == "per-extra"

### 13. Sync both extras with different source types

```bash
ss sync extras -g
# rules should come from extras_source (~/custom-extras/rules/)
readlink ~/.claude/rules/custom.md
# commands should come from per-extra source (~/override-commands/)
readlink ~/.claude/commands/cmd.md
```

Expected:
- exit_code: 0
- custom-extras/rules/custom.md
- override-commands/cmd.md

### 14. Doctor reports correct custom path when source missing

```bash
# Remove the per-extra source dir to trigger doctor warning
rm -rf ~/override-commands
ss doctor -g 2>&1
```

Expected:
- exit_code: 0
- override-commands

### 15. Merge mode: file synced as symlink with correct content

```bash
# rules extra is already in merge mode from earlier steps
# Verify the synced file is a symlink AND readable
ss extras remove commands --force -g >/dev/null 2>&1
rm -rf ~/override-commands ~/.claude/commands 2>/dev/null || true
echo "# Merge Content" > ~/custom-extras/rules/merge-test.md
ss sync extras -g >/dev/null 2>&1
test -L ~/.claude/rules/merge-test.md && echo "IS_SYMLINK"
cat ~/.claude/rules/merge-test.md
```

Expected:
- exit_code: 0
- IS_SYMLINK
- Merge Content

### 16. Copy mode: file synced as regular file with correct content

```bash
# Create a new extra with copy mode under extras_source
mkdir -p ~/custom-extras/copy-test
echo "# Copy Content" > ~/custom-extras/copy-test/file.md
ss extras init copy-test --target ~/.claude/copy-test --mode copy -g >/dev/null 2>&1
mkdir -p ~/.claude/copy-test
ss sync extras -g >/dev/null 2>&1
# Verify it's a regular file (NOT a symlink) AND has correct content
test -f ~/.claude/copy-test/file.md && ! test -L ~/.claude/copy-test/file.md && echo "IS_REGULAR_FILE"
cat ~/.claude/copy-test/file.md
```

Expected:
- exit_code: 0
- IS_REGULAR_FILE
- Copy Content

### 17. Symlink mode: directory symlink with accessible files

```bash
# Create a new extra with symlink (whole-dir) mode + per-extra source
mkdir -p ~/symlink-source
echo "# Symlink Content" > ~/symlink-source/sym.md
ss extras init sym-test --target ~/.claude/sym-test --mode symlink --source ~/symlink-source -g >/dev/null 2>&1
ss sync extras -g >/dev/null 2>&1
# Verify target is a directory symlink AND files are accessible through it
test -L ~/.claude/sym-test && echo "IS_DIR_SYMLINK"
cat ~/.claude/sym-test/sym.md
readlink ~/.claude/sym-test
```

Expected:
- exit_code: 0
- IS_DIR_SYMLINK
- Symlink Content
- symlink-source

### 18. Cleanup mode test extras

```bash
ss extras remove copy-test --force -g >/dev/null 2>&1
ss extras remove sym-test --force -g >/dev/null 2>&1
rm -rf ~/.claude/copy-test ~/.claude/sym-test ~/.claude/rules/merge-test.md ~/symlink-source ~/custom-extras/copy-test 2>/dev/null || true
echo "MODE_CLEANUP_DONE"
```

Expected:
- exit_code: 0
- MODE_CLEANUP_DONE

### 19. Backfill: extras init auto-populates extras_source

```bash
# Reset: remove all extras and extras_source from config
ss extras remove rules --force -g >/dev/null 2>&1
ss extras remove commands --force -g >/dev/null 2>&1
rm -rf ~/.claude/rules ~/.claude/commands ~/custom-extras ~/override-commands 2>/dev/null || true
sed -i '/^extras_source:/d' ~/.config/skillshare/config.yaml
sed -i '/^extras:/,$d' ~/.config/skillshare/config.yaml
# Now init a new extra — extras_source should be auto-filled
ss extras init rules --target ~/.claude/rules -g >/dev/null 2>&1
cat ~/.config/skillshare/config.yaml
```

Expected:
- exit_code: 0
- extras_source:

### 20. Status shows correct custom source path

```bash
# Set up extras_source and create a file
sed -i '/^extras_source:/d' ~/.config/skillshare/config.yaml
sed -i '/^extras:/,$d' ~/.config/skillshare/config.yaml
echo 'extras_source: ~/status-test-extras' >> ~/.config/skillshare/config.yaml
ss extras remove rules --force -g 2>/dev/null || true
rm -rf ~/status-test-extras 2>/dev/null || true
ss extras init rules --target ~/.claude/rules -g >/dev/null 2>&1
echo "# Rule" > ~/status-test-extras/rules/rule.md
ss sync extras -g >/dev/null 2>&1
ss status -g 2>&1
```

Expected:
- exit_code: 0
- rules
- regex: (synced|1 file)

### 21. Diff with custom source detects changes

```bash
# Add a new file to source (not yet synced)
echo "# New Rule" > ~/status-test-extras/rules/new-rule.md
ss diff -g 2>&1
```

Expected:
- exit_code: 0
- new-rule.md

### 22. Tilde expansion in extras_source

```bash
# Verify the resolved source_dir in JSON doesn't contain literal ~
ss extras list -g --json
```

Expected:
- exit_code: 0
- jq: .[0].source_dir | startswith("/")
- Not ~/status-test-extras

### 23. Backward compatibility: no extras_source uses default path

```bash
# Remove extras_source, re-init — backfill sets extras_source to default
ss extras remove rules --force -g >/dev/null 2>&1
rm -rf ~/.claude/rules ~/status-test-extras 2>/dev/null || true
sed -i '/^extras_source:/d' ~/.config/skillshare/config.yaml
sed -i '/^extras:/,$d' ~/.config/skillshare/config.yaml
ss extras init rules --target ~/.claude/rules -g >/dev/null 2>&1
ss extras list -g --json
```

Expected:
- exit_code: 0
- jq: .[0].source_exists == true
- regex: skillshare/extras/rules

### 24. Cleanup

```bash
ss extras remove rules --force -g 2>/dev/null || true
ss extras remove commands --force -g 2>/dev/null || true
ss extras remove copy-test --force -g 2>/dev/null || true
ss extras remove sym-test --force -g 2>/dev/null || true
rm -rf ~/custom-extras ~/per-extra-rules ~/override-commands ~/status-test-extras ~/symlink-source
rm -rf ~/.claude/rules ~/.claude/commands ~/.claude/copy-test ~/.claude/sym-test 2>/dev/null || true
sed -i '/^extras_source:/d' ~/.config/skillshare/config.yaml
sed -i '/^extras:/,$d' ~/.config/skillshare/config.yaml
echo "CLEANUP_DONE"
```

Expected:
- exit_code: 0
- CLEANUP_DONE

## Pass Criteria

All 24 steps pass. Key validations:
- `--source` flag persists per-extra source in config
- `extras_source` config creates dirs under custom path
- `source_type` correctly reflects resolution level in JSON
- Sync and collect use the resolved path (not default)
- Priority chain works: per-extra overrides extras_source
- Doctor reports correct custom paths
- All three sync modes work with custom source (merge=symlink, copy=regular, symlink=dir-link)
- Synced files have correct content accessible at target
- Sync auto-creates missing source directories
- `extras init` auto-populates `extras_source` when missing
- Status and diff use correct custom source paths
- Tilde paths expand correctly (no literal `~/`)
- No `extras_source` = identical default behavior
