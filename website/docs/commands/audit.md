---
sidebar_position: 3
---

# audit

Scan installed skills for security threats and malicious patterns.

```bash
skillshare audit                        # Scan all installed skills
skillshare audit <name>                 # Scan a specific installed skill
skillshare audit <path>                 # Scan a file/directory path
skillshare audit --threshold high       # Block on HIGH+ findings
skillshare audit --json                 # JSON output
skillshare audit -p                     # Scan project skills
```

## What It Detects

### CRITICAL (blocks installation and counted as Failed)

| Pattern | Description |
|---------|------------|
| `prompt-injection` | "Ignore previous instructions", "SYSTEM:", "You are now", etc. |
| `data-exfiltration` | `curl`/`wget` commands sending environment variables externally |
| `credential-access` | Reading `~/.ssh/`, `.env`, `~/.aws/credentials` |

### HIGH (strong warning, counted as Warning)

| Pattern | Description |
|---------|------------|
| `hidden-unicode` | Zero-width characters that hide content from human review |
| `destructive-commands` | `rm -rf /`, `chmod 777`, `sudo`, `dd if=`, `mkfs` |
| `obfuscation` | Base64 decode pipes, long base64-encoded strings |

### MEDIUM (informational warning, counted as Warning)

| Pattern | Description |
|---------|------------|
| `suspicious-fetch` | URLs used in command context (`curl`, `wget`, `fetch`) |
| `system-writes` | Commands writing to `/usr`, `/etc`, `/var` |

### LOW / INFO (non-blocking signal by default)

These are lower-severity indicators that contribute to risk scoring and reporting:

- `LOW`: weaker suspicious patterns that still deserve review
- `INFO`: contextual hints (for triage / visibility)

## Example Output

```
┌─ skillshare audit ──────────────────────────────────────────┐
│  Scanning 12 skills for threats                             │
│  mode: global                                               │
│  path: /Users/alice/.config/skillshare/skills               │
└─────────────────────────────────────────────────────────────┘

[1/12] ✓ react-best-practices         0.1s
[2/12] ✓ typescript-patterns           0.1s
[3/12] ! ci-release-helper             0.2s
       └─ HIGH: Destructive command pattern (SKILL.md:42)
          "sudo apt-get install -y jq"
[4/12] ✗ suspicious-skill              0.2s
       ├─ CRITICAL: Prompt injection (SKILL.md:15)
       │  "Ignore all previous instructions and..."
       └─ HIGH: Destructive command (SKILL.md:42)
          "rm -rf / # clean up"
[5/12] ! frontend-utils                0.1s
       └─ MEDIUM: URL in command context (SKILL.md:3)

┌─ Summary ────────────────────────────┐
│  Scanned:  12 skills                 │
│  Passed:   9                         │
│  Warning:  2 (1 high, 1 medium)      │
│  Failed:   1 (1 critical)            │
└──────────────────────────────────────┘
```

`Failed` counts skills with findings at or above the active threshold (`--threshold` or config `audit.block_threshold`; default `CRITICAL`).

`audit.block_threshold` only controls the blocking threshold. It does **not** disable scanning.

## Install-time Scanning

Skills are automatically scanned during installation. Findings at or above `audit.block_threshold` block installation (default: `CRITICAL`):

```bash
skillshare install /path/to/evil-skill
# Error: security audit failed: critical threats detected in skill

skillshare install /path/to/evil-skill --force
# Installs with warnings (use with caution)

skillshare install /path/to/skill --skip-audit
# Bypasses scanning (use with caution)
```

`--force` overrides block decisions. `--skip-audit` disables scanning for that install command.

There is no config flag to globally disable install-time audit. Use `--skip-audit` only for commands where you intentionally want to bypass scanning.

Difference summary:

| Install flag | Audit runs? | Findings available? |
|--------------|-------------|---------------------|
| `--force` | Yes | Yes (installation still proceeds) |
| `--skip-audit` | No | No (scan is bypassed) |

If both are provided, `--skip-audit` effectively wins because audit is not executed.

## Web UI

The audit feature is also available in the web dashboard at `/audit`:

```bash
skillshare ui
# Navigate to Audit page → Click "Run Audit"
```

![Security Audit page in web dashboard](/img/web-audit-demo.png)

The Dashboard page includes a Security Audit section with a quick-scan summary.

### Custom Rules Editor

The web dashboard includes a dedicated **Audit Rules** page at `/audit/rules` for creating and editing custom rules directly in the browser:

- **Create**: If no `audit-rules.yaml` exists, click "Create Rules File" to scaffold one
- **Edit**: YAML editor with syntax highlighting and validation
- **Save**: Validates YAML format and regex patterns before saving

Access it from the Audit page via the "Custom Rules" button.

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | No findings at or above active threshold |
| `1` | One or more findings at or above active threshold |

## Scanned Files

The audit scans text-based files in skill directories:

- `.md`, `.txt`, `.yaml`, `.yml`, `.json`, `.toml`
- `.sh`, `.bash`, `.zsh`, `.fish`
- `.py`, `.js`, `.ts`, `.rb`, `.go`, `.rs`
- Files without extensions (e.g., `Makefile`, `Dockerfile`)

Scanning is recursive within each skill directory, so `SKILL.md`, nested `references/*.md`, and `scripts/*.sh` are all inspected when they match supported text file types.

Binary files (images, `.wasm`, etc.) and hidden directories (`.git`) are skipped.

## Custom Rules

You can add, override, or disable audit rules using YAML files. Rules are merged in order: **built-in → global user → project user**.

Use `--init-rules` to create a starter file with commented examples:

```bash
skillshare audit --init-rules         # Create global rules file
skillshare audit -p --init-rules      # Create project rules file
```

### File Locations

| Scope | Path |
|-------|------|
| Global | `~/.config/skillshare/audit-rules.yaml` |
| Project | `.skillshare/audit-rules.yaml` |

### Format

```yaml
rules:
  # Add a new rule
  - id: my-custom-rule
    severity: HIGH
    pattern: custom-check
    message: "Custom pattern detected"
    regex: 'DANGEROUS_PATTERN'

  # Add a rule with an exclude (suppress matches on certain lines)
  - id: url-check
    severity: MEDIUM
    pattern: url-usage
    message: "External URL detected"
    regex: 'https?://\S+'
    exclude: 'https?://(localhost|127\.0\.0\.1)'

  # Override an existing built-in rule (match by id)
  - id: destructive-commands-2
    severity: MEDIUM
    pattern: destructive-commands
    message: "Sudo usage (downgraded to MEDIUM)"
    regex: '(?i)\bsudo\s+'

  # Disable a built-in rule
  - id: system-writes-0
    enabled: false
```

### Fields

| Field | Required | Description |
|-------|----------|-------------|
| `id` | Yes | Stable identifier. Matching IDs override built-in rules. |
| `severity` | Yes* | `CRITICAL`, `HIGH`, `MEDIUM`, `LOW`, or `INFO` |
| `pattern` | Yes* | Rule category name (e.g., `prompt-injection`) |
| `message` | Yes* | Human-readable description shown in findings |
| `regex` | Yes* | Regular expression to match against each line |
| `exclude` | No | If a line matches both `regex` and `exclude`, the finding is suppressed |
| `enabled` | No | Set to `false` to disable a rule. Only `id` is required when disabling. |

*Required unless `enabled: false`.

### Merge Semantics

Each layer (global, then project) is applied on top of the previous:

- **Same `id`** + `enabled: false` → disables the rule
- **Same `id`** + other fields → replaces the entire rule
- **New `id`** → appends as a custom rule

### Practical Templates

Use this as a starting point for real-world policy tuning:

```yaml
rules:
  # Team policy: detect obvious hardcoded API tokens
  - id: hardcoded-token-policy
    severity: HIGH
    pattern: hardcoded-token
    message: "Potential hardcoded token detected"
    regex: '(?i)\b(ghp_[A-Za-z0-9]{20,}|sk-[A-Za-z0-9]{20,})\b'

  # Override built-in suspicious-fetch with internal allowlist
  - id: suspicious-fetch-0
    severity: MEDIUM
    pattern: suspicious-fetch
    message: "External URL used in command context"
    regex: '(?i)(curl|wget|invoke-webrequest|iwr)\s+https?://'
    exclude: '(?i)https?://(localhost|127\.0\.0\.1|artifacts\.company\.internal|registry\.company\.internal)'

  # Governance exception: disable noisy path-write signal in your environment
  - id: system-writes-0
    enabled: false
```

### Validate Changes

Typical workflow:

```bash
# 1) Create starter file (once)
skillshare audit --init-rules

# 2) Edit ~/.config/skillshare/audit-rules.yaml

# 3) Re-run audit to verify expected classification changes
skillshare audit
```

Summary interpretation:

- `Failed` counts skills with findings at or above the active threshold.
- `Warning` counts skills with findings below threshold but above clean (for example `HIGH/MEDIUM/LOW/INFO` when threshold is `CRITICAL`).

### Built-in Rule IDs

Use `id` values to override or disable specific built-in rules:

Source of truth (full built-in definitions):
[`internal/audit/rules.yaml`](https://github.com/runkids/skillshare/blob/main/internal/audit/rules.yaml)

| ID | Pattern | Severity |
|----|---------|----------|
| `prompt-injection-0` | prompt-injection | CRITICAL |
| `prompt-injection-1` | prompt-injection | CRITICAL |
| `data-exfiltration-0` | data-exfiltration | CRITICAL |
| `data-exfiltration-1` | data-exfiltration | CRITICAL |
| `credential-access-0` | credential-access | CRITICAL |
| `credential-access-1` | credential-access | CRITICAL |
| `credential-access-2` | credential-access | CRITICAL |
| `hidden-unicode-0` | hidden-unicode | HIGH |
| `destructive-commands-0` | destructive-commands | HIGH |
| `destructive-commands-1` | destructive-commands | HIGH |
| `destructive-commands-2` | destructive-commands | HIGH |
| `destructive-commands-3` | destructive-commands | HIGH |
| `destructive-commands-4` | destructive-commands | HIGH |
| `obfuscation-0` | obfuscation | HIGH |
| `obfuscation-1` | obfuscation | HIGH |
| `suspicious-fetch-0` | suspicious-fetch | MEDIUM |
| `system-writes-0` | system-writes | MEDIUM |

## Options

| Flag | Description |
|------|------------|
| `-p`, `--project` | Scan project-level skills |
| `-g`, `--global` | Scan global skills |
| `--init-rules` | Create a starter `audit-rules.yaml` (respects `-p`/`-g`) |
| `-h`, `--help` | Show help |

## Related

- [install](/docs/commands/install) — Install skills (with automatic scanning)
- [doctor](/docs/commands/doctor) — Diagnose setup issues
- [list](/docs/commands/list) — List installed skills
