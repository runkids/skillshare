# Init Command

Initialize skillshare configuration (global or project).

## Global Init

**Source:** `~/.config/skillshare/skills` by default. Respects `$XDG_CONFIG_HOME` — if set, uses `$XDG_CONFIG_HOME/skillshare/skills`. Use `--source` only if user explicitly requests a custom path.

### Flags

| Flag | Description |
|------|-------------|
| `--copy-from <name\|path>` | Import skills from target/path |
| `--no-copy` | Start with empty source |
| `--targets "claude,cursor"` | Specific targets |
| `--all-targets` | All detected targets |
| `--no-targets` | Skip target setup |
| `--git` | Initialize git repo |
| `--no-git` | Skip git init |
| `--discover` | Discover new AI tools (interactive) |
| `--discover --select "a,b"` | Non-interactive discovery |
| `--source <path>` | Custom source path |
| `--remote <url>` | Set git remote |
| `--skill` | Install built-in skillshare skill (opt-in) |
| `--no-skill` | Skip built-in skill installation |
| `--dry-run` | Preview changes |

### AI Usage (Non-Interactive)

```bash
# Step 1: Check for existing skills
ls ~/.claude/skills ~/.cursor/skills 2>/dev/null | head -10

# Step 2a: Fresh start
skillshare init --no-copy --all-targets --git --skill

# Step 2b: Import existing skills
skillshare init --copy-from claude --all-targets --git --skill

# Step 3: Verify
skillshare status
```

### Adding New Targets Later

```bash
skillshare init --discover --select "windsurf,kilocode"
```

---

## Project Init (`-p`)

Creates `.skillshare/` in current directory with `config.yaml`, `.gitignore`, and `skills/`.

### Flags

| Flag | Description |
|------|-------------|
| `-p, --project` | Enable project mode |
| `--targets "claude,cursor"` | Specific targets (non-interactive) |
| `--discover` | Discover new AI tools |
| `--discover --select "a,b"` | Non-interactive discovery |
| `--dry-run, -n` | Preview changes |

**Note:** `--copy-from`, `--git`, `--source` are not available in project mode.

### AI Usage (Non-Interactive)

```bash
# Initialize with specific targets
skillshare init -p --targets "claude,cursor"

# Verify
skillshare status
```

### What It Creates

```
.skillshare/
├── config.yaml       # targets list
├── .gitignore        # ignores cloned repos
└── skills/           # project skill source
```

### Adding Targets to Existing Project

```bash
skillshare init -p --discover --select "windsurf"
```
