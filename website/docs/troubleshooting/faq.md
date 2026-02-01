---
sidebar_position: 4
---

# FAQ

Frequently asked questions about skillshare.

## General

### Isn't this just `ln -s`?

Yes, at its core. But skillshare handles:
- Multi-target detection
- Backup/restore
- Merge mode (per-skill symlinks)
- Cross-device sync
- Broken symlink recovery

So you don't have to.

### What happens if I modify a skill in the target directory?

Since targets are symlinks, changes are made directly to the source. All targets see the change immediately.

### How do I keep a CLI-specific skill?

Use `merge` mode (default). Local skills in the target won't be overwritten or synced.

```bash
skillshare target claude --mode merge
skillshare sync
```

Then create skills directly in `~/.claude/skills/` — they won't be touched.

---

## Installation

### Can I sync skills to a custom or uncommon tool?

Yes. Use `skillshare target add <name> <path>` with the tool's skills directory.

```bash
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
skillshare sync
```

### Can I use skillshare with a private git repo?

Yes. Use SSH URLs:

```bash
skillshare init --remote git@github.com:you/private-skills.git
```

---

## Sync

### How do I sync across multiple machines?

Use git-based cross-machine sync:

```bash
# Machine A: push changes
skillshare push -m "Add new skill"

# Machine B: pull and sync
skillshare pull
```

See [Cross-Machine Sync](/docs/guides/cross-machine-sync) for full setup.

### What if I accidentally delete a skill through a symlink?

If you have git initialized (recommended), recover with:

```bash
cd ~/.config/skillshare/skills
git checkout -- deleted-skill/
```

Or restore from backup:
```bash
skillshare restore claude
```

---

## Targets

### How does `target remove` work? Is it safe?

Yes, it's safe:

1. **Backup** — Creates backup of the target
2. **Detect mode** — Checks if symlink or merge mode
3. **Unlink** — Removes symlinks, copies source content back
4. **Update config** — Removes target from config.yaml

This is why `skillshare target remove` is safe, while `rm -rf ~/.claude/skills` would delete your source files.

### Why is `rm -rf` on a target dangerous?

In symlink mode, the entire target directory is a symlink to source. Deleting it deletes source.

In merge mode, each skill is a symlink. Deleting a skill through the symlink deletes the source file.

**Always use:**
```bash
skillshare target remove <name>   # Safe
skillshare uninstall <skill>      # Safe
```

---

## Tracked Repos

### How do tracked repos differ from regular skills?

| Aspect | Regular Skill | Tracked Repo |
|--------|---------------|--------------|
| Source | Copied to source | Cloned with `.git` |
| Update | `install --update` | `update <name>` (git pull) |
| Prefix | None | `_` prefix |
| Nested skills | Flattened | Flattened with `__` |

### Why the underscore prefix?

The `_` prefix identifies tracked repositories:
- Helps you distinguish from regular skills
- Prevents name collisions
- Shows in listings clearly

---

## Skills

### What's the SKILL.md format?

```markdown
---
name: skill-name
description: Brief description
---

# Skill Name

Instructions for the AI...
```

See [Skill Format](/docs/concepts/skill-format) for full details.

### Can a skill have multiple files?

Yes. A skill directory can contain:
- `SKILL.md` (required)
- Any additional files (examples, templates, etc.)

Reference them in your SKILL.md instructions.

---

## Performance

### Sync seems slow

Check for large files in your skills directory. Add ignore patterns:

```yaml
# ~/.config/skillshare/config.yaml
ignore:
  - "**/.DS_Store"
  - "**/.git/**"
  - "**/node_modules/**"
  - "**/*.log"
```

### How many skills can I have?

No hard limit. Performance depends on:
- Number of skills
- Size of skill files
- Number of targets

Thousands of small skills work fine.

---

## Backups

### Where are backups stored?

```
~/.config/skillshare/backups/<timestamp>/
```

### How long are backups kept?

By default, indefinitely. Clean up with:
```bash
skillshare backup --cleanup
```

---

## Getting Help

### Where do I report bugs?

[GitHub Issues](https://github.com/runkids/skillshare/issues)

### Where do I ask questions?

[GitHub Discussions](https://github.com/runkids/skillshare/discussions)

---

## Related

- [Common Errors](./common-errors) — Error solutions
- [Windows](./windows) — Windows-specific FAQ
- [Troubleshooting Workflow](/docs/workflows/troubleshooting-workflow) — Step-by-step debugging
