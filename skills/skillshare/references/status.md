# Status & Inspection Commands

Commands with auto-detection run in project mode when `.skillshare/config.yaml` exists in cwd. Use `-g` to force global.

## status

Overview of source, targets, and sync state.

```bash
skillshare status          # Auto-detects mode
skillshare status -g       # Force global
```

Project mode output includes: source path, targets with sync mode, remote skills list.

**Sync drift detection:** Warns when targets have fewer linked skills than source (merge mode). Example: `⚠ claude: 3 skill(s) not synced (12/15 linked)`. Run `skillshare sync` to fix.

## diff

Show differences between source and targets.

```bash
skillshare diff                # All targets
skillshare diff claude         # Specific target
```

## list

List installed skills.

```bash
skillshare list                # Auto-detects mode
skillshare list --verbose      # With source info
skillshare list -g             # Force global
```

Project mode shows local vs remote skills.

## search

Search GitHub for skills (repos containing SKILL.md).

```bash
skillshare search <query>           # Interactive (select to install)
skillshare search <query> --list    # List only
skillshare search <query> --json    # JSON output
skillshare search <query> -n 10     # Limit results (default: 20)
```

**Requires:** GitHub auth (`gh` CLI or `GITHUB_TOKEN` env var).

**Query examples:**
- `react performance` - Performance optimization
- `pr review` - Code review skills
- `commit` - Git commit helpers
- `changelog` - Changelog generation

## doctor

Diagnose configuration and environment issues. Also checks for sync drift.

```bash
skillshare doctor
```

## upgrade

Upgrade CLI binary and/or built-in skillshare skill.

```bash
skillshare upgrade              # Both CLI + skill
skillshare upgrade --cli        # CLI only
skillshare upgrade --skill      # Skill only
skillshare upgrade --force      # Skip confirmation
skillshare upgrade --dry-run    # Preview
```

**After upgrading skill:** `skillshare sync`
