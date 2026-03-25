# CLI E2E Runbook: Extras Flatten Option

Validates `flatten: true` for extras targets — syncing subdirectory files
directly into the target root instead of preserving directory structure.

**Origin**: Issue #97 — flatten option for extras sync.

## Scope

- `flatten: true` syncs files flat (ignoring subdirectory structure)
- `--flatten` flag on `extras init`
- `--flatten` / `--no-flatten` on `extras mode`
- Filename collision: first file wins, subsequent skipped with warning
- Flatten + symlink mode: rejected
- Flatten prune
- JSON output includes flatten field and warnings

## Environment

Run inside devcontainer.

## Steps

### 1. Flatten sync — files from subdirectories land in target root

```bash
ss extras remove agents --force -g >/dev/null 2>&1 || true
rm -rf ~/.claude/agents 2>/dev/null || true
mkdir -p ~/.config/skillshare/extras/agents/curriculum
mkdir -p ~/.config/skillshare/extras/agents/software
echo "# Tactician" > ~/.config/skillshare/extras/agents/curriculum/tactician.md
echo "# Planner" > ~/.config/skillshare/extras/agents/curriculum/planner.md
echo "# Implementer" > ~/.config/skillshare/extras/agents/software/implementer.md
echo "# Reviewer" > ~/.config/skillshare/extras/agents/reviewer.md
mkdir -p ~/.claude/agents
ss extras init agents --target ~/.claude/agents --flatten -g >/dev/null
ss sync extras --json -g
```

Expected:
- exit_code: 0
- jq: .extras[0].targets[0].synced == 4

### 2. Verify flat file layout — no subdirectories in target

```bash
ss extras remove agents --force -g >/dev/null 2>&1 || true
rm -rf ~/.claude/agents 2>/dev/null || true
mkdir -p ~/.config/skillshare/extras/agents/sub1 ~/.config/skillshare/extras/agents/sub2
echo "a" > ~/.config/skillshare/extras/agents/sub1/a.md
echo "b" > ~/.config/skillshare/extras/agents/sub2/b.md
echo "c" > ~/.config/skillshare/extras/agents/root.md
mkdir -p ~/.claude/agents
ss extras init agents --target ~/.claude/agents --flatten -g >/dev/null
ss sync extras -g >/dev/null
echo "files=$(find ~/.claude/agents/ -maxdepth 1 -name '*.md' | wc -l)"
echo "dirs=$(find ~/.claude/agents/ -mindepth 1 -type d | wc -l)"
```

Expected:
- exit_code: 0
- files=3
- dirs=0

### 3. Extras list JSON includes flatten field

```bash
ss extras remove agents --force -g >/dev/null 2>&1 || true
rm -rf ~/.claude/agents 2>/dev/null || true
mkdir -p ~/.config/skillshare/extras/agents
echo "x" > ~/.config/skillshare/extras/agents/x.md
mkdir -p ~/.claude/agents
ss extras init agents --target ~/.claude/agents --flatten -g >/dev/null
ss extras list --json -g
```

Expected:
- exit_code: 0
- jq: .[0].targets[0].flatten == true

### 4. Flatten collision — first file wins, second skipped with warning

```bash
ss extras remove agents --force -g >/dev/null 2>&1 || true
rm -rf ~/.claude/agents 2>/dev/null || true
mkdir -p ~/.config/skillshare/extras/agents/team-a
mkdir -p ~/.config/skillshare/extras/agents/team-b
echo "# From team-a" > ~/.config/skillshare/extras/agents/team-a/agent.md
echo "# From team-b" > ~/.config/skillshare/extras/agents/team-b/agent.md
mkdir -p ~/.claude/agents
ss extras init agents --target ~/.claude/agents --flatten -g >/dev/null
ss sync extras --json -g
```

Expected:
- exit_code: 0
- jq: .extras[0].targets[0].synced == 1
- jq: .extras[0].targets[0].skipped == 1
- jq: .extras[0].targets[0].warnings | length == 1

### 5. Flatten collision warning in human-readable output

```bash
ss extras remove agents --force -g >/dev/null 2>&1 || true
rm -rf ~/.claude/agents 2>/dev/null || true
mkdir -p ~/.config/skillshare/extras/agents/a ~/.config/skillshare/extras/agents/b
echo "1" > ~/.config/skillshare/extras/agents/a/same.md
echo "2" > ~/.config/skillshare/extras/agents/b/same.md
mkdir -p ~/.claude/agents
ss extras init agents --target ~/.claude/agents --flatten -g >/dev/null
ss sync extras -g
```

Expected:
- exit_code: 0
- flatten conflict

### 6. Toggle flatten off via extras mode

```bash
ss extras remove agents --force -g >/dev/null 2>&1 || true
rm -rf ~/.claude/agents 2>/dev/null || true
mkdir -p ~/.config/skillshare/extras/agents
echo "x" > ~/.config/skillshare/extras/agents/x.md
mkdir -p ~/.claude/agents
ss extras init agents --target ~/.claude/agents --flatten -g >/dev/null
ss extras agents --no-flatten -g >/dev/null
ss extras list --json -g
```

Expected:
- exit_code: 0
- jq: .[0].targets[0].flatten == false

### 7. Toggle flatten on via extras mode

```bash
ss extras remove agents --force -g >/dev/null 2>&1 || true
rm -rf ~/.claude/agents 2>/dev/null || true
mkdir -p ~/.config/skillshare/extras/agents
echo "x" > ~/.config/skillshare/extras/agents/x.md
mkdir -p ~/.claude/agents
ss extras init agents --target ~/.claude/agents -g >/dev/null
ss extras agents --flatten -g >/dev/null
ss extras list --json -g
```

Expected:
- exit_code: 0
- jq: .[0].targets[0].flatten == true

### 8. Flatten + symlink mode rejected

```bash
ss extras remove agents --force -g >/dev/null 2>&1 || true
rm -rf ~/.claude/agents 2>/dev/null || true
mkdir -p ~/.config/skillshare/extras/agents
echo "x" > ~/.config/skillshare/extras/agents/x.md
mkdir -p ~/.claude/agents
ss extras init agents --target ~/.claude/agents --flatten -g >/dev/null
ss extras agents --mode symlink -g 2>&1 || true
```

Expected:
- flatten cannot be used with symlink mode

### 9. Flatten prune — orphaned flat file removed after source deletion

```bash
ss extras remove agents --force -g >/dev/null 2>&1 || true
rm -rf ~/.claude/agents 2>/dev/null || true
mkdir -p ~/.config/skillshare/extras/agents/sub
echo "# Keep" > ~/.config/skillshare/extras/agents/sub/keep.md
echo "# Remove" > ~/.config/skillshare/extras/agents/sub/remove.md
mkdir -p ~/.claude/agents
ss extras init agents --target ~/.claude/agents --flatten -g >/dev/null
ss sync extras -g >/dev/null
rm ~/.config/skillshare/extras/agents/sub/remove.md
ss sync extras --json -g
```

Expected:
- exit_code: 0
- jq: .extras[0].targets[0].pruned == 1

## Pass Criteria

- All steps pass
- Flatten sync produces flat symlinks (no subdirectories in target)
- Collision detection works with correct warning and skip
- Flatten toggle via `--flatten`/`--no-flatten` works
- Flatten + symlink mode is rejected
- Prune works with flattened paths
