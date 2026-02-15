# Target Management

Manage AI CLI tool targets (Claude, Cursor, Windsurf, etc.). Skillshare supports 46+ built-in targets.

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

## Safety

**Always use** `target remove` to unlink targets.

**NEVER** `rm -rf` on symlinked targets — this deletes the source!
