# CLI E2E Runbook: target_naming

Validates `target_naming` for flat-only clients, including global default,
per-target override, standard-only validation and collision handling, managed
entry migration from `flat` to `standard`, the symlink-mode ignore path, and
`prefixed` naming for copy targets with migrations between all three namings.

## Scope

- Top-level `target_naming: standard` applies to merge/copy targets
- Per-target `skills.target_naming: flat` overrides the global default
- `standard` mode uses the `SKILL.md` `name` as the target entry name
- `standard` mode warns and skips invalid skills
- `standard` mode warns about skills that share a name and syncs them under their flat names
- `flat -> standard` migration renames provably managed merge/copy entries
- Existing local bare-name entries block migration and preserve legacy managed entries
- `flat` and `standard` naming are ignored in `symlink` mode; `prefixed` fails validation there
- `prefixed` names tracked-repo skills `<repo>-<name>` in folder and copied `name:`, leaving the source untouched
- `flat -> prefixed -> standard` migrations rename managed copies in place and rewrite `name:`
- `prefixed` on a merge target fails validation

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
echo "$OUTPUT" | grep -q "skill(s) skipped (selection or naming)" && echo "MERGE_INVALID_WARN=OK" || echo "MERGE_INVALID_WARN=FAIL"
echo "$OUTPUT" | grep -q "skill(s) skipped (selection or naming)" && echo "COPY_INVALID_WARN=OK" || echo "COPY_INVALID_WARN=FAIL"
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
test ! -e "$MERGE_DIR/dev" && test -L "$MERGE_DIR/frontend__dev" && test -L "$MERGE_DIR/backend__dev" && echo "MERGE_COLLISION_FLAT=OK" || echo "MERGE_COLLISION_FLAT=FAIL"
test ! -e "$MERGE_DIR/bad" && echo "MERGE_INVALID_SKIPPED=OK" || echo "MERGE_INVALID_SKIPPED=FAIL"

test -f "$COPY_STD_DIR/tooling/SKILL.md" && echo "COPY_STANDARD_TOOLING=OK" || echo "COPY_STANDARD_TOOLING=FAIL"
test -f "$COPY_STD_DIR/alpha/SKILL.md" && echo "COPY_STANDARD_ALPHA=OK" || echo "COPY_STANDARD_ALPHA=FAIL"
test ! -e "$COPY_STD_DIR/frontend__tooling" && echo "COPY_NO_FLAT_TOOLING=OK" || echo "COPY_NO_FLAT_TOOLING=FAIL"
test ! -e "$COPY_STD_DIR/dev" && test -f "$COPY_STD_DIR/frontend__dev/SKILL.md" && test -f "$COPY_STD_DIR/backend__dev/SKILL.md" && echo "COPY_COLLISION_FLAT=OK" || echo "COPY_COLLISION_FLAT=FAIL"
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
- MERGE_COLLISION_FLAT=OK
- MERGE_INVALID_SKIPPED=OK
- COPY_STANDARD_TOOLING=OK
- COPY_STANDARD_ALPHA=OK
- COPY_NO_FLAT_TOOLING=OK
- COPY_COLLISION_FLAT=OK
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

### Step 7: Sync two tracked repos with a same-named skill under flat copy naming

```bash
SRC="$HOME/.e2e-target-naming/prefixed-source"
rm -rf "$SRC" "$HOME/.e2e-target-naming/copy-prefixed" "$HOME/.e2e-target-naming/merge-prefixed"
for dir in _emil-design/skills/prototype _emil-design/skills/solo _mattpocock-skills/skills/engineering/prototype my-skill; do
  mkdir -p "$SRC/$dir"
  name=$(basename "$dir")
  printf '%s\n' '---' "name: $name" "description: $dir" '---' "# $dir" > "$SRC/$dir/SKILL.md"
done

printf '%s\n' \
  'source: ~/.e2e-target-naming/prefixed-source' \
  'targets:' \
  '  copy-prefixed:' \
  '    skills:' \
  '      path: ~/.e2e-target-naming/copy-prefixed' \
  '      mode: copy' \
  > "$HOME/.config/skillshare/config.yaml"

ss sync -g >/dev/null 2>&1
ls "$HOME/.e2e-target-naming/copy-prefixed"
```

Expected:
- exit_code: 0
- _emil-design__skills__prototype
- _mattpocock-skills__skills__engineering__prototype
- my-skill

### Step 8: Switch to prefixed and verify in-place renames and rewritten names

```bash
SRC="$HOME/.e2e-target-naming/prefixed-source"
DIR="$HOME/.e2e-target-naming/copy-prefixed"
printf '%s\n' \
  'source: ~/.e2e-target-naming/prefixed-source' \
  'targets:' \
  '  copy-prefixed:' \
  '    skills:' \
  '      path: ~/.e2e-target-naming/copy-prefixed' \
  '      mode: copy' \
  '      target_naming: prefixed' \
  > "$HOME/.config/skillshare/config.yaml"

ss sync -g >/dev/null 2>&1
grep -qx 'name: emil-design-prototype' "$DIR/emil-design-prototype/SKILL.md" && echo "PREFIXED_EMIL=OK" || echo "PREFIXED_EMIL=FAIL"
grep -qx 'name: mattpocock-skills-prototype' "$DIR/mattpocock-skills-prototype/SKILL.md" && echo "PREFIXED_MATT=OK" || echo "PREFIXED_MATT=FAIL"
grep -qx 'name: my-skill' "$DIR/my-skill/SKILL.md" && echo "PREFIXED_UNTRACKED_KEPT=OK" || echo "PREFIXED_UNTRACKED_KEPT=FAIL"
grep -qx 'name: prototype' "$SRC/_emil-design/skills/prototype/SKILL.md" && echo "SOURCE_UNTOUCHED=OK" || echo "SOURCE_UNTOUCHED=FAIL"
test ! -e "$DIR/_emil-design__skills__prototype" && echo "FLAT_ENTRY_RENAMED=OK" || echo "FLAT_ENTRY_RENAMED=FAIL"
jq -r '.naming["emil-design-solo"]' "$DIR/.skillshare-manifest.json"
```

Expected:
- exit_code: 0
- PREFIXED_EMIL=OK
- PREFIXED_MATT=OK
- PREFIXED_UNTRACKED_KEPT=OK
- SOURCE_UNTOUCHED=OK
- FLAT_ENTRY_RENAMED=OK
- prefixed

### Step 9: Switch to standard and verify the prefixed copies go back

```bash
DIR="$HOME/.e2e-target-naming/copy-prefixed"
sed -i 's/target_naming: prefixed/target_naming: standard/' "$HOME/.config/skillshare/config.yaml"

ss sync -g >/dev/null 2>&1
grep -qx 'name: solo' "$DIR/solo/SKILL.md" && echo "STANDARD_SOLO_RENAMED=OK" || echo "STANDARD_SOLO_RENAMED=FAIL"
test ! -e "$DIR/emil-design-solo" && echo "PREFIXED_SOLO_GONE=OK" || echo "PREFIXED_SOLO_GONE=FAIL"
test ! -e "$DIR/prototype" && test ! -e "$DIR/emil-design-prototype" && grep -qx 'name: prototype' "$DIR/_emil-design__skills__prototype/SKILL.md" && grep -qx 'name: prototype' "$DIR/_mattpocock-skills__skills__engineering__prototype/SKILL.md" && echo "COLLIDING_PROTOTYPES_FLAT=OK" || echo "COLLIDING_PROTOTYPES_FLAT=FAIL"
jq -r '.naming.solo' "$DIR/.skillshare-manifest.json"
```

Expected:
- exit_code: 0
- STANDARD_SOLO_RENAMED=OK
- PREFIXED_SOLO_GONE=OK
- COLLIDING_PROTOTYPES_FLAT=OK
- standard

### Step 10: Verify prefixed is rejected on a merge target

```bash
printf '%s\n' \
  'source: ~/.e2e-target-naming/prefixed-source' \
  'targets:' \
  '  merge-prefixed:' \
  '    skills:' \
  '      path: ~/.e2e-target-naming/merge-prefixed' \
  '      mode: merge' \
  '      target_naming: prefixed' \
  > "$HOME/.config/skillshare/config.yaml"

OUTPUT=$(ss sync -g 2>&1)
echo "SYNC_EXIT=$?"
echo "$OUTPUT" | grep -q 'target naming "prefixed" requires copy mode' && echo "MERGE_PREFIXED_REJECTED=OK" || echo "MERGE_PREFIXED_REJECTED=FAIL"
test -z "$(ls -A "$HOME/.e2e-target-naming/merge-prefixed" 2>/dev/null)" && echo "MERGE_PREFIXED_EMPTY=OK" || echo "MERGE_PREFIXED_EMPTY=FAIL"
```

Expected:
- exit_code: 0
- SYNC_EXIT=1
- MERGE_PREFIXED_REJECTED=OK
- MERGE_PREFIXED_EMPTY=OK

## Pass Criteria

- All 10 steps pass
- `standard` mode produces bare target entry names for merge/copy targets
- invalid skills are warned and skipped only in `standard` mode; skills that share a name sync under their flat names
- per-target `flat` override preserves the legacy flattened naming contract
- managed flat entries migrate safely to bare names in `standard` mode
- destination conflicts preserve the legacy managed entry instead of overwriting local content
- `symlink` mode ignores `target_naming`
- `prefixed` keeps same-named skills from two tracked repos, rewrites only the copy's `name:`, and is copy-only
- switching between `flat`, `standard` and `prefixed` renames managed copies in place without orphans
