# Target Management

Manage AI CLI tool targets (Claude, Cursor, Windsurf, Firebender, etc.). Use `skillshare target list --json` to inspect configured targets; avoid assuming skill
and MCP client support are identical.

## Global Targets

```bash
skillshare target list                        # List all targets
skillshare target claude                      # Show target info
skillshare target add myapp ~/.myapp/skills   # Add custom target
skillshare target remove myapp                # Remove target (safe)
```

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
