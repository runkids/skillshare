# Target Management

## Overview

Targets are AI CLI skill directories that skillshare syncs to.

## Supported Agents (35)

```text
┌────────────────────────────────────────────────────────────────────────────┐
│                           SUPPORTED TARGETS                                │
│                                                                            │
│  Built-in (auto-detected):                                                 │
│  ┌─────────┐ ┌─────────┐ ┌───────────┐ ┌─────────┐ ┌─────────┐             │
│  │ Agents  │ │   Amp   │ │Antigravity│ │ Claude  │ │  Cline  │             │
│  └─────────┘ └─────────┘ └───────────┘ └─────────┘ └─────────┘             │
│  ┌──────────┐ ┌─────────┐ ┌─────────┐ ┌──────────┐ ┌─────────┐             │
│  │CodeBuddy │ │  Codex  │ │Cmd Code │ │ Continue │ │ Copilot │             │
│  └──────────┘ └─────────┘ └─────────┘ └──────────┘ └─────────┘             │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐               │
│  │  Crush  │ │ Cursor  │ │  Droid  │ │ Gemini  │ │  Goose  │               │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘               │
│  ┌─────────┐ ┌──────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐              │
│  │  Junie  │ │ Kilocode │ │  Kiro   │ │  Kode   │ │  Letta  │              │
│  └─────────┘ └──────────┘ └─────────┘ └─────────┘ └─────────┘              │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌──────────┐ ┌──────────┐             │
│  │ MCPJam  │ │ Moltbot │ │   Mux   │ │ Neovate  │ │ OpenCode │             │
│  └─────────┘ └─────────┘ └─────────┘ └──────────┘ └──────────┘             │
│  ┌───────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐             │
│  │ OpenHands │ │   Pi    │ │  Pochi  │ │  Qoder  │ │  Qwen   │             │
│  └───────────┘ └─────────┘ └─────────┘ └─────────┘ └─────────┘             │
│  ┌─────────┐ ┌─────────┐ ┌──────────┐ ┌──────────┐                         │
│  │   Roo   │ │  Trae   │ │ Windsurf │ │ Zencoder │                         │
│  └─────────┘ └─────────┘ └──────────┘ └──────────┘                         │
│                                                                            │
│  Custom (add your own):                                                    │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐                                       │
│  │  Aider  │ │   ...   │ │         │                                       │
│  └─────────┘ └─────────┘ └─────────┘                                       │
└────────────────────────────────────────────────────────────────────────────┘
```

Run `skillshare target <name>` to see the exact path for any agent.

---

## Commands

| Command | Description |
|---------|-------------|
| `target list` | List all configured targets |
| `target <name>` | Show target details |
| `target <name> --mode <mode>` | Change sync mode |
| `target add <name> <path>` | Add custom target |
| `target remove <name>` | Safely unlink target |

---

## List Targets

```bash
skillshare target list
```

### Example Output

```
Targets
─────────────────────────────────────────
  claude    merge     ~/.claude/skills         5 skills
  cursor    merge     ~/.cursor/skills         5 skills
  codex     symlink   ~/.codex/skills          linked
  myapp     merge     ~/.myapp/skills          3 skills
```

---

## Show Target Details

```bash
skillshare target claude
```

### Example Output

```
Target: claude
─────────────────────────────────────────
  Path:     ~/.claude/skills
  Mode:     merge
  Status:   synced (5/5 skills)

  Skills:
    my-skill        → source/my-skill (symlink)
    another         → source/another (symlink)
    local-only      (local, not synced)
```

---

## Add Custom Target

Add any tool with a skills directory.

```bash
skillshare target add <name> <path>
```

### Examples

```bash
# Add Aider
skillshare target add aider ~/.aider/skills

# Add custom app
skillshare target add myapp ~/apps/myapp/skills

# Then sync
skillshare sync
```

### Requirements

- Path must exist (create with `mkdir -p` first)
- Path should end with `/skills` (recommended)

```bash
# If path doesn't exist:
mkdir -p ~/.myapp/skills
skillshare target add myapp ~/.myapp/skills
```

---

## Change Sync Mode

```bash
skillshare target <name> --mode <mode>
```

### Modes

| Mode | Behavior |
|------|----------|
| `merge` | Each skill symlinked individually. Local skills preserved. **(default)** |
| `symlink` | Entire directory is one symlink. |

### Examples

```bash
# Switch to symlink mode
skillshare target codex --mode symlink
skillshare sync

# Switch back to merge mode
skillshare target codex --mode merge
skillshare sync
```

### Visual Comparison

**Merge mode:**
```
~/.claude/skills/
├── my-skill/     → ~/.config/skillshare/skills/my-skill/ (symlink)
├── another/      → ~/.config/skillshare/skills/another/  (symlink)
└── local-only/   (local file, preserved)
```

**Symlink mode:**
```
~/.claude/skills  → ~/.config/skillshare/skills/ (entire dir is symlink)
```

---

## Remove Target

Safely unlink a target (preserves your files).

```bash
skillshare target remove <name>
skillshare target remove <name> --dry-run    # Preview
skillshare target remove --all               # Remove all
```

### What Happens

```
┌─────────────────────────────────────────────────────────────────┐
│ skillshare target remove claude                                 │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. Create backup                                                │
│    → ~/.config/skillshare/backups/2026-01-20.../claude/         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Detect mode                                                  │
└─────────────────────────────────────────────────────────────────┘
                    │                   │
           merge mode                   symlink mode
                    │                   │
                    ▼                   ▼
┌───────────────────────────┐   ┌───────────────────────────┐
│ For each symlink:         │   │ Remove symlink:           │
│  • Remove symlink         │   │  ~/.claude/skills →       │
│  • Copy source file back  │   │                           │
│                           │   │ Copy source contents:     │
│ Local files: preserved    │   │  → ~/.claude/skills/      │
└───────────────────────────┘   └───────────────────────────┘
                    │                   │
                    └─────────┬─────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Remove from config.yaml                                      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ Result:                                                         │
│  • ~/.claude/skills/ now contains real files (not symlinks)     │
│  • skillshare no longer manages this target                     │
│  • Backup available if needed                                   │
└─────────────────────────────────────────────────────────────────┘
```

### Why This Is Safe

| Operation | Danger | Safety |
|-----------|--------|--------|
| `rm -rf ~/.claude/skills` | **Deletes source files!** | ❌ |
| `skillshare target remove` | Backs up, copies files back | ✅ |

---

## Common Scenarios

### Add a new AI CLI tool

```bash
# 1. Create the skills directory
mkdir -p ~/.newtool/skills

# 2. Add as target
skillshare target add newtool ~/.newtool/skills

# 3. Sync skills
skillshare sync

# 4. Verify
skillshare target newtool
```

### Stop managing a target temporarily

```bash
# Remove (keeps your files)
skillshare target remove cursor

# Re-add later
skillshare target add cursor ~/.cursor/skills
skillshare sync
```

### Switch all targets to symlink mode

```bash
for target in claude cursor codex; do
  skillshare target $target --mode symlink
done
skillshare sync
```

---

## Related

- [sync.md](sync.md) — Sync operations
- [configuration.md](configuration.md) — Config file reference
- [faq.md](faq.md) — Troubleshooting
