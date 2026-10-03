# Init Command

Initialize skillshare configuration (global or project).

## Global Init

**Source:** `~/.config/skillshare/skills` by default. Respects `$XDG_CONFIG_HOME` — if set, uses `$XDG_CONFIG_HOME/skillshare/skills`. Use `--source` only if user explicitly requests a custom path.

### Flags

| Flag | Description |
|------|-------------|
| `-c, --copy-from <name\|path>` | Import skills from target/path |
| `--no-copy` | Start with empty source |
| `-t, --targets "claude,cursor"` | Specific targets |
| `--all-targets` | All detected targets |
| `--no-targets` | Skip target setup |
| `--git` | Initialize git repo |
| `--no-git` | Skip git init |
| `-d, --discover` | Discover new AI tools (no terminal: adds every new tool) |
| `--discover --select "a,b"` | Non-interactive discovery |
| `-s, --source <path>` | Custom source path |
| `--remote <url>` | Set git remote (implies --git); pulls a repo that has skills, repo version wins for same-name skills |
| `--git-root <scope>` | Git scope: `skills`, `agents`, `extras`, or `root` |
| `--skill` | Install built-in skillshare skill (default) |
| `--no-skill` | Skip built-in skill installation |
| `-n, --dry-run` | Preview changes |

### AI Usage (Non-Interactive)

Without a terminal, `init` asks nothing and uses the defaults: every detected tool, every existing skill imported, git on, built-in skill installed, then a first sync. Each decision is printed with the flag that changes it.

```bash
# Defaults: a working setup in one command
skillshare init

# New machine with an existing skillshare repo (pulls its skills)
skillshare init --remote git@github.com:you/skills.git

# Start empty instead of importing
skillshare init --no-copy

# Verify
skillshare status
```

### Adding New Targets Later

```bash
skillshare init --discover --select "windsurf,kilocode"
```

---

## Project Init (`-p`)

Creates `.skillshare/` in the current directory. `--visible` uses `skillshare/` instead.
Examples below use the default hidden directory.

### Flags

| Flag | Description |
|------|-------------|
| `-p, --project` | Enable project mode |
| `--visible` | Use `skillshare/` instead of `.skillshare/` |
| `-t, --targets "claude,cursor"` | Specific targets (default: every detected tool) |
| `-d, --discover` | Discover new AI tools |
| `--discover --select "a,b"` | Non-interactive discovery |
| `-n, --dry-run` | Preview changes |

**Note:** `--copy-from`, `--git`, `--source` are not available in project mode.

### AI Usage (Non-Interactive)

```bash
# Every detected tool
skillshare init -p

# Or specific targets
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
