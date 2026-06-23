# Extras

Manage non-skill resources (rules, commands, prompts) that sync to arbitrary directories.

| Command | What it does | Project? | `--json`? |
|---------|-------------|:--------:|:---------:|
| `extras init <name>` | Create a new extra | ✓ (auto) | ✗ |
| `extras list` | List all extras + sync status | ✓ (auto) | ✓ |
| `extras remove <name>` | Remove an extra from config | ✓ (auto) | ✗ |
| `extras collect <name>` | Collect target files into source | ✓ (auto) | ✗ |
| `sync extras` | Sync all extras to targets | ✓ (auto) | ✓ |
| `diff` | Includes extras diff automatically | ✓ (auto) | ✓ |

**Source directories:**
- Global: `~/.config/skillshare/extras/<name>/`; `extras_source` and per-extra `source` can override it.
- Project: `.skillshare/extras/<name>/`; per-extra `source` is ignored. Use top-level `sources.extras` to move all project extras.

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
| `--source <path>` | Custom source directory for this extra (global mode only) |
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

## diff (extras included)

`skillshare diff` automatically includes extras when configured — no extra flags needed.

```bash
skillshare diff                     # Skills + extras (if configured)
skillshare diff --json              # JSON output includes extras
```

## Config format

```yaml
extras:
  - name: rules
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy
  - name: agents
    targets:
      - path: .claude/agents
      - path: .codex/agents
        flatten: true
        extension: codex-agents   # transform + rename via .skillshare/extensions/codex-agents/
  - name: commands
    targets:
      - path: ~/.claude/commands
        mode: symlink
```

The `extension:` field names an extension directory under `.skillshare/extensions/` (project) or `~/.config/skillshare/extensions/` (global). It transforms each source file during sync and implies `copy` mode.

For project agents, prefer native target `agents:` config. Use `extras: agents` only when you need extras-only behavior like `flatten` or `extension`.

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

## Extensions

An extension transforms each source file before writing it to a target. Set `extension: <name>` on any extras target; implies `copy` mode.

### Directory layout

```
.skillshare/extensions/<name>/
├── extension.yaml   ← required
├── convert.js       ← transformer (or convert.py, etc.)
└── helper.js        ← optional shared utilities
```

### extension.yaml

```yaml
run: ["node", "convert.js"]   # command array — run from extension directory
output_ext: toml              # renames output file (e.g. rule.md → rule.toml)
description: "MD → Codex TOML"
```

- `output_ext` is the only way to change the output file extension
- Omit to keep the source extension
- Global extensions: `~/.config/skillshare/extensions/<name>/`

### I/O contract

- Input: raw source file piped to `stdin`
- Output: transformed content on `stdout`; non-zero exit skips the file with a warning
- `SS_REL_PATH` env var: source file path relative to extras root

### Official extensions

Ready-to-copy reference implementations at `https://github.com/runkids/skillshare/tree/main/extensions`:

| Extension | Converts | Output |
|-----------|----------|--------|
| `codex-agents` | Claude agent MD (frontmatter + body) | Codex TOML (`name`, `description`, `developer_instructions`) |
| `gemini-commands` | Markdown command docs | Gemini CLI TOML commands |

The `codex-agents` extension requires `name` and `description` frontmatter — files missing either field are skipped with an error. Non-agent files (e.g. prompts, changelogs) should be excluded from the extras source or given a `.agentignore`-style prefix.

### Caveats

- Native agents targets (`agents: { path: ... }`) do **not** support `extension:` — extras only
- Node.js extensions break inside Claude Code because it sets `NODE_OPTIONS` to preload an internal module. Fix: `run: ["env", "-u", "NODE_OPTIONS", "node", "convert.js"]`
