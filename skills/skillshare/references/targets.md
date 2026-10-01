# Target Management

Manage AI CLI tool targets (Claude, Cursor, Windsurf, Firebender, etc.). Use `skillshare target list --json` to inspect configured targets; avoid assuming skill
and MCP client support are identical.

## Global Targets

```bash
skillshare target list                        # List all targets
skillshare target claude                      # Show target info
skillshare target add myapp ~/.myapp/skills   # Add custom target
skillshare target add claude-work --agent claude --config-dir ~/.claude-work   # Another account of an Agent
skillshare target remove myapp                # Remove target (safe)
```

Another account (`agent` + `config_dir` in config.yaml): `--agent` accepts `claude`
(`CLAUDE_CONFIG_DIR`), `codex` (`CODEX_HOME`) and `pi` (`PI_CODING_AGENT_DIR`). The skills
path follows the directory (`<config_dir>/skills` for Codex and Pi, even though Codex also
reads the shared `~/.agents/skills`); only Claude has an agents directory. Any number of
accounts can be added, and the target name is also valid in `mcp.targets` and a server's
`targets`. Optional `--cli <executable>` (`cli:` in config.yaml) runs the account's plugin
commands with a compatible CLI, such as `omo` for Pi: a name on PATH or an absolute path, no
arguments, shell aliases not seen. It affects plugins only.

Instruction file of a custom target (`instructions` in config.yaml; `target add` has no
flag for it, the dashboard sets it in the Custom target dialog or the target's file tab):

```yaml
targets:
  myapp:
    path: ~/.myapp/skills
    instructions:
      path: ~/.myapp/AGENTS.md   # global: absolute or ~/; project: relative to the root
      import: true               # the tool follows @path lines
```

Other files a tool reads (`files` in config.yaml, dashboard only): each entry is relative
to the tool's folder (`~/.pi/agent` for pi; `.pi` in a project) and gets its own tab.
Pi and oh-my-pi already get `APPEND_SYSTEM.md`. Removing an entry never deletes the file.

```yaml
targets:
  pi:
    files: [SYSTEM.md, prompts/review.md]
```

## Skills Off

For a tool that already reads another target's folder (Pi also reads `~/.agents/skills`
of `universal`), stop syncing skills to it so it does not find each skill twice. Agents, MCP
servers and instructions stay managed.

```bash
skillshare target pi --skills=false --dry-run   # Preview what is removed
skillshare target pi --skills=false             # Save skills.enabled: false, remove links into the source
skillshare target pi --skills=true              # Back on; next `skillshare sync` syncs again
skillshare target add gemini ~/.gemini/skills --no-skills   # Add with skills off
```

Merge mode removes the links; symlink mode removes the folder link; copy mode keeps the
copies and lists them apart (the tool still loads them; delete them to avoid duplicates);
a folder an enabled target also writes to is left alone. `sync`/`diff`/`status`/`doctor`
skip the target's skills. `--skills` cannot be combined with include/exclude flags in one
command. Works with `-p`.

When `sync` warns that two targets "sync skills to <folder> with different filters, so each
sync undoes the other", run the `skillshare target <name> --skills=false` it prints.

## Project Targets (`-p`)

```bash
skillshare target list -p                              # List project targets
skillshare target claude -p                       # Show project target info
skillshare target add windsurf -p                      # Add known target
skillshare target add custom-tool ./tools/skills -p    # Add custom path (relative)
skillshare target remove windsurf -p                   # Remove project target
```

**Config format** (`.skillshare/config.yaml`):

```yaml
targets:
  - claude                    # Short: known target, merge mode
  - name: cursor                   # Long: with explicit mode
    mode: symlink
  - name: custom-ide               # Long: with custom path
    path: ./tools/ide/skills
    mode: merge
```

## Project Folders From the Global Config (`projects`)

Use when projects should get **different** skills. Global targets already give every project the same set; project mode (`-p`) keeps the setup in the repo for teammates. For personal projects, list the folders in the global `config.yaml` instead of giving each a `.skillshare/`. One `skillshare sync` writes them all.

```yaml
projects:
  ~/work/shop-web:
    targets: [claude, cursor, codex]   # tools used there; paths are derived
    skills: { mode: copy, include: ["frontend-*"] }   # present = on, empty = all
    agents: {}
```

- Shows up in `sync`/`status`/`diff` as `<name>@<target>` (e.g. `shop-web@claude`). Tools sharing a folder (`.agents/skills`) are one target.
- A missing folder is skipped with a warning, never recreated.
- `skillshare target` and `collect` ignore these; edit `config.yaml` or the dashboard's Projects page.
- MCP for the same folders lives under `mcp.projects` (see [mcp.md](mcp.md)).

## Target Filters

Control which skills and agents sync to each target using include/exclude glob patterns.

```bash
# Add skill filters
skillshare target claude --add-include "team-*"       # Only sync matching skills
skillshare target claude --add-exclude "_legacy*"     # Skip matching skills
skillshare target claude --add-include "team-*" -p    # Project target filter

# Add agent filters
skillshare target claude --add-agent-include "team-*"
skillshare target claude --add-agent-exclude "draft-*"

# Remove filters
skillshare target claude --remove-include "team-*"
skillshare target claude --remove-exclude "_legacy*"
skillshare target claude --remove-agent-include "team-*"
```

**Config format** with filters:

```yaml
targets:
  - name: claude
    include: ["team-*", "core-*"]
    exclude: ["_legacy*"]
  - cursor                          # No filters = sync all skills
```

**Pattern syntax:** `filepath.Match` globs — `*` matches any non-separator chars, `?` matches single char.

**Precedence:** Include filters apply first (whitelist), then exclude filters remove from that set. No filters = all matching resources. Agent filters require a target with an agents path, and they are ignored in `symlink` mode.

## Skill-Level Targets

Skills can declare which targets they should sync to via `metadata.targets` in SKILL.md. Top-level `targets` is still supported for older skills, but `metadata.targets` wins when both are present:

```yaml
---
name: enterprise-skill
metadata:
  targets: [claude, cursor]
---
```

- Skills **without** `targets` sync to all targets (backward compatible)
- `check` warns about unknown target names in the `targets` field
- Works with both global and project mode target names

## Sync Modes

Per-target mode (both global and project):

```bash
skillshare target claude --mode merge         # Per-skill symlinks (default)
skillshare target claude --mode copy          # Real-file copies with manifest tracking
skillshare target claude --mode symlink       # Entire dir symlinked
skillshare target claude --mode copy -p       # Project target mode
```

| Mode | Description | Local Skills |
|------|-------------|--------------|
| `merge` | Individual symlinks per skill (default) | Preserved |
| `copy` | Real-file copies with `.skillshare-manifest.json` | Preserved |
| `symlink` | Single symlink for entire dir | Not possible |

Copy mode is useful for AI CLIs that can't follow symlinks. Use `sync --force` to re-copy all files.

## Unified Target Names

Global and project modes use the **same short names** (e.g., `claude`, `cursor`, `windsurf`). Old project-only names (e.g., `claude-code`) are supported as aliases for backward compatibility.

## Safety

**Always use** `target remove` to unlink targets.

Inspect whether a path is a symlink before filesystem operations. Removing a symlink
itself and traversing its target are different operations; `target remove` handles the
managed relationship without requiring manual recursive deletion.
