---
sidebar_position: 4
---

# log

View persistent operations and audit logs for debugging and compliance.

```bash
skillshare log                  # Show operations + audit sections
skillshare log --audit          # Show only audit log
skillshare log --tail 50        # Show last 50 entries per section
skillshare log --clear          # Clear operations log
skillshare log -p               # Show project operations + audit logs
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

## Example Output

```
┌─ skillshare log ───────────────────────────────┐
  Operations (last 2)
  2026-02-10 14:31  sync   targets=3, failed=1, scope=global            error   0.8s
  2026-02-10 14:35  sync   targets=3, scope=global                       ok      0.3s

┌─ skillshare log ───────────────────────────────┐
  Audit (last 1)
  2026-02-10 14:36  audit  all-skills, scanned=12, passed=11, failed=1   blocked 1.1s
                     -> failed skills: prompt-injection-skill, data-exfil-skill
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

## Options

| Flag | Description |
|------|------------|
| `-a`, `--audit` | Show only audit log |
| `-t`, `--tail <N>` | Show last N entries (default: 20) |
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
- **Table view** with time, command, details, status, and duration
- **Audit detail rows** showing failed/warning skill names when present
- **Clear** and **Refresh** controls

## Related

- [audit](/docs/commands/audit) — Security scanning (logged to audit.log)
- [status](/docs/commands/status) — Show current sync state
- [doctor](/docs/commands/doctor) — Diagnose setup issues
