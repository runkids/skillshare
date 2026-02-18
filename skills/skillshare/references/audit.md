# Security Audit

Scan skills for prompt injection, data exfiltration, credential access, destructive commands, obfuscation, and suspicious URLs.

## Usage

```bash
skillshare audit                   # Scan all skills
skillshare audit <name>            # Scan specific skill
skillshare audit <path>            # Scan file or directory path
skillshare audit -p                # Scan project skills
```

## Flags

| Flag | Description |
|------|-------------|
| `-p, --project` | Scan project-level skills |
| `-g, --global` | Scan global skills |
| `--threshold <t>` | Block threshold override: `critical\|high\|medium\|low\|info` |
| `--json` | Machine-readable JSON output with risk scores |
| `--init-rules` | Create a starter `audit-rules.yaml` |
| `-h, --help` | Show help |

## Severity Levels

| Level | Meaning | Default install behavior |
|-------|---------|--------------------------|
| **CRITICAL** | Prompt injection, data exfil, credential theft | **Blocked** (use `--force` to override) |
| **HIGH** | Destructive commands, hidden unicode, obfuscation | Warning shown |
| **MEDIUM** | Suspicious URLs, system path writes | Warning shown |
| **LOW** | Minor concerns, uncommon patterns | Warning shown |
| **INFO** | Informational observations | Warning shown |

## Configurable Block Threshold

The default threshold is `CRITICAL` — only CRITICAL findings block `install`. Override per-command or globally:

```bash
# Per-command override
skillshare audit --threshold high          # Block on HIGH+ findings

# Global config (config.yaml)
audit:
  block_threshold: HIGH                    # Block on HIGH+ for all installs
```

## Install Integration

`skillshare install` auto-scans after download:

- **Findings at/above threshold → install blocked.** User must `--force` to proceed.
- **Findings below threshold → warning displayed** after successful install.
- **`--skip-audit`** skips security scanning entirely for a single install.
- Web UI shows a confirm dialog on blocked findings with "Force Install" option.

```bash
skillshare install user/repo              # Auto-audit, block on threshold
skillshare install user/repo --force      # Override block
skillshare install user/repo --skip-audit # Skip audit entirely
```

## Output

Per-skill results with risk scoring:

```
[1/5] ✓ my-skill 0.1s
[2/5] ! risky-skill 0.1s  (MODERATE 35/100)
[3/5] ✗ bad-skill 0.1s  (SEVERE 85/100)
```

Single-skill scan shows detailed findings:

```
  MEDIUM: URL used in command context (scripts/run.sh:14)
  HIGH: base64 decode pipe may hide malicious content (SKILL.md:42)
  Risk: MODERATE (35/100)
```

Summary box:

```
┌─ Summary ───────────────────────┐
│  Threshold: CRITICAL            │
│  Scanned:   5 skill(s)         │
│  Passed:    3                   │
│  Warning:   1                   │
│  Failed:    1                   │
│  Severity:  c/h/m/l/i = 1/1/1/0/0 │
│  Risk:      SEVERE (85/100)     │
└─────────────────────────────────┘
```

`Failed` counts skills with findings at/above the threshold. `Warning` counts skills with findings below the threshold.

## Built-In Detection Patterns

| Pattern | Severity | Detects | False-Positive Guards |
|---------|----------|---------|----------------------|
| `prompt-injection` | CRITICAL | Direct prompt override attempts | — |
| `data-exfiltration` | CRITICAL | Piping sensitive data to network | — |
| `credential-access` | CRITICAL | Reads from ~/.ssh, ~/.aws, etc. | — |
| `destructive-commands` | HIGH | rm -rf, mkfs, disk wipe | — |
| `dynamic-code-exec` | HIGH | Dynamic code evaluation calls | Excludes evaluate(), execFile() |
| `shell-execution` | HIGH | Python shell invocation via stdlib | — |
| `hidden-comment-injection` | HIGH | Prompt injection in HTML comments | — |
| `obfuscation` | HIGH | Hidden unicode, long base64 strings | — |
| `env-access` | MEDIUM | Environment variable references | Excludes NODE_ENV, npm_* |
| `escape-obfuscation` | MEDIUM | 3+ consecutive hex/unicode escapes | — |
| `suspicious-fetch` | MEDIUM | URLs used in command context | — |
| `insecure-http` | MEDIUM | HTTP URLs (non-HTTPS) | — |
| `system-writes` | MEDIUM | Writes to /etc, /usr, system paths | — |
| `shell-chain` | MEDIUM | Long shell pipe chains | — |

## Custom Audit Rules

Create custom rules to extend or override built-in patterns.

### Init

```bash
skillshare audit --init-rules       # Global: ~/.config/skillshare/audit-rules.yaml
skillshare audit --init-rules -p    # Project: .skillshare/audit-rules.yaml
```

### Three-Layer Merge

Rules merge in order (later overrides earlier):

1. **Built-in** — shipped with skillshare binary
2. **Global** — `~/.config/skillshare/audit-rules.yaml`
3. **Project** — `.skillshare/audit-rules.yaml` (project mode only)

### Rule Format

```yaml
rules:
  # Add a new rule
  - id: flag-todo
    severity: MEDIUM
    pattern: todo-comment
    message: "TODO comment found"
    regex: '(?i)\bTODO\b'

  # Disable a built-in rule
  - id: system-writes-0
    enabled: false

  # Override severity of a built-in rule
  - id: destructive-commands-2
    severity: LOW
```

## Logging

Audit results are logged to `audit.log` (JSONL). View with:

```bash
skillshare log --audit
```
