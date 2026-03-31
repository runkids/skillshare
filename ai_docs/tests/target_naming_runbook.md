# CLI E2E Runbook: target_naming

Validates `target_naming` for flat-only clients, including global default,
per-target override, standard-only validation and collision handling, managed
entry migration from `flat` to `standard`, and the symlink-mode ignore path.

## Scope

- Top-level `target_naming: standard` applies to merge/copy targets
- Per-target `skills.target_naming: flat` overrides the global default
- `standard` mode uses the `SKILL.md` `name` as the target entry name
- `standard` mode warns and skips invalid skills
- `standard` mode warns and skips target-visible collisions
- `flat -> standard` migration renames provably managed merge/copy entries
- Existing local bare-name entries block migration and preserve legacy managed entries
- `target_naming` is ignored in `symlink` mode

## Environment

Run inside devcontainer with mdproof isolation. Setup hook initializes global
skillshare config before the runbook starts.

## Steps

### Step 1: Configure standard-by-default targets with one flat override

```bash
rm -rf "$HOME/.e2e-target-naming" "$HOME/.e2e-source"
mkdir -p "$HOME/.e2e-target-naming" "$HOME/.e2e-source"

mkdir -p "$HOME/.e2e-source/alpha"
printf '%s\n' \
  '---' \
  'name: alpha' \
  'description: Alpha skill' \
  '---' \
  '# Alpha' \
  > "$HOME/.e2e-source/alpha/SKILL.md"

mkdir -p "$HOME/.e2e-source/frontend/tooling"
printf '%s\n' \
  '---' \
  'name: tooling' \
  'description: Tooling skill' \
  '---' \
  '# Tooling' \
  > "$HOME/.e2e-source/frontend/tooling/SKILL.md"

mkdir -p "$HOME/.e2e-source/frontend/bad"
printf '%s\n' \
  '---' \
  'name: wrong-name' \
  'description: Invalid standard naming' \
  '---' \
  '# Bad' \
  > "$HOME/.e2e-source/frontend/bad/SKILL.md"

mkdir -p "$HOME/.e2e-source/frontend/dev"
printf '%s\n' \
  '---' \
  'name: dev' \
  'description: Frontend dev' \
  '---' \
  '# Frontend Dev' \
  > "$HOME/.e2e-source/frontend/dev/SKILL.md"

mkdir -p "$HOME/.e2e-source/backend/dev"
printf '%s\n' \
  '---' \
  'name: dev' \
  'description: Backend dev' \
  '---' \
  '# Backend Dev' \
  > "$HOME/.e2e-source/backend/dev/SKILL.md"

printf '%s\n' \
  'source: ~/.e2e-source' \
  'target_naming: standard' \
  'targets:' \
  '  merge-standard:' \
  '    skills:' \
  '      path: ~/.e2e-target-naming/merge-standard' \
  '      mode: merge' \
  '  copy-standard:' \
  '    skills:' \
  '      path: ~/.e2e-target-naming/copy-standard' \
  '      mode: copy' \
  '  copy-flat:' \
  '    skills:' \
  '      path: ~/.e2e-target-naming/copy-flat' \
  '      mode: copy' \
  '      target_naming: flat' \
  > "$HOME/.config/skillshare/config.yaml"

cat "$HOME/.config/skillshare/config.yaml"
```

Expected:
- exit_code: 0
- target_naming: standard
- merge-standard
- copy-standard
- copy-flat
- target_naming: flat

### Step 2: Sync and verify standard warnings are emitted

```bash
OUTPUT=$(ss sync -g 2>&1)
printf '%s\n' "$OUTPUT"
echo "$OUTPUT" | grep -q "Target 'merge-standard': skipped frontend/bad because" && echo "MERGE_INVALID_WARN=OK" || echo "MERGE_INVALID_WARN=FAIL"
echo "$OUTPUT" | grep -q "Target 'copy-standard': skipped frontend/bad because" && echo "COPY_INVALID_WARN=OK" || echo "COPY_INVALID_WARN=FAIL"
echo "$OUTPUT" | grep -q "duplicate skill names" && echo "MERGE_COLLISION_WARN=OK" || echo "MERGE_COLLISION_WARN=FAIL"
echo "$OUTPUT" | grep -q "dev" && echo "COPY_COLLISION_WARN=OK" || echo "COPY_COLLISION_WARN=FAIL"
```

Expected:
- exit_code: 0
- MERGE_INVALID_WARN=OK
- COPY_INVALID_WARN=OK
- MERGE_COLLISION_WARN=OK
- COPY_COLLISION_WARN=OK

### Step 3: Verify standard targets use bare names and flat override preserves flattened names

```bash
MERGE_DIR="$HOME/.e2e-target-naming/merge-standard"
COPY_STD_DIR="$HOME/.e2e-target-naming/copy-standard"
COPY_FLAT_DIR="$HOME/.e2e-target-naming/copy-flat"

test -L "$MERGE_DIR/tooling" && echo "MERGE_STANDARD_TOOLING=OK" || echo "MERGE_STANDARD_TOOLING=FAIL"
test -L "$MERGE_DIR/alpha" && echo "MERGE_STANDARD_ALPHA=OK" || echo "MERGE_STANDARD_ALPHA=FAIL"
test ! -e "$MERGE_DIR/frontend__tooling" && echo "MERGE_NO_FLAT_TOOLING=OK" || echo "MERGE_NO_FLAT_TOOLING=FAIL"
test ! -e "$MERGE_DIR/dev" && echo "MERGE_COLLISION_SKIPPED=OK" || echo "MERGE_COLLISION_SKIPPED=FAIL"
test ! -e "$MERGE_DIR/bad" && echo "MERGE_INVALID_SKIPPED=OK" || echo "MERGE_INVALID_SKIPPED=FAIL"

test -f "$COPY_STD_DIR/tooling/SKILL.md" && echo "COPY_STANDARD_TOOLING=OK" || echo "COPY_STANDARD_TOOLING=FAIL"
test -f "$COPY_STD_DIR/alpha/SKILL.md" && echo "COPY_STANDARD_ALPHA=OK" || echo "COPY_STANDARD_ALPHA=FAIL"
test ! -e "$COPY_STD_DIR/frontend__tooling" && echo "COPY_NO_FLAT_TOOLING=OK" || echo "COPY_NO_FLAT_TOOLING=FAIL"
test ! -e "$COPY_STD_DIR/dev" && echo "COPY_COLLISION_SKIPPED=OK" || echo "COPY_COLLISION_SKIPPED=FAIL"
test ! -e "$COPY_STD_DIR/bad" && echo "COPY_INVALID_SKIPPED=OK" || echo "COPY_INVALID_SKIPPED=FAIL"
cat "$COPY_STD_DIR/.skillshare-manifest.json" | jq -e '.managed | has("tooling")' >/dev/null && echo "COPY_STANDARD_MANIFEST_BARE=OK" || echo "COPY_STANDARD_MANIFEST_BARE=FAIL"
cat "$COPY_STD_DIR/.skillshare-manifest.json" | jq -e '.managed | has("frontend__tooling") | not' >/dev/null && echo "COPY_STANDARD_MANIFEST_NO_FLAT=OK" || echo "COPY_STANDARD_MANIFEST_NO_FLAT=FAIL"

test -f "$COPY_FLAT_DIR/frontend__tooling/SKILL.md" && echo "COPY_FLAT_TOOLING=OK" || echo "COPY_FLAT_TOOLING=FAIL"
test -f "$COPY_FLAT_DIR/frontend__dev/SKILL.md" && echo "COPY_FLAT_FRONTEND_DEV=OK" || echo "COPY_FLAT_FRONTEND_DEV=FAIL"
test -f "$COPY_FLAT_DIR/backend__dev/SKILL.md" && echo "COPY_FLAT_BACKEND_DEV=OK" || echo "COPY_FLAT_BACKEND_DEV=FAIL"
test -f "$COPY_FLAT_DIR/frontend__bad/SKILL.md" && echo "COPY_FLAT_INVALID_STILL_SYNCS=OK" || echo "COPY_FLAT_INVALID_STILL_SYNCS=FAIL"
```

Expected:
- exit_code: 0
- MERGE_STANDARD_TOOLING=OK
- MERGE_STANDARD_ALPHA=OK
- MERGE_NO_FLAT_TOOLING=OK
- MERGE_COLLISION_SKIPPED=OK
- MERGE_INVALID_SKIPPED=OK
- COPY_STANDARD_TOOLING=OK
- COPY_STANDARD_ALPHA=OK
- COPY_NO_FLAT_TOOLING=OK
- COPY_COLLISION_SKIPPED=OK
- COPY_INVALID_SKIPPED=OK
- COPY_STANDARD_MANIFEST_BARE=OK
- COPY_STANDARD_MANIFEST_NO_FLAT=OK
- COPY_FLAT_TOOLING=OK
- COPY_FLAT_FRONTEND_DEV=OK
- COPY_FLAT_BACKEND_DEV=OK
- COPY_FLAT_INVALID_STILL_SYNCS=OK

### Step 4: Create legacy flat entries for migration targets

```bash
rm -rf "$HOME/.e2e-target-naming/migration-source" \
  "$HOME/.e2e-target-naming/merge-migrate" \
  "$HOME/.e2e-target-naming/copy-migrate" \
  "$HOME/.e2e-target-naming/merge-preserve"

mkdir -p "$HOME/.e2e-target-naming/migration-source/frontend/migrate-dev"
printf '%s\n' \
  '---' \
  'name: migrate-dev' \
  'description: Skill used for migration checks' \
  '---' \
  '# Migrate Dev' \
  > "$HOME/.e2e-target-naming/migration-source/frontend/migrate-dev/SKILL.md"

printf '%s\n' \
  'source: ~/.e2e-target-naming/migration-source' \
  'targets:' \
  '  merge-migrate:' \
  '    skills:' \
  '      path: ~/.e2e-target-naming/merge-migrate' \
  '      mode: merge' \
  '  copy-migrate:' \
  '    skills:' \
  '      path: ~/.e2e-target-naming/copy-migrate' \
  '      mode: copy' \
  '  merge-preserve:' \
  '    skills:' \
  '      path: ~/.e2e-target-naming/merge-preserve' \
  '      mode: merge' \
  > "$HOME/.config/skillshare/config.yaml"

OUTPUT=$(ss sync -g 2>&1)
printf '%s\n' "$OUTPUT"

test -L "$HOME/.e2e-target-naming/merge-migrate/frontend__migrate-dev" && echo "LEGACY_MERGE_CREATED=OK" || echo "LEGACY_MERGE_CREATED=FAIL"
test -f "$HOME/.e2e-target-naming/copy-migrate/frontend__migrate-dev/SKILL.md" && echo "LEGACY_COPY_CREATED=OK" || echo "LEGACY_COPY_CREATED=FAIL"
test -L "$HOME/.e2e-target-naming/merge-preserve/frontend__migrate-dev" && echo "LEGACY_PRESERVE_CREATED=OK" || echo "LEGACY_PRESERVE_CREATED=FAIL"
```

Expected:
- exit_code: 0
- LEGACY_MERGE_CREATED=OK
- LEGACY_COPY_CREATED=OK
- LEGACY_PRESERVE_CREATED=OK

### Step 5: Switch to standard naming and verify managed migration plus preservation behavior

```bash
mkdir -p "$HOME/.e2e-target-naming/merge-preserve/migrate-dev"
printf '%s\n' '# Local skill blocks migration' > "$HOME/.e2e-target-naming/merge-preserve/migrate-dev/SKILL.md"

printf '%s\n' \
  'source: ~/.e2e-target-naming/migration-source' \
  'target_naming: standard' \
  'targets:' \
  '  merge-migrate:' \
  '    skills:' \
  '      path: ~/.e2e-target-naming/merge-migrate' \
  '      mode: merge' \
  '  copy-migrate:' \
  '    skills:' \
  '      path: ~/.e2e-target-naming/copy-migrate' \
  '      mode: copy' \
  '  merge-preserve:' \
  '    skills:' \
  '      path: ~/.e2e-target-naming/merge-preserve' \
  '      mode: merge' \
  > "$HOME/.config/skillshare/config.yaml"

OUTPUT=$(ss sync -g 2>&1)
printf '%s\n' "$OUTPUT"
echo "$OUTPUT" | grep -q "kept legacy managed entry frontend__migrate-dev" && echo "PRESERVE_WARNING=OK" || echo "PRESERVE_WARNING=FAIL"

test -L "$HOME/.e2e-target-naming/merge-migrate/migrate-dev" && echo "MERGE_MIGRATED=OK" || echo "MERGE_MIGRATED=FAIL"
test ! -e "$HOME/.e2e-target-naming/merge-migrate/frontend__migrate-dev" && echo "MERGE_LEGACY_REMOVED=OK" || echo "MERGE_LEGACY_REMOVED=FAIL"

test -f "$HOME/.e2e-target-naming/copy-migrate/migrate-dev/SKILL.md" && echo "COPY_MIGRATED=OK" || echo "COPY_MIGRATED=FAIL"
test ! -e "$HOME/.e2e-target-naming/copy-migrate/frontend__migrate-dev" && echo "COPY_LEGACY_REMOVED=OK" || echo "COPY_LEGACY_REMOVED=FAIL"
cat "$HOME/.e2e-target-naming/copy-migrate/.skillshare-manifest.json" | jq -e '.managed | has("migrate-dev")' >/dev/null && echo "COPY_MANIFEST_RENAMED=OK" || echo "COPY_MANIFEST_RENAMED=FAIL"
cat "$HOME/.e2e-target-naming/copy-migrate/.skillshare-manifest.json" | jq -e '.managed | has("frontend__migrate-dev") | not' >/dev/null && echo "COPY_MANIFEST_NO_LEGACY=OK" || echo "COPY_MANIFEST_NO_LEGACY=FAIL"

test -L "$HOME/.e2e-target-naming/merge-preserve/frontend__migrate-dev" && echo "PRESERVE_LEGACY_KEPT=OK" || echo "PRESERVE_LEGACY_KEPT=FAIL"
test -f "$HOME/.e2e-target-naming/merge-preserve/migrate-dev/SKILL.md" && echo "PRESERVE_LOCAL_BARE_KEPT=OK" || echo "PRESERVE_LOCAL_BARE_KEPT=FAIL"
```

Expected:
- exit_code: 0
- PRESERVE_WARNING=OK
- MERGE_MIGRATED=OK
- MERGE_LEGACY_REMOVED=OK
- COPY_MIGRATED=OK
- COPY_LEGACY_REMOVED=OK
- COPY_MANIFEST_RENAMED=OK
- COPY_MANIFEST_NO_LEGACY=OK
- PRESERVE_LEGACY_KEPT=OK
- PRESERVE_LOCAL_BARE_KEPT=OK

### Step 6: Verify symlink mode ignores target_naming

```bash
rm -rf "$HOME/.e2e-target-naming/symlink-source" "$HOME/.e2e-target-naming/symlink-target"
mkdir -p "$HOME/.e2e-target-naming/symlink-source/frontend/dev"
printf '%s\n' \
  '---' \
  'name: dev' \
  'description: Symlink mode ignore check' \
  '---' \
  '# Dev' \
  > "$HOME/.e2e-target-naming/symlink-source/frontend/dev/SKILL.md"

printf '%s\n' \
  'source: ~/.e2e-target-naming/symlink-source' \
  'target_naming: standard' \
  'targets:' \
  '  symlink-target:' \
  '    skills:' \
  '      path: ~/.e2e-target-naming/symlink-target' \
  '      mode: symlink' \
  > "$HOME/.config/skillshare/config.yaml"

OUTPUT=$(ss sync -g 2>&1)
printf '%s\n' "$OUTPUT"
TARGET_LINK=$(readlink "$HOME/.e2e-target-naming/symlink-target")
echo "TARGET_LINK=$TARGET_LINK"
test -L "$HOME/.e2e-target-naming/symlink-target" && echo "SYMLINK_MODE_TARGET_IS_LINK=OK" || echo "SYMLINK_MODE_TARGET_IS_LINK=FAIL"
[ "$TARGET_LINK" = "$HOME/.e2e-target-naming/symlink-source" ] && echo "SYMLINK_MODE_POINTS_TO_SOURCE=OK" || echo "SYMLINK_MODE_POINTS_TO_SOURCE=FAIL"
echo "$OUTPUT" | grep -q "kept legacy managed entry" && echo "SYMLINK_MODE_NO_MIGRATION_WARN=FAIL" || echo "SYMLINK_MODE_NO_MIGRATION_WARN=OK"
```

Expected:
- exit_code: 0
- SYMLINK_MODE_TARGET_IS_LINK=OK
- SYMLINK_MODE_POINTS_TO_SOURCE=OK
- SYMLINK_MODE_NO_MIGRATION_WARN=OK

## Pass Criteria

- All 6 steps pass
- `standard` mode produces bare target entry names for merge/copy targets
- invalid skills and target-visible collisions are warned and skipped only in `standard` mode
- per-target `flat` override preserves the legacy flattened naming contract
- managed flat entries migrate safely to bare names in `standard` mode
- destination conflicts preserve the legacy managed entry instead of overwriting local content
- `symlink` mode ignores `target_naming`
