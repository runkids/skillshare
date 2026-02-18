---
name: skillshare
version: 0.14.0
description: |
  Syncs skills across AI CLI tools (Claude, Cursor, Windsurf, etc.) from a single source of truth.
  Global mode (~/.config/skillshare/) and project mode (.skillshare/ per-repo).
  Commands: status, sync, install, uninstall, update, check, search, new, collect,
  push, pull, diff, list, doctor, audit, init-rules, trash, log, backup, restore, target, ui, upgrade.
  Features: target-level skill filtering (include/exclude), skill-level targets field,
  XDG Base Directory support, fuzzy subdirectory resolution for monorepo installs,
  .skillignore for repo-level skill filtering, --exclude flag, license display,
  multi-skill and group uninstall (--group/-G), declarative skill manifest (global + project),
  group field for organized placement, 49+ supported targets.
  Use when: managing skills across AI tools, "skillshare" CLI, skill sync/install/search,
  project skills setup, target filtering, security audit, web dashboard, or troubleshooting.
argument-hint: "[command] [target] [--dry-run] [-p|-g]"
---

# Skillshare CLI

## Two Modes

```
Global: ~/.config/skillshare/skills → ~/.claude/skills, ~/.cursor/skills, ...
Project: .skillshare/skills/        → .claude/skills, .cursor/skills (per-repo)
```

Auto-detection: commands run in **project mode** when `.skillshare/config.yaml` exists in cwd.
Force with `-p` (project) or `-g` (global).

## Commands

| Category | Commands | Project? |
|----------|----------|:--------:|
| **Inspect** | `status`, `diff`, `list`, `doctor` | ✓ (auto) |
| **Sync** | `sync`, `collect` | ✓ (auto) |
| **Remote** | `push`, `pull` | ✗ (use git) |
| **Skills** | `new`, `install`, `uninstall`, `update`, `check`, `search` | ✓ (`-p`) |
| **Targets** | `target add/remove/list` | ✓ (`-p`) |
| **Security** | `audit [name]` | ✓ (`-p`) |
| **Trash** | `trash list\|restore\|delete\|empty` | ✓ (`-p`) |
| **Log** | `log [--audit] [--tail N]` | ✓ (`-p`) |
| **Backup** | `backup`, `restore` | ✗ |
| **Web UI** | `ui` (`-g` global, `-p` project) | ✓ (`-p`) |
| **Upgrade** | `upgrade [--cli\|--skill]` | — |

**Workflow:** Most commands require `sync` afterward to distribute changes.

## AI Usage Notes

### Non-Interactive Mode

AI cannot respond to CLI prompts. Always pass flags to skip interactive prompts.

```bash
# Key non-interactive patterns
skillshare init --no-copy --all-targets --git --skill   # Global fresh start
skillshare init -p --targets "claude,cursor"       # Project init
skillshare install user/repo --all                      # Install all skills
skillshare install user/repo -s pdf,commit              # Select specific
```

See [init.md](references/init.md) and [install.md](references/install.md) for all flags.

### Safety

- `install` auto-scans skills; **CRITICAL** findings block install (`--force` to override)
- `install` shows license from SKILL.md frontmatter in selection/confirmation prompts
- `uninstall` accepts multiple names and `--group`/`-G` for batch removal; auto-detects group directories
- `uninstall` moves to **trash** (7-day retention) — restore with `trash restore <name>`
- **NEVER** `rm -rf` symlinked skills — deletes source. Use `skillshare uninstall` or `target remove`

See [audit.md](references/audit.md) and [trash.md](references/trash.md) for details.

## References

| Topic | File |
|-------|------|
| Init flags (global + project) | [init.md](references/init.md) |
| Sync/collect/push/pull | [sync.md](references/sync.md) |
| Install/update/uninstall/new | [install.md](references/install.md) |
| Status/diff/list/search/check | [status.md](references/status.md) |
| Security audit | [audit.md](references/audit.md) |
| Trash (soft-delete) | [trash.md](references/trash.md) |
| Operation log | [log.md](references/log.md) |
| Target management | [targets.md](references/targets.md) |
| Backup/restore | [backup.md](references/backup.md) |
| Troubleshooting | [TROUBLESHOOTING.md](references/TROUBLESHOOTING.md) |
