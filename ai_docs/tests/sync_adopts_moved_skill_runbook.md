# CLI E2E Runbook: sync Adopts a Hand-Moved Skill

Verifies that `skillshare sync` moves the install record of a skill that was
moved with a plain `mv`, and that `sync` never prunes a record whose skill is
gone. `sync --dry-run` changes no record.

**Origin**: follow-up to [#510](https://github.com/runkids/skillshare/issues/510) —
before this, only `install` or **Install missing** adopted a moved copy, so
`update` and `uninstall` could not find a skill after a hand `mv` + `sync`.

## Scope

- Global mode, one local-path skill
- `sync --dry-run` leaves the record at the old path
- `sync` re-keys the record to the new path and renames the target link
- `sync` keeps the record of a skill whose directory was removed by hand
- `uninstall` finds the skill by its new path

## Environment

Run inside the devcontainer with `ssenv` isolation (`--init`).
Requires `jq`. mdproof replaces `$HOME`, so the source is read through
`$XDG_CONFIG_HOME` and the target path through `ss status --json`.

## Steps

### 1. Setup: install two local skills and sync

```bash
rm -rf /tmp/ss-adopt-src
mkdir -p /tmp/ss-adopt-src/adopt-skill /tmp/ss-adopt-src/gone-skill
printf -- "---\nname: adopt-skill\n---\n# Adopt\n" > /tmp/ss-adopt-src/adopt-skill/SKILL.md
printf -- "---\nname: gone-skill\n---\n# Gone\n" > /tmp/ss-adopt-src/gone-skill/SKILL.md
ss install /tmp/ss-adopt-src/adopt-skill -g
ss install /tmp/ss-adopt-src/gone-skill -g
ss sync -g
CLAUDE=$(ss status -g --json | jq -r '.targets[] | select(.name=="claude") | .path')
ls "$CLAUDE"
```

Expected:
- exit_code: 0
- adopt-skill
- gone-skill

### 2. Records are keyed by the flat names

```bash
jq -c '.entries | {adopt: .["adopt-skill"].type, gone: .["gone-skill"].type}' $XDG_CONFIG_HOME/skillshare/skills/.metadata.json
```

Expected:
- exit_code: 0
- jq: .adopt == "local"
- jq: .gone == "local"

### 3. Move one skill by hand and delete the other

```bash
SRC="$XDG_CONFIG_HOME/skillshare/skills"
mkdir -p "$SRC/grp"
mv "$SRC/adopt-skill" "$SRC/grp/adopt-skill"
rm -rf "$SRC/gone-skill"
ls "$SRC/grp"
```

Expected:
- exit_code: 0
- adopt-skill

### 4. Dry-run sync reports a dry run

```bash
ss sync -g --dry-run --json
```

Expected:
- exit_code: 0
- jq: .dry_run == true

### 5. Dry run changed no record

```bash
jq -c '.entries | keys' $XDG_CONFIG_HOME/skillshare/skills/.metadata.json
```

Expected:
- exit_code: 0
- jq: index("adopt-skill") != null
- jq: index("grp/adopt-skill") == null
- jq: index("gone-skill") != null

### 6. Real sync succeeds

```bash
ss sync -g --json
```

Expected:
- exit_code: 0
- jq: .dry_run == false

### 7. The moved record followed and the gone record survived

```bash
jq -c '.entries | {keys: keys, group: .["grp/adopt-skill"].group, type: .["grp/adopt-skill"].type}' $XDG_CONFIG_HOME/skillshare/skills/.metadata.json
```

Expected:
- exit_code: 0
- jq: (.keys | index("grp/adopt-skill")) != null
- jq: (.keys | index("adopt-skill")) == null
- jq: (.keys | index("gone-skill")) != null
- jq: .group == "grp"
- jq: .type == "local"

### 8. The target link uses the new flat name

```bash
CLAUDE=$(ss status -g --json | jq -r '.targets[] | select(.name=="claude") | .path')
ls "$CLAUDE"
echo "gone-count=$(ls "$CLAUDE" | grep -c gone-skill)"
```

Expected:
- exit_code: 0
- grp__adopt-skill
- gone-count=0

### 9. uninstall finds the skill by its new path

```bash
ss uninstall grp/adopt-skill -g --json
```

Expected:
- exit_code: 0

### 10. The record is gone after uninstall

```bash
jq -c '.entries | keys' $XDG_CONFIG_HOME/skillshare/skills/.metadata.json
```

Expected:
- exit_code: 0
- jq: index("grp/adopt-skill") == null

## Pass Criteria

- Step 5: `sync --dry-run` leaves `adopt-skill` and `gone-skill` keyed at their old paths.
- Step 7: `sync` re-keys `adopt-skill` to `grp/adopt-skill` with group `grp` and keeps `gone-skill`.
- Step 8: the claude target holds `grp__adopt-skill` and the removed `gone-skill` link is pruned.
- Step 9: `uninstall grp/adopt-skill` exits 0.
