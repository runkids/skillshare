# Security Audit

Scan skills for prompt injection, data exfiltration, credential access, destructive commands, obfuscation, suspicious URLs, and broken local links. Use `audit rules --no-tui` for the installed rule set.

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
| `--analyzer <id>` | Only run specified analyzer (repeatable): `static`, `dataflow`, `tier`, `integrity`, `metadata`, `structure`, `cross-skill` |
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
| `permissive` | `CRITICAL` | `legacy` | Legacy deduplication; still blocks CRITICAL |

Explicit flags always override profile defaults.

## Severity Levels

| Level | Meaning | Default install behavior |
|-------|---------|--------------------------|
| **CRITICAL** | Prompt injection, data exfil, credential theft | **Blocked** (use `--force` to override) |
| **HIGH** | Destructive commands, hidden unicode, obfuscation | Warning shown |
| **MEDIUM** | Suspicious URLs, system path writes | Warning shown |
| **LOW** | Minor concerns, uncommon patterns | Warning shown |
| **INFO** | Informational observations | Warning shown |

### Markdown Context and Disclosure

Markdown is still scanned, including fenced code blocks. A shared Markdown parser supplies code-block context; it does not establish trust. Fenced shell samples in raw HTML blocks and comments also receive command-tier and dataflow analysis, with taint isolated per block. Raw HTML blocks do not qualify for SDK parameter downgrades.

- `prompt-injection-1`: SDK-style `system:` / `System:` parameters inside a fenced code block are downgraded from CRITICAL to HIGH. Recognized values include quoted strings, arrays, function calls, comma-terminated variables, YAML block scalars, and a quoted string or array on the next line. Uppercase `SYSTEM:` directives, bare role labels, and prose outside fences do not qualify. Other rules still scan the parameter contents.
- `prompt-injection-4`: explicit concealment of actions, changes, or instructions, and requests to hide content or remove conversation history, remain CRITICAL.
- `prompt-injection-5`: generic disclosure restrictions using `don't` or `do not` followed by `tell the user` are HIGH in all file types. These can be advice or concealment; no framework, repository, or schema wording is treated as proof of safety. A separate explicit-concealment match on the same line still blocks at CRITICAL.

HIGH findings warn at the default CRITICAL threshold and block at a HIGH threshold (the `strict` profile default). Some malicious wording can also match only the generic HIGH rule; use strict when that ambiguity must block. SDK parameter context is only a syntax heuristic. Credential access, data exfiltration, and other injection rules retain their configured severity.

Explicit global or project severity overrides, including CRITICAL, take precedence over SDK parameter downgrades. Generic disclosure restrictions now use `prompt-injection-5`; overrides for `prompt-injection-4` apply only to explicit concealment. Built-in rule 5 excludes explicit-concealment phrases, not separate generic restrictions on the same line. Full custom regex replacements retain whole-line exclusion semantics. To make generic restrictions CRITICAL, override `prompt-injection-5`. For reviewed false positives, findings accepted with `--force` remain visible and are remembered for that resource; a different rule, file, or matched text requires acceptance again.

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

- **Findings at/above threshold → install blocked.** Inspect findings; explicit `--force` overrides the gate.
- **Findings below threshold → warning displayed** after successful install.
- **`--skip-audit`** skips security scanning entirely for a single install.

```bash
skillshare install user/repo              # Auto-audit, block on threshold
skillshare install user/repo --force      # Override block
skillshare install user/repo --skip-audit # Skip audit entirely
```

`install --json` retains the audit gate even though it enables overwrite/noninteractive
selection. Explicit `--force` and `--skip-audit` have different security effects.

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

Audit exits `0` when findings are below the threshold, `1` for threshold findings, and `2` for execution, usage, configuration, or scan errors. Scan errors take precedence over findings.

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
