# Extras

Manage non-skill resources (rules, commands, prompts) that sync to arbitrary directories.

| Command | What it does | Project? | `--json`? |
|---------|-------------|:--------:|:---------:|
| `extras init <name>` | Create a new extra | ✓ (auto) | ✗ |
| `extras list` | List all extras + sync status | ✓ (auto) | ✓ |
| `extras remove <name>` | Remove an extra from config | ✓ (auto) | ✗ |
| `extras collect <name>` | Collect target files into source | ✓ (auto) | ✗ |
| `sync extras` | Sync all extras to targets | ✓ (auto) | ✓ |
| `diff --extras` | Show extras diff | ✓ (auto) | ✓ |

**Source directories:**
- Global: `~/.config/skillshare/extras/<name>/`
- Project: `.skillshare/extras/<name>/`

## extras init

Create a new extra resource type. Without arguments, launches an interactive TUI wizard.

```bash
skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules
skillshare extras init commands --target ~/.claude/commands --mode copy
skillshare extras init prompts --target .claude/prompts -p
skillshare extras init                    # TUI wizard
skillshare extras init rules --no-tui ... # Skip wizard
```

| Flag | Description |
|------|-------------|
| `--target <path>` | Target directory (repeatable, at least one required) |
| `--mode <mode>` | Sync mode: `merge` (default), `copy`, `symlink` |
| `--no-tui` | Skip interactive wizard |
| `-p` / `-g` | Force project / global mode |

## extras list

Show all configured extras with sync status per target.

```bash
skillshare extras list
skillshare extras list --json
skillshare extras list -p
```

Statuses: `synced`, `drift`, `not synced`, `no source`.

JSON output returns an array of:
```json
[{
  "name": "rules",
  "source_dir": "~/.config/skillshare/extras/rules",
  "file_count": 3,
  "source_exists": true,
  "targets": [{"path": "~/.claude/rules", "mode": "merge", "status": "synced"}]
}]
```

## extras remove

Remove an extra from config. Source files are preserved.

```bash
skillshare extras remove rules
skillshare extras remove rules --force    # Skip confirmation
skillshare extras remove prompts -p
```

| Flag | Description |
|------|-------------|
| `--force` / `-f` | Skip y/N confirmation prompt |
| `-p` / `-g` | Force project / global mode |

After removal, run `sync extras` to clean up orphaned links.

## extras collect

Collect local (non-symlinked) files from a target back into the extras source directory.

```bash
skillshare extras collect rules
skillshare extras collect rules --from ~/.claude/rules --dry-run
skillshare extras collect prompts -p
```

| Flag | Description |
|------|-------------|
| `--from <path>` | Target to collect from (required if multiple targets) |
| `--dry-run` | Preview without changes |
| `-p` / `-g` | Force project / global mode |

## sync extras

Distribute extra files from source to all configured targets.

```bash
skillshare sync extras              # Sync all extras
skillshare sync extras --dry-run    # Preview
skillshare sync extras --force      # Overwrite conflicts
skillshare sync extras --json       # JSON output
skillshare sync --all               # Skills + extras together
```

Sync modes (per-target):
- `merge` (default): per-file symlinks, preserves local files in target
- `copy`: real-file copies
- `symlink`: entire directory symlinked

## diff --extras

Show what would change if you ran `sync extras`.

```bash
skillshare diff --extras            # Extras only
skillshare diff --all               # Skills + extras
skillshare diff --extras --json     # JSON output
```

## Config format

```yaml
extras:
  - name: rules
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy
  - name: commands
    targets:
      - path: ~/.claude/commands
        mode: symlink
```

## Typical workflow

```bash
# 1. Create an extra
skillshare extras init rules --target ~/.claude/rules --target ~/.cursor/rules

# 2. Add files to source
cp my-rule.md ~/.config/skillshare/extras/rules/

# 3. Sync to targets
skillshare sync extras

# 4. Verify
skillshare extras list
skillshare diff --extras

# 5. Or collect existing local files first
skillshare extras collect rules --from ~/.claude/rules
skillshare sync extras
```
