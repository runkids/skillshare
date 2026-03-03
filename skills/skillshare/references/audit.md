# Security Audit

Scan skills for prompt injection, data exfiltration, credential access, destructive commands, obfuscation, suspicious URLs, and broken local links. 100+ built-in rules across 6 analyzers.

## Usage

```bash
skillshare audit                   # Scan all skills
skillshare audit <name>            # Scan specific skill
skillshare audit a b c             # Scan multiple skills
skillshare audit --group frontend  # Scan all skills in a group
skillshare audit <path>            # Scan file or directory path
skillshare audit -T h              # Block on HIGH+ findings
skillshare audit -p                # Scan project skills
skillshare audit --profile strict  # Use strict profile (block HIGH+)
skillshare audit --dedupe global   # Full composite-key deduplication
skillshare audit --analyzer static # Run only the static analyzer
```

## Flags

| Flag | Description |
|------|-------------|
| `-G, --group <name>` | Scan all skills in a group (repeatable) |
| `-p, --project` | Scan project-level skills |
| `-g, --global` | Scan global skills |
| `--threshold <t>`, `-T <t>` | Block threshold: `critical\|high\|medium\|low\|info` (shorthand: `c\|h\|m\|l\|i`) |
| `--profile <p>` | Audit profile preset: `default`, `strict`, `permissive` |
| `--dedupe <mode>` | Dedup mode: `legacy`, `global` (default) |
| `--analyzer <id>` | Only run specified analyzer (repeatable): `static`, `dataflow`, `tier`, `integrity`, `structure`, `cross-skill` |
| `--format <f>` | Output format: `text` (default), `json`, `sarif`, `markdown` |
| `--json` | Same as `--format json` (deprecated) |
| `--quiet, -q` | Only show skills with findings + summary |
| `--no-tui` | Disable interactive TUI, print plain text |
| `--yes, -y` | Skip large-scan confirmation prompt |
| `--init-rules` | Create a starter `audit-rules.yaml` |
| `-h, --help` | Show help |

## Profiles

| Profile | Threshold | Dedupe | Use case |
|---------|-----------|--------|----------|
| `default` | `CRITICAL` | `global` | Standard — block only critical threats |
| `strict` | `HIGH` | `global` | Security-conscious teams |
| `permissive` | `CRITICAL` | `legacy` | Advisory-only — minimal blocking |

Explicit flags always override profile defaults.

## Severity Levels

| Level | Meaning | Default install behavior |
|-------|---------|--------------------------|
| **CRITICAL** | Prompt injection, data exfil, credential theft | **Blocked** (use `--force` to override) |
| **HIGH** | Destructive commands, hidden unicode, obfuscation | Warning shown |
| **MEDIUM** | Suspicious URLs, system path writes | Warning shown |
| **LOW** | Minor concerns, uncommon patterns | Warning shown |
| **INFO** | Informational observations | Warning shown |

## Config

```yaml
# config.yaml
audit:
  block_threshold: HIGH                         # Blocking severity gate
  profile: strict                               # Profile preset
  dedupe_mode: global                           # Dedup mode (global/legacy)
  enabled_analyzers: [static, dataflow, tier]   # Limit to specific analyzers
```

CLI flags override config values. Precedence: CLI > project config > global config > profile defaults.

## Install Integration

`skillshare install` auto-scans after download:

- **Findings at/above threshold → install blocked.** User must `--force` to proceed.
- **Findings below threshold → warning displayed** after successful install.
- **`--skip-audit`** skips security scanning entirely for a single install.

```bash
skillshare install user/repo              # Auto-audit, block on threshold
skillshare install user/repo --force      # Override block
skillshare install user/repo --skip-audit # Skip audit entirely
```

## Managing Rules

```bash
skillshare audit rules                          # Interactive TUI rule browser
skillshare audit rules --no-tui                 # Plain text table
skillshare audit rules disable <id>             # Disable single rule
skillshare audit rules disable --pattern <p>    # Disable entire pattern group
skillshare audit rules enable <id>              # Re-enable rule
skillshare audit rules severity <id> <level>    # Override severity
skillshare audit rules reset                    # Restore built-in defaults
skillshare audit rules init                     # Create starter audit-rules.yaml
```

## Custom Audit Rules

```bash
skillshare audit --init-rules       # Global: ~/.config/skillshare/audit-rules.yaml
skillshare audit --init-rules -p    # Project: .skillshare/audit-rules.yaml
```

Three-layer merge (later overrides earlier): built-in → global → project.

```yaml
rules:
  - id: flag-todo
    severity: MEDIUM
    pattern: todo-comment
    message: "TODO comment found"
    regex: '(?i)\bTODO\b'

  - id: insecure-http-0
    enabled: false              # Disable a built-in rule

  - pattern: destructive-commands
    severity: MEDIUM            # Downgrade entire pattern group
```

## Output

Summary includes severity breakdown (`c/h/m/l/i`) and threat category breakdown (`inj`, `exfil`, `cred`, `obfusc`, `priv`, `integ`, `struct`, `risk`).

`Failed` counts skills at/above threshold. `Warning` counts skills with findings below threshold.

## Logging

Audit results are logged to `audit.log` (JSONL). View with:

```bash
skillshare log --audit
```
