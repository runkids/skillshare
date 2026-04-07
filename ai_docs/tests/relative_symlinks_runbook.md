# Relative Symlinks E2E Test

Validates that project-mode sync creates relative symlinks (committable to git),
while global-mode sync continues to create absolute symlinks.

## Scope

- `internal/sync/sync.go` — `SyncTargetMergeWithSkills` passes `projectRoot`
- `internal/sync/symlink_unix.go` — `createLink` computes relative path
- `internal/sync/relative.go` — `shouldUseRelative` determines mode
- `internal/sync/extras.go` — extras use `createLink` with relative support

## Environment

- Devcontainer with `ssenv` isolation
- Pre-initialized via `ssenv create --init`

## Steps

### Step 1: Create project with skill and sync

```bash
rm -rf /tmp/reltest 2>/dev/null
mkdir -p /tmp/reltest/.skillshare/skills/demo
printf '%s\n' '---' 'name: demo' '---' '# Demo skill' > /tmp/reltest/.skillshare/skills/demo/SKILL.md
printf '%s\n' 'targets:' '  - claude' > /tmp/reltest/.skillshare/config.yaml
cd /tmp/reltest && ss sync -p
```

Expected:
- exit_code: 0
- 1 linked

### Step 2: Verify project-mode symlink is relative

```bash
readlink /tmp/reltest/.claude/skills/demo
```

Expected:
- exit_code: 0
- regex: ^\.\.

### Step 3: Verify content is accessible through relative symlink

```bash
cat /tmp/reltest/.claude/skills/demo/SKILL.md
```

Expected:
- exit_code: 0
- Demo skill

### Step 4: Verify target outside project root falls back to absolute

```bash
rm -rf /tmp/reltest-outside /tmp/outside-target 2>/dev/null
mkdir -p /tmp/reltest-outside/.skillshare/skills/ext
printf '%s\n' '---' 'name: ext' '---' '# External' > /tmp/reltest-outside/.skillshare/skills/ext/SKILL.md
mkdir -p /tmp/outside-target
printf '%s\n' 'targets:' '  - name: custom' '    skills:' '      path: /tmp/outside-target' > /tmp/reltest-outside/.skillshare/config.yaml
cd /tmp/reltest-outside && ss sync -p
readlink /tmp/outside-target/ext
```

Expected:
- exit_code: 0
- regex: ^/

### Step 5: Project-mode sync JSON output

```bash
cd /tmp/reltest && ss sync -p --json
```

Expected:
- exit_code: 0
- jq: .details | length == 1
- jq: .details[0].name == "claude"

### Step 6: Project-mode with multiple targets

```bash
rm -rf /tmp/reltest2 2>/dev/null
mkdir -p /tmp/reltest2/.skillshare/skills/multi
printf '%s\n' '---' 'name: multi' '---' '# Multi' > /tmp/reltest2/.skillshare/skills/multi/SKILL.md
printf '%s\n' 'targets:' '  - claude' '  - cursor' > /tmp/reltest2/.skillshare/config.yaml
cd /tmp/reltest2 && ss sync -p
CLAUDE_LINK=$(readlink .claude/skills/multi 2>/dev/null)
CURSOR_LINK=$(readlink .cursor/skills/multi 2>/dev/null)
echo "claude=$CLAUDE_LINK"
echo "cursor=$CURSOR_LINK"
```

Expected:
- exit_code: 0
- regex: claude=\.\.
- regex: cursor=\.\.

### Step 7: Relative symlink resolves correctly

```bash
cd /tmp/reltest
RESOLVED=$(readlink -f .claude/skills/demo)
EXPECTED=$(readlink -f .skillshare/skills/demo)
echo "resolved=$RESOLVED"
echo "expected=$EXPECTED"
test "$RESOLVED" = "$EXPECTED" && echo "MATCH" || echo "MISMATCH"
```

Expected:
- exit_code: 0
- MATCH

### Step 8: Re-sync is idempotent

```bash
cd /tmp/reltest && ss sync -p
LINK=$(readlink .claude/skills/demo)
echo "link=$LINK"
```

Expected:
- exit_code: 0
- regex: link=\.\.
- 1 linked

### Step 9: Symlink mode produces relative symlink

```bash
rm -rf /tmp/reltest-sym 2>/dev/null
mkdir -p /tmp/reltest-sym/.skillshare/skills/sym-skill
printf '%s\n' '---' 'name: sym-skill' '---' '# Symlink mode' > /tmp/reltest-sym/.skillshare/skills/sym-skill/SKILL.md
printf '%s\n' 'targets:' '  - name: claude' '    skills:' '      mode: symlink' > /tmp/reltest-sym/.skillshare/config.yaml
cd /tmp/reltest-sym && ss sync -p
readlink .claude/skills
```

Expected:
- exit_code: 0
- regex: ^\.\.

### Step 10: Cleanup

```bash
rm -rf /tmp/reltest /tmp/reltest2 /tmp/reltest-outside /tmp/outside-target /tmp/reltest-sym 2>/dev/null
echo "cleanup done"
```

Expected:
- exit_code: 0
- cleanup done

## Pass Criteria

- All steps pass
- Project-mode symlinks are relative (start with `..`)
- Global-mode symlinks are absolute (start with `/`)
- Content accessible through relative symlinks
- Multiple targets all get relative symlinks
