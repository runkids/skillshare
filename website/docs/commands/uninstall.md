---
sidebar_position: 3
---

# uninstall

Remove a skill or tracked repository from the source directory. Skills are moved to trash and kept for 7 days before automatic cleanup.

```bash
skillshare uninstall my-skill          # Remove a skill
skillshare uninstall team-repo         # Remove tracked repository (_ prefix optional)
skillshare uninstall my-skill --force  # Skip confirmation
```

![uninstall demo](/img/uninstall-demo.png)

## What Happens

```
┌─────────────────────────────────────────────────────────────────┐
│ skillshare uninstall my-skill                                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. Locate skill in source directory                             │
│    → ~/.config/skillshare/skills/my-skill/                      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Confirm removal (unless --force)                             │
│    → "Are you sure you want to uninstall this skill? [y/N]"     │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Move to trash (kept 7 days)                                  │
│    → ~/.local/share/skillshare/trash/my-skill_<timestamp>/      │
└─────────────────────────────────────────────────────────────────┘
```

## Options

| Flag | Description |
|------|-------------|
| `--force, -f` | Skip confirmation and ignore uncommitted changes |
| `--dry-run, -n` | Preview without making changes |
| `--help, -h` | Show help |

## Tracked Repositories

For tracked repositories (folders starting with `_`):

- Checks for uncommitted changes (use `--force` to override)
- Automatically removes the entry from `.gitignore`
- The `_` prefix is optional when uninstalling

```bash
skillshare uninstall _team-skills        # With prefix
skillshare uninstall team-skills         # Without prefix (auto-detected)
skillshare uninstall _team-skills --force # Force remove with uncommitted changes
```

## Examples

```bash
# Remove a regular skill
skillshare uninstall my-skill

# Preview removal
skillshare uninstall my-skill --dry-run

# Remove tracked repository
skillshare uninstall team-repo

# Force remove (skip confirmation)
skillshare uninstall my-skill --force
```

## Safety

Uninstalled skills are **moved to trash**, not permanently deleted:

- **Location:** `~/.local/share/skillshare/trash/` (global) or `.skillshare/trash/` (project)
- **Retention:** 7 days, then automatically cleaned up
- **Reinstall hint:** If the skill was installed from a remote source, the reinstall command is shown
- **Restore:** Use `skillshare trash restore <name>` to recover from trash

```
✓ Uninstalled: my-skill
ℹ Moved to trash (7 days): ~/.local/share/skillshare/trash/my-skill_2026-01-20_15-30-00
ℹ Reinstall: skillshare install github.com/user/repo/my-skill
```

To restore an accidentally uninstalled skill:

```bash
skillshare trash list                  # See what's in trash
skillshare trash restore my-skill      # Restore to source
skillshare sync                        # Sync back to targets
```

## After Uninstalling

Run `skillshare sync` to remove the skill from all targets:

```bash
skillshare uninstall old-skill
skillshare sync  # Remove from Claude, Cursor, etc.
```

## Project Mode

Uninstall a skill or tracked repo from the project's `.skillshare/skills/`:

```bash
skillshare uninstall my-skill -p          # Remove a skill
skillshare uninstall team-skills -p       # Remove tracked repo (_ prefix optional)
skillshare uninstall team-skills -p -f    # Force remove with uncommitted changes
```

In project mode, uninstall:
- Moves the skill directory to `.skillshare/trash/` (kept 7 days)
- Removes the skill's entry from `.skillshare/config.yaml` `skills:` list (for remote skills)
- Removes the entry from `.skillshare/.gitignore` (for remote/tracked skills)
- For tracked repos: checks for uncommitted changes (use `--force` to override)
- The `_` prefix is optional — auto-detected

```bash
skillshare uninstall pdf -p
skillshare sync
git add .skillshare/ && git commit -m "Remove pdf skill"
```

## Related

- [install](/docs/commands/install) — Install skills
- [list](/docs/commands/list) — List installed skills
- [trash](/docs/commands/trash) — Manage trashed skills
- [Project Skills](/docs/concepts/project-skills) — Project mode concepts
