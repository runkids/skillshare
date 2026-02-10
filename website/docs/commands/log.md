---
sidebar_position: 4
---

# log

View persistent operations and audit logs for debugging and compliance.

```bash
skillshare log                    # Show operations + audit sections
skillshare log --audit            # Show only audit log
skillshare log --tail 50          # Show last 50 entries per section
skillshare log --cmd sync         # Show only sync entries
skillshare log --status error     # Show only errors
skillshare log --since 2d         # Entries from last 2 days
skillshare log --json             # Output as JSONL
skillshare log --clear            # Clear operations log
skillshare log -p                 # Show project operations + audit logs
```

## What Gets Logged

Every mutating CLI and Web UI operation is recorded as a JSONL entry with timestamp, command, status, duration, and contextual args.

| Command | Log File |
|---------|----------|
| `install`, `uninstall`, `sync`, `push`, `pull`, `collect`, `backup`, `restore`, `update`, `target`, `trash`, `config` | `operations.log` |
| `audit` | `audit.log` |

Web UI actions that call these APIs are logged the same way as CLI operations.

## Log Types

### Default View
Shows **both sections** in one output:
- Operations log
- Audit log

```bash
skillshare log
```

### Audit-Only View

Records security audit scans separately from normal operations.

```bash
skillshare log --audit
```

### Filtering

Narrow results by command, status, or time range. When `--cmd` targets a specific log (e.g. `--cmd audit` only appears in audit.log), the irrelevant section is automatically skipped.

```bash
skillshare log --cmd install              # Only install entries
skillshare log --status error             # Only errors
skillshare log --since 1h                 # Last hour (also: 30m, 2d, 1w)
skillshare log --since 2026-01-15         # Since a specific date
skillshare log --cmd sync --status error  # Combine filters
```

### JSON Output

Output raw JSONL for scripting and automation:

```bash
skillshare log --json                     # All entries as JSONL
skillshare log --json --cmd sync          # Filtered JSONL
```

## Example Output

```
┌─ skillshare log ───────────────────────────────┐
│ Operations (last 2)                            │
│ mode: global                                   │
│ file: ~/.config/skillshare/logs/operations.log │
└────────────────────────────────────────────────┘
  TIME             | CMD       | STATUS  | DUR
  -----------------+-----------+---------+--------
  2026-02-10 14:31 | SYNC      | error   | 0.8s
  targets: 3
  failed: 1
  scope: global

  2026-02-10 14:35 | SYNC      | ok      | 0.3s
  targets: 3
  scope: global

┌─ skillshare log ───────────────────────────────┐
│ Audit (last 1)                                 │
│ mode: global                                   │
│ file: ~/.config/skillshare/logs/audit.log      │
└────────────────────────────────────────────────┘
  TIME             | CMD       | STATUS  | DUR
  -----------------+-----------+---------+--------
  2026-02-10 14:36 | AUDIT     | blocked | 1.1s
  scope: all-skills
  scanned: 12
  passed: 11
  failed: 1
  failed skills:
    - prompt-injection-skill
    - data-exfil-skill
```

## Log Format

Entries are stored in JSONL format (one JSON object per line):

```json
{"ts":"2026-02-10T14:30:00Z","cmd":"install","args":{"source":"anthropics/skills/pdf"},"status":"ok","ms":1200}
```

| Field | Description |
|-------|-------------|
| `ts` | ISO 8601 timestamp |
| `cmd` | Command name |
| `args` | Command-specific context (source, name, target, etc.) |
| `status` | `ok`, `error`, `partial`, or `blocked` |
| `msg` | Error message (when status is not ok) |
| `ms` | Duration in milliseconds |

## Log Location

```
~/.config/skillshare/logs/operations.log    # Global operations
~/.config/skillshare/logs/audit.log         # Global audit
<project>/.skillshare/logs/operations.log   # Project operations
<project>/.skillshare/logs/audit.log        # Project audit
```

## Track Logs In Git (Project Mode)

Project mode ignores `.skillshare/logs/` by default to avoid noisy commits.

If your team wants to version log files, add these **user override** rules in `.skillshare/.gitignore` after the managed block:

```gitignore
# User override: track logs
!logs/
!logs/*.log
```

If your repository root `.gitignore` also ignores `.skillshare/`, add matching unignore rules there as well.

## Options

| Flag | Description |
|------|------------|
| `-a`, `--audit` | Show only audit log |
| `-t`, `--tail <N>` | Show last N entries (default: 20) |
| `--cmd <name>` | Filter by command name (e.g. `sync`, `install`, `audit`) |
| `--status <status>` | Filter by status (`ok`, `error`, `partial`, `blocked`) |
| `--since <dur\|date>` | Filter by time (`30m`, `2h`, `2d`, `1w`, or `2006-01-02`) |
| `--json` | Output raw JSONL (one JSON object per line) |
| `-c`, `--clear` | Clear selected log file (operations by default, audit with `--audit`) |
| `-p`, `--project` | Use project-level log |
| `-g`, `--global` | Use global log |
| `-h`, `--help` | Show help |

## Web UI

The log is also available in the web dashboard at `/log`:

```bash
skillshare ui
# Navigate to Log page
```

The Log page provides:
- **Tabs** for `All`, `Operations`, and `Audit`
- **Filters** for command, status, and time range (1h, 24h, 7d, 30d)
- **Table view** with time, command, details, status, and duration
- **Audit detail rows** showing failed/warning skill names when present
- **Clear** and **Refresh** controls

## Related

- [audit](/docs/commands/audit) — Security scanning (logged to audit.log)
- [status](/docs/commands/status) — Show current sync state
- [doctor](/docs/commands/doctor) — Diagnose setup issues
