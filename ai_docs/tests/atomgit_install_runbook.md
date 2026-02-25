# AtomGit Install E2E Test

## Scope

Verify that skillshare can install skills from AtomGit (Chinese git platform) using full HTTPS URLs. This confirms the "any Git host" promise works for non-standard platforms.

## Known Limitation

AtomGit's git protocol (clone/ls-remote) returns **403** for non-China IPs. The HTTPS web interface (curl) returns 200, but `git clone` is geo-blocked. This means this runbook **can only run from a China-based machine or VPN**. The devcontainer (US/EU) cannot complete Steps 1-7.

**Verified via unit tests**: `gitHTTPSPattern` correctly parses `https://atomgit.com/owner/repo` — the URL parsing layer is fully covered.

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
- Install completes without error
- Output contains "Installed" or skill name
- Exit code 0

### Step 2: Verify skill appears in list

```bash
ss list --no-tui
```

**Expected:**
- Output contains at least one skill name from the installed repo
- Skills are listed under source directory

### Step 3: Verify skill files exist on disk

```bash
ls ~/.config/skillshare/skills/
```

**Expected:**
- At least one directory exists (skill from AtomGit repo)
- Directory contains a `SKILL.md` or markdown files

### Step 4: Verify sync distributes to targets

```bash
ss sync
```

**Expected:**
- Sync completes without error
- Exit code 0

### Step 5: Verify symlinks created in Claude target

```bash
ls -la ~/.claude/skills/
```

**Expected:**
- Symlinks exist pointing to `~/.config/skillshare/skills/` source
- At least one symlink from the AtomGit-installed skill

### Step 6: Uninstall the skill

```bash
ss uninstall cangjie-docs-mcp --force
```

**Expected:**
- Uninstall completes without error
- Skill moved to trash
- Exit code 0

### Step 7: Verify cleanup

```bash
ss list --no-tui
```

**Expected:**
- The uninstalled skill no longer appears in the list

## Pass Criteria

All 7 steps pass. The full install → list → sync → uninstall lifecycle works for an AtomGit-hosted repository.
