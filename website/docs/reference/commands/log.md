---
sidebar_position: 4
---

# log

View persistent operations and audit logs for debugging and compliance.

```bash
skillshare log                    # Interactive TUI (default on TTY)
skillshare log --audit            # Show only audit log
skillshare log --tail 50          # Show last 50 entries
skillshare log --cmd sync         # Show only sync entries
skillshare log --status error     # Show only errors
skillshare log --since 2d         # Entries from last 2 days
skillshare log --stats            # Show summary statistics
skillshare log --json             # Output as JSONL
skillshare log --no-tui           # Plain text output
skillshare log --clear            # Clear operations log
skillshare log -p                 # Project operations + audit logs
```

## When to Use

- Browse and filter log entries interactively
- Debug what happened during a failed operation
- Review the audit trail for compliance or troubleshooting
- Filter logs by command, status, or time range for investigation

## Interactive TUI

On a TTY, `skillshare log` opens an interactive list of operations with the details of the selected entry beside it, a filter, and a stats view broken down by command. Operations and audit entries are merged and sorted by time. The keys are listed at the bottom of the screen.

Use `--no-tui` to skip the TUI and print plain text instead:

```bash
skillshare log --no-tui           # Plain text output
skillshare log --no-tui | less    # Pipe to pager manually
```

## What Gets Logged

Every mutating CLI and Web UI operation is recorded as a JSONL entry with timestamp, command, status, duration, and contextual args.

| Command | Log File |
|---------|----------|
| `install`, `uninstall`, `sync`, `push`, `pull`, `collect`, `backup`, `restore`, `update`, `target`, `trash`, `config`, `check`, `diff`, `init`, `upgrade` | `operations.log` |
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

## Example Output (Plain Text)

With `--no-tui` in a terminal (when piped, each entry's details are joined on one line):

```
Operations · last 2
✓ sync      2026-02-10 14:35 · 0.3s
            targets: 3
            scope: global

✗ sync      2026-02-10 14:31 · 0.8s · error
            targets: 3
            failed: 1
            scope: global
  ~/.local/state/skillshare/logs/operations.log

Audit · last 1
✗ audit     2026-02-10 14:36 · 1.1s · blocked
            scope: all-skills
            scanned: 12
            passed: 11
            failed: 1
            failed skills:
              - prompt-injection-skill
              - data-exfil-skill
  ~/.local/state/skillshare/logs/audit.log
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
~/.local/state/skillshare/logs/operations.log    # Global operations
~/.local/state/skillshare/logs/audit.log         # Global audit
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
| `--stats` | Show summary statistics (total, success rate, per-command breakdown) |
| `--json` | Output raw JSONL (one JSON object per line) |
| `--no-tui` | Disable interactive TUI, use plain text output |
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

## Stats View

### CLI

```bash
skillshare log --stats                # Summary of all operations
skillshare log --stats --cmd sync     # Stats for sync only
skillshare log --stats --since 7d     # Stats for last 7 days
```

## Log Retention

Logs are automatically truncated to prevent unbounded growth. The default limit is **1000 entries per log file** (each CLI or Web UI operation = 1 entry). `operations.log` and `audit.log` are tracked separately.

To override the default, add to `config.yaml`:

```yaml
log:
  max_entries: 500  # entries per file; 0 = unlimited (default: 1000)
```

## See Also

- [audit](/docs/reference/commands/audit) — Security scanning (logged to audit.log)
- [status](/docs/reference/commands/status) — Show current sync state
- [doctor](/docs/reference/commands/doctor) — Diagnose setup issues
