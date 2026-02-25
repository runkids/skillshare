---
sidebar_position: 5
---

# Sync Modes Explained

> A deep dive into the three sync modes — merge, copy, and symlink — when to use each, and the trade-offs.

## The Three Modes

skillshare offers three sync modes that control how skills are delivered from your source directory to AI tool target directories.

### Merge Mode (Default)

```
Source: ~/.config/skillshare/skills/
├── code-review/SKILL.md
├── testing/SKILL.md
└── debugging/SKILL.md

Target: ~/.claude/skills/
├── code-review → ~/.config/skillshare/skills/code-review  (symlink)
├── testing → ~/.config/skillshare/skills/testing           (symlink)
├── debugging → ~/.config/skillshare/skills/debugging       (symlink)
└── my-local-skill/SKILL.md                                 (untouched)
```

**How it works**: Creates one symlink per skill. Each skill directory in the target points back to the source.

**Key property**: **Non-destructive**. Local skills in the target directory (like `my-local-skill` above) are preserved. skillshare only manages symlinks it created.

### Copy Mode

```
Source: ~/.config/skillshare/skills/
├── code-review/SKILL.md
├── testing/SKILL.md
└── debugging/SKILL.md

Target: ~/.cursor/skills/
├── code-review/SKILL.md                (physical copy)
├── testing/SKILL.md                    (physical copy)
├── debugging/SKILL.md                  (physical copy)
├── .skillshare-manifest.json           (tracks managed files)
└── my-local-skill/SKILL.md             (untouched)
```

**How it works**: Physically copies each skill into the target. A `.skillshare-manifest.json` file tracks which skills are managed and their SHA-256 checksums. On subsequent syncs, only changed skills are re-copied.

**Key property**: **Maximum compatibility**. Works everywhere — no symlink support required. Local skills are preserved just like merge mode.

### Symlink Mode

```
Source: ~/.config/skillshare/skills/
├── code-review/SKILL.md
├── testing/SKILL.md
└── debugging/SKILL.md

Target: ~/.claude/skills → ~/.config/skillshare/skills/  (single symlink)
```

**How it works**: Replaces the entire target directory with a single symlink pointing to the source.

**Key property**: **Total control**. The target is exactly the source. No local skills can exist in the target.

## When to Use Each

| Factor | Merge | Copy | Symlink |
|--------|-------|------|---------|
| Preserves local skills | Yes | Yes | No |
| Cross-platform support | May have issues | Works everywhere | May have issues |
| Source changes reflected | Instantly | After `sync` | Instantly |
| Handles nested paths | Flattens (`a/b/c` → `a__b__c`) | Flattens | Native structure |
| Orphan cleanup | Automatic | Automatic | Not needed |
| Disk usage | Minimal (symlinks) | Full copies | Minimal (one symlink) |
| Recommended for | Most users | WSL, Docker, CI | Single-source setups |

### Choose Merge When

- You have local skills in your AI tool that you don't want managed by skillshare
- You use multiple AI tools with different local customizations
- You're adopting skillshare incrementally (some skills managed, some not)

### Choose Copy When

- Your platform has unreliable symlink support (WSL, some Docker setups)
- The AI tool doesn't follow symlinks correctly
- You're in a CI/CD pipeline or containerized environment
- You want the target to work independently of the source directory

### Choose Symlink When

- skillshare is the only source of skills for a target
- You want zero ambiguity about what's in the target
- You're setting up a fresh environment

## Nested Path Handling

In merge and copy modes, nested source paths are flattened using double underscores:

```
Source: skills/frontend/react-patterns/SKILL.md
Target: ~/.claude/skills/frontend__react-patterns → skills/frontend/react-patterns
```

This avoids directory creation in targets that expect a flat skill structure. In symlink mode, the directory structure is preserved as-is.

## Orphan Cleanup

Merge and copy modes automatically remove orphaned entries during `skillshare sync`. If you uninstall a skill from your source, the corresponding symlink (or copied directory) in the target is cleaned up on the next sync.

```bash
skillshare uninstall old-skill
skillshare sync
# → Pruned orphan: old-skill
```

## Per-Target Mode Override

You can set different modes per target. Global config uses map format:

```yaml
targets:
  claude:
    path: ~/.claude/skills
    mode: merge
  cursor:
    path: ~/.cursor/skills
    mode: copy
```

Project config uses list format:

```yaml
targets:
  - name: claude
    mode: merge
  - name: cursor
    mode: copy
```

Or change mode via CLI:

```bash
skillshare target claude --mode copy
```

## Related

- [Sync modes concept page](/docs/understand/sync-modes)
- [`sync` command reference](/docs/reference/commands/sync)
- [Source and targets](/docs/understand/source-and-targets)
