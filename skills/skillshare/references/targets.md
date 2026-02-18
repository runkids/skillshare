# Target Management

Manage AI CLI tool targets (Claude, Cursor, Windsurf, Firebender, etc.). Skillshare supports 49+ built-in targets.

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

Control which skills sync to each target using include/exclude glob patterns.

```bash
# Add filters
skillshare target claude --add-include "team-*"       # Only sync matching skills
skillshare target claude --add-exclude "_legacy*"     # Skip matching skills
skillshare target claude --add-include "team-*" -p    # Project target filter

# Remove filters
skillshare target claude --remove-include "team-*"
skillshare target claude --remove-exclude "_legacy*"
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

**Precedence:** Include filters apply first (whitelist), then exclude filters remove from that set. No filters = all skills.

## Skill-Level Targets

Skills can declare which targets they should sync to via a `targets` frontmatter field in SKILL.md:

```yaml
---
name: enterprise-skill
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
skillshare target claude --mode symlink       # Entire dir symlinked
skillshare target claude --mode symlink -p   # Project target mode
```

| Mode | Description | Local Skills |
|------|-------------|--------------|
| `merge` | Individual symlinks per skill | Preserved |
| `symlink` | Single symlink for entire dir | Not possible |

## Unified Target Names

Global and project modes use the **same short names** (e.g., `claude`, `cursor`, `windsurf`). Old project-only names (e.g., `claude-code`) are supported as aliases for backward compatibility.

## Safety

**Always use** `target remove` to unlink targets.

**NEVER** `rm -rf` on symlinked targets — this deletes the source!
