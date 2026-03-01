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

### I use a dotfiles manager (stow/chezmoi/yadm) — will skillshare break my symlinks?

No. Since v0.16.7, skillshare detects external symlinks on both source and target directories and preserves them. Sync resolves the symlinks and operates on the underlying directories without removing the links themselves. See [Dotfiles Manager Compatibility](/docs/reference/commands/sync#dotfiles-manager-compatibility) for details.

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

See [Why Sync is a Separate Step](/docs/understand/source-and-targets#why-sync-is-a-separate-step) for the full design rationale.

### How do I sync across multiple machines?

Use git-based cross-machine sync:

```bash
# Machine A: push changes
skillshare push -m "Add new skill"

# Machine B: pull and sync
skillshare pull
```

See [Cross-Machine Sync](/docs/how-to/sharing/cross-machine-sync) for full setup.

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
| **Location** | `~/.local/share/skillshare/backups/` | `~/.local/share/skillshare/trash/` |
| **Triggered by** | `sync`, `target remove` | `uninstall` |
| **Restore with** | `skillshare restore <target>` | `skillshare trash restore <name>` |
| **Auto-cleanup** | manual (`backup --cleanup`) | 7 days |

They are complementary — backup protects targets from sync changes, trash protects source skills from accidental deletion.

### Can I sync specific skills to specific CLIs?

Yes. For example, skill A only to Claude, skill B to Gemini and Codex, skill C to all:

**Option 1: `targets` field in SKILL.md** (set by skill author)

```yaml
# skills/skill-a/SKILL.md
---
name: skill-a
targets: [claude]
---
```

```yaml
# skills/skill-b/SKILL.md
---
name: skill-b
targets: [gemini, codex]
---
```

```yaml
# skills/skill-c/SKILL.md — no targets field = syncs to all
---
name: skill-c
---
```

**Option 2: `include`/`exclude` filters in config** (set by consumer)

```yaml
# ~/.config/skillshare/config.yaml
targets:
  claude:
    path: ~/.claude/skills
    include: [skill-a, skill-c]
  codex:
    path: ~/.codex/skills
    include: [skill-b, skill-c]
```

Both approaches can be combined — config filters are applied first, then the skill-level `targets` field. See [Skill Format — `targets`](/docs/understand/skill-format#targets) and [Configuration — filters](/docs/reference/targets/configuration#skill-level-targets).

---

## Targets

### Using universal alongside npx skills

The `universal` target points to `~/.agents/skills`, the same directory used by the [npx skills CLI](https://github.com/vercel-labs/skills). Both tools can manage this directory simultaneously with some caveats:

**What works:**
- In merge mode (default), skillshare creates **symlinks** in `~/.agents/skills/`; npx skills creates **real directories**. They coexist as long as skill names don't collide.
- skillshare's prune logic only removes entries it manages — it won't delete files installed by npx skills.
- Agent CLIs (Claude Code, Cursor, etc.) read directories directly, so they see skills from both tools.

**What to watch out for:**
- **Name collision** — If both tools install a skill with the same name, the last sync/install wins. Avoid installing the same skill through both tools.
- **Copy mode is more aggressive** — In copy mode (`skillshare target universal --mode copy`), skillshare overwrites managed directories on every sync. If npx skills modifies a same-named skill between syncs, skillshare will replace it. Merge mode (default) only creates symlinks, which is safer for coexistence.
- **`npx skills list` won't show skillshare skills** — The npx skills CLI tracks installs via a lock file (`~/.agents/.skill-lock.json`), not by scanning the directory. Skills synced by skillshare won't appear in `npx skills list -g`, but they **are** visible to agent CLIs.
- **Other agent-specific targets still useful** — `universal` and `claude` point to different paths (`~/.agents/skills` vs `~/.claude/skills`). Selecting both is safe and not redundant.

**Recommended workflow:**
```bash
# Use skillshare as your primary skill manager
skillshare install github.com/user/skills --track
skillshare sync

# Use npx skills only for one-off community installs you don't need to sync
npx skills add someone/skill -g
```

:::tip
Keep the universal target in **merge mode** (default) for the safest coexistence with npx skills. Avoid switching to copy mode unless you don't use npx skills at all.
:::

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

See [Skill Format](/docs/understand/skill-format) for full details.

### What does "unknown target" warning mean?

When running `skillshare check` or `skillshare doctor`, you may see:

```
! Skill targets: my-skill: unknown target "*"
```

This means the skill's `SKILL.md` frontmatter has a `targets` field with an unrecognized name — often `"*"` (wildcard). skillshare expects **exact target names** (e.g., `claude`, `cursor`, `codex`), not glob patterns.

**If you want a skill to sync to all targets**, omit the `targets` field entirely:

```yaml
---
name: my-skill
description: Works everywhere
# no targets field = syncs to all targets
---
```

**If this warning is from a third-party skill**, the skill author used an unsupported syntax. You can:
1. **Ignore the warning** — the skill still installs, it just won't auto-filter to specific targets
2. **Fork and fix** — Remove or correct the `targets` field in the skill's `SKILL.md`

See [Skill Format — `targets`](/docs/understand/skill-format#targets) for the full specification.

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
~/.local/share/skillshare/backups/<timestamp>/
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

See [audit command](/docs/reference/commands/audit) for the full list of detection patterns.

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

- [Common Errors](./common-errors.md) — Error solutions
- [Windows](./windows.md) — Windows-specific FAQ
- [Troubleshooting Workflow](./troubleshooting-workflow.md) — Step-by-step debugging
