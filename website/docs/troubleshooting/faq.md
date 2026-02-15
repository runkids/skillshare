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

### Why do I need to run `sync` after every install/update?

Sync is intentionally a separate step. Operations like `install`, `update`, and `uninstall` only modify the **source** directory — `sync` propagates those changes to all targets.

This lets you:
- **Batch changes** — Install 5 skills, then sync once instead of 5 times
- **Preview first** — Run `sync --dry-run` before applying
- **Stay in control** — You decide when targets update

**Note:** `pull` is the only command that auto-syncs, because its intent is "bring everything up to date."

See [Why Sync is a Separate Step](/docs/concepts/source-and-targets#why-sync-is-a-separate-step) for the full design rationale.

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

### What if I accidentally uninstall a skill?

Uninstalled skills are moved to trash and kept for 7 days. Restore with:

```bash
skillshare trash list                  # See what's in trash
skillshare trash restore my-skill      # Restore to source
skillshare sync                        # Sync back to targets
```

If the skill was installed from a remote source, you can also reinstall:

```bash
skillshare install github.com/user/repo/my-skill
skillshare sync
```

For project mode, trash is at `.skillshare/trash/` within the project directory. Use `-p` flag with trash commands.

Run `skillshare doctor` to see current trash status (item count, size, age).

### What's the difference between backup and trash?

| | backup | trash |
|---|---|---|
| **Protects** | target directories (sync snapshots) | source skills (uninstall) |
| **Location** | `~/.config/skillshare/backups/` | `~/.config/skillshare/trash/` |
| **Triggered by** | `sync`, `target remove` | `uninstall` |
| **Restore with** | `skillshare restore <target>` | `skillshare trash restore <name>` |
| **Auto-cleanup** | manual (`backup --cleanup`) | 7 days |

They are complementary — backup protects targets from sync changes, trash protects source skills from accidental deletion.

---

## Targets

### I used `claude-code` (or `gemini-cli`, etc.) as a project target — is that still valid?

Yes. Old project target names like `claude-code`, `gemini-cli`, `github-copilot` still resolve via aliases. However, the canonical name is now the same as the global name (e.g., `claude`, `gemini`, `copilot`). We recommend updating your `.skillshare/config.yaml` to use the short name:

```yaml
# Before
targets:
  - claude-code

# After
targets:
  - claude
```

### How does `target remove` work? Is it safe?

Yes, it's safe:

1. **Backup** — Creates backup of the target
2. **Detect mode** — Checks if symlink or merge mode
3. **Unlink** — Removes all skillshare-managed symlinks, copies source content back as real files. In merge mode, only symlinks pointing to the source directory are removed; local (non-symlink) skills are preserved.
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

## Security

### Can I trust third-party skills?

Skills are instructions for your AI agent — a malicious skill could tell the AI to exfiltrate secrets or run destructive commands. skillshare mitigates this with a built-in security scanner:

- **Auto-scan on install** — Every skill is scanned during `skillshare install`
- **CRITICAL findings block** — Prompt injection, data exfiltration, credential access are blocked by default
- **Manual scan** — Run `skillshare audit` to scan all installed skills at any time

See [audit command](/docs/commands/audit) for the full list of detection patterns.

### What if audit blocks my install?

If a skill triggers a CRITICAL finding, installation is blocked. You have two options:

1. **Review the finding** — Check if it's a false positive (e.g., a documentation example)
2. **Force install** — Use `--force` to bypass the check if you trust the source

```bash
skillshare install suspicious-skill --force
```

### Does audit catch everything?

No scanner is perfect. `skillshare audit` catches common patterns like prompt injection, `curl`/`wget` with secrets, credential file access, and obfuscated payloads. Always review skills from untrusted sources manually.

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
- [Troubleshooting Workflow](./troubleshooting-workflow) — Step-by-step debugging
