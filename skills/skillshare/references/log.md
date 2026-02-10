# Operation Log

JSONL-based persistent audit trail. All mutating commands (sync, install, uninstall, audit, etc.) are logged automatically.

## Usage

```bash
skillshare log                    # Show operations + audit logs
skillshare log --audit            # Audit log only
skillshare log --tail 50          # Last 50 entries per section
skillshare log --cmd sync         # Show only sync entries
skillshare log --status error     # Show only errors
skillshare log --since 2d         # Entries from last 2 days
skillshare log --json             # Output as JSONL
skillshare log --clear            # Clear operations log
skillshare log --clear --audit    # Clear audit log
skillshare log -p                 # Project logs
```

## Flags

| Flag | Description |
|------|-------------|
| `--audit, -a` | Show audit log only |
| `--tail, -t <N>` | Last N entries (default: 20) |
| `--cmd <name>` | Filter by command name (e.g. sync, install, audit) |
| `--status <status>` | Filter by status (ok, error, partial, blocked) |
| `--since <dur\|date>` | Filter by time (30m, 2h, 2d, 1w, or 2006-01-02) |
| `--json` | Output raw JSONL |
| `--clear, -c` | Clear selected log file |
| `-p, --project` | Project-level logs |
| `-g, --global` | Global logs |

When `--cmd` targets a command that only exists in one log file (e.g. `--cmd audit`), the other section is automatically skipped.

## Log Files

| File | Contents |
|------|----------|
| `operations.log` | All CLI commands (sync, install, update, etc.) |
| `audit.log` | Security scan results |

Location: `~/.config/skillshare/logs/` (global) or `.skillshare/logs/` (project).

## Output Format

```
  TIME             | CMD       | STATUS  | DUR
  -----------------+-----------+---------+--------
  2026-02-10 14:30 | SYNC      | ok      | 120ms
  targets: 3
  scope: global
```

Fields: timestamp, command, status, duration, then key-value detail lines.

## Status Colors (TTY)

- **Green**: ok
- **Red**: error, blocked
- **Yellow**: partial
