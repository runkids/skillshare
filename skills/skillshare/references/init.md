# Init Command

Initializes skillshare configuration.

## Key Concept

**Source is always `~/.config/skillshare/skills`** — never a CLI directory like `.claude/skills`.

- `--copy-from claude` = import skills FROM claude INTO source
- `--copy-from` does NOT change where source is located

## Copy Source Flags (mutually exclusive)

| Flag | Description |
|------|-------------|
| `--copy-from <name\|path>` | Copy skills from target name or directory path |
| `--no-copy` | Start with empty source |

## Target Flags (mutually exclusive)

| Flag | Description |
|------|-------------|
| `--targets <list>` | Comma-separated targets: `"claude,cursor,codex"` |
| `--all-targets` | Add all detected CLI targets |
| `--no-targets` | Skip target setup |

## Git Flags (mutually exclusive)

| Flag | Description |
|------|-------------|
| `--git` | Initialize git in source (recommended) |
| `--no-git` | Skip git initialization |

## Discover Flags (for adding new agents to existing config)

| Flag | Description |
|------|-------------|
| `--discover` | Detect and add new agents to existing config (interactive) |
| `--select <list>` | Comma-separated agents to add (non-interactive, requires `--discover`) |

## Other Flags

| Flag | Description |
|------|-------------|
| `--source <path>` | Custom source directory (**only if user explicitly requests**) |
| `--remote <url>` | Set git remote (implies `--git`) |
| `--dry-run` | Preview without making changes |

**AI Note:** Never use `--source` unless the user explicitly asks to change the source location.

## Examples

```bash
# Fresh start with all targets and git
skillshare init --no-copy --all-targets --git

# Copy from Claude, specific targets
skillshare init --copy-from claude --targets "claude,cursor" --git

# Minimal setup
skillshare init --no-copy --no-targets --no-git

# Custom source with remote
skillshare init --source ~/my-skills --remote git@github.com:user/skills.git

# Add new agents to existing config (non-interactive)
skillshare init --discover --select "windsurf,kilocode"

# Add new agents (interactive)
skillshare init --discover
```
