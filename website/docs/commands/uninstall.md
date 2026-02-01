---
sidebar_position: 3
---

# uninstall

Remove a skill or tracked repository from the source directory.

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
│ 3. Remove from source                                           │
│    → Deleted: ~/.config/skillshare/skills/my-skill/             │
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

## After Uninstalling

Run `skillshare sync` to remove the skill from all targets:

```bash
skillshare uninstall old-skill
skillshare sync  # Remove from Claude, Cursor, etc.
```

## Related

- [install](/docs/commands/install) — Install skills
- [list](/docs/commands/list) — List installed skills
