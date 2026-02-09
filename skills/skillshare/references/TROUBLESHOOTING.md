# Troubleshooting

## Quick Fixes

| Problem | Solution |
|---------|----------|
| "config not found" | `skillshare init` (global) or `skillshare init -p` (project) |
| Target shows differences | `skillshare sync` |
| Sync drift warning | `skillshare sync` to re-link missing skills |
| Lost source files (global) | `cd ~/.config/skillshare/skills && git checkout -- .` |
| Lost source files (project) | `git checkout -- .skillshare/skills/` |
| Accidentally uninstalled | `skillshare trash restore <name>` (within 7 days) |
| Skill not appearing | `skillshare sync` after install |
| Install blocked by audit | `skillshare install ... --force` to override CRITICAL |
| Git push fails | Check remote: `git -C ~/.config/skillshare/skills remote -v` |
| Project mode not detected | Verify `.skillshare/config.yaml` exists in cwd |
| Wrong mode detected | Use `-p` (project) or `-g` (global) to force |

## Diagnostic Commands

```bash
skillshare doctor          # Check environment + sync drift
skillshare status          # Overview (auto-detects mode)
skillshare diff            # Show differences
skillshare log             # Recent operations and audit log
skillshare log --audit     # Security scan history
ls -la ~/.claude/skills    # Check global symlinks
ls -la .claude/skills      # Check project symlinks
```

## Recovery

```bash
skillshare backup          # Safety backup first (global only)
skillshare sync --dry-run  # Preview changes
skillshare sync            # Apply fix
```

## Git Recovery (Global)

```bash
cd ~/.config/skillshare/skills
git status                 # Check state
git checkout -- <skill>/   # Restore specific skill
git checkout -- .          # Restore all skills
```

## Project Recovery

```bash
# Re-install remote skills from config
skillshare install -p

# Re-sync to targets
skillshare sync
```

## AI Assistant Notes

### Symlink Safety

- **merge mode** (default): Per-skill symlinks. Edit anywhere = edit source.
- **symlink mode**: Entire directory symlinked.

Both modes apply to global and project targets.

**Safe commands:** `skillshare uninstall`, `skillshare target remove`

**DANGEROUS:** `rm -rf` on symlinked skills deletes source!

### Non-Interactive Usage

AI cannot respond to CLI prompts. Always use flags:

```bash
# Good (non-interactive)
skillshare init --copy-from claude --all-targets --git
skillshare init -p --targets "claude-code,cursor"
skillshare uninstall my-skill --force
skillshare uninstall my-skill --force -p

# Bad (requires user input)
skillshare init
skillshare uninstall my-skill
```

### When to Use --dry-run

- First-time operations
- Before `sync`, `collect --all`, `restore`
- Before `install` from unknown sources
