# AtomGit Install E2E Test

## Scope

Verify that skillshare can install skills from AtomGit (Chinese git platform) using full HTTPS URLs. This confirms the "any Git host" promise works for non-standard platforms.

## Known Limitation

AtomGit's git protocol (clone/ls-remote) may return **403** for non-China IPs. If `git clone` is geo-blocked, Step 1 will fail with a clone error. The URL parsing layer is fully covered via unit tests (`gitHTTPSPattern`).

## Environment

- Devcontainer with `ssenv` isolation (or China-based machine)
- Network access required (online test — clones from `atomgit.com`)
- Binary: `ss` (skillshare)

## Steps

### Step 1: Install from AtomGit full HTTPS URL

```bash
ss install https://atomgit.com/Cangjie-SIG/cangjie-docs-mcp --all
```

**Expected:**
- exit_code: 0
- Installed
- cangjie-docs

### Step 2: Verify skill appears in list

```bash
ss list --no-tui
```

**Expected:**
- exit_code: 0
- cangjie-docs-navigator

### Step 3: Verify skill files exist on disk

```bash
ls ~/.config/skillshare/skills/
```

**Expected:**
- exit_code: 0
- cangjie-docs-navigator

### Step 4: Verify sync distributes to targets

```bash
ss sync
```

**Expected:**
- exit_code: 0
- Sync complete
- regex: \d+ linked

### Step 5: Verify symlinks created in Claude target

```bash
ls -la ~/.claude/skills/
```

**Expected:**
- exit_code: 0
- cangjie-docs-navigator

### Step 6: Uninstall the skill

```bash
ss uninstall cangjie-docs-navigator --force
```

**Expected:**
- exit_code: 0
- Moved to trash

### Step 7: Verify cleanup

```bash
ss list --no-tui
```

**Expected:**
- exit_code: 0
- Not cangjie-docs-navigator

## Pass Criteria

All 7 steps pass. The full install → list → sync → uninstall lifecycle works for an AtomGit-hosted repository.
