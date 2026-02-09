# Operation Log

JSONL-based persistent audit trail. All mutating commands (sync, install, uninstall, audit, etc.) are logged automatically.

## Usage

```bash
skillshare log                    # Show operations + audit logs
skillshare log --audit            # Audit log only
skillshare log --tail 50          # Last 50 entries per section
skillshare log --clear            # Clear operations log
skillshare log --clear --audit    # Clear audit log
skillshare log -p                 # Project logs
```

## Flags

| Flag | Description |
|------|-------------|
| `--audit, -a` | Show audit log only |
| `--tail, -t <N>` | Last N entries (default: 20) |
| `--clear, -c` | Clear selected log file |
| `-p, --project` | Project-level logs |
| `-g, --global` | Global logs |

## Log Files

| File | Contents |
|------|----------|
| `operations.log` | All CLI commands (sync, install, update, etc.) |
| `audit.log` | Security scan results |

Location: `~/.config/skillshare/` (global) or `.skillshare/` (project).

## Output Format

```
2026-02-10 14:30:01  sync     targets=3, scope=global       ok     120ms
2026-02-10 14:29:15  install  source=user/repo, name=pdf     ok     2.1s
2026-02-10 14:28:00  audit    skill=*, scanned=5, failed=1   error  340ms
```

Fields: timestamp, command, detail, status, duration.

## Status Colors (TTY)

- **Green**: ok
- **Red**: error, blocked
- **Yellow**: partial
