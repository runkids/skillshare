---
sidebar_position: 4
---

# audit rules

Browse, enable, disable, and customize audit rules.

```bash
skillshare audit rules                          # Interactive TUI rule browser
skillshare audit rules --no-tui                 # Plain text table
skillshare audit rules --pattern credential-access  # Filter by pattern
skillshare audit rules --severity high          # Filter by severity
skillshare audit rules --disabled               # Show only disabled rules
skillshare audit rules --format json            # JSON output

skillshare audit rules disable prompt-injection-0           # Disable single rule
skillshare audit rules disable --pattern credential-access  # Disable entire group
skillshare audit rules enable prompt-injection-0            # Re-enable rule
skillshare audit rules enable --pattern credential-access   # Re-enable group

skillshare audit rules severity destructive-commands-2 medium      # Downgrade one rule
skillshare audit rules severity --pattern destructive-commands low  # Downgrade entire group
skillshare audit rules reset                    # Remove all custom rules, restore defaults

skillshare audit rules init                     # Create starter audit-rules.yaml
skillshare audit rules init -p                  # Create project-level rules file
```

## Pattern-Level Rules

You can disable or override entire pattern groups in `audit-rules.yaml`:

```yaml
rules:
  # Disable all credential-access rules
  - pattern: credential-access
    enabled: false

  # But keep .env detection
  - id: credential-access-env-file
    enabled: true

  # Downgrade all destructive-commands to MEDIUM
  - pattern: destructive-commands
    severity: MEDIUM
```

Pattern-level entries use `pattern` without `id`. Merge order: pattern-level rules apply first, then id-level rules can override individual entries within a disabled group.

## Custom Rules

You can add, override, or disable audit rules using YAML files. Rules are merged in order: **built-in → global user → project user**.

Use `--init-rules` (or `audit rules init`) to create a starter file with commented examples:

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
  - id: insecure-http-0
    enabled: false

  # Disable the dangling-link structural check
  - id: dangling-link
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
- **`pattern` only** (no `id`) + `enabled: false` → disables all rules matching that pattern
- **`pattern` only** + `severity` → overrides severity for all matching rules
- **Pattern then id** → id-level entries can re-enable individual rules within a disabled pattern group

### Practical Templates

Use this as a starting point for real-world policy tuning:

```yaml
rules:
  # Downgrade hardcoded-secret to MEDIUM for educational/reference skills
  - pattern: hardcoded-secret
    severity: MEDIUM

  # Override built-in suspicious-fetch with internal allowlist
  - id: suspicious-fetch-0
    severity: MEDIUM
    pattern: suspicious-fetch
    message: "External URL used in command context"
    regex: '(?i)(curl|wget|invoke-webrequest|iwr)\s+https?://'
    exclude: '(?i)https?://(localhost|127\.0\.0\.1|artifacts\.company\.internal|registry\.company\.internal)'

  # Governance exception: disable noisy insecure-http signal
  - id: insecure-http-0
    enabled: false
```

### Getting Started with `init`

`audit rules init` (or `audit --init-rules`) creates a starter `audit-rules.yaml` with commented examples you can uncomment and adapt:

```bash
skillshare audit rules init          # → ~/.config/skillshare/audit-rules.yaml
skillshare audit rules init -p       # → .skillshare/audit-rules.yaml
```

The generated file looks like this:

```yaml
# Custom audit rules for skillshare.
# Rules are merged on top of built-in rules in order:
#   built-in → global (~/.config/skillshare/audit-rules.yaml)
#            → project (.skillshare/audit-rules.yaml)
#
# Each rule needs: id, severity, pattern, message, regex.
# Optional: exclude (suppress match), enabled (false to disable).

rules:
  # Example: flag TODO comments as informational
  # - id: flag-todo
  #   severity: MEDIUM
  #   pattern: todo-comment
  #   message: "TODO comment found"
  #   regex: '(?i)\bTODO\b'

  # Example: disable a built-in rule by id
  # - id: insecure-http-0
  #   enabled: false

  # Example: disable the dangling-link structural check
  # - id: dangling-link
  #   enabled: false

  # Example: override a built-in rule (match by id, change severity)
  # - id: destructive-commands-2
  #   severity: MEDIUM
  #   pattern: destructive-commands
  #   message: "Sudo usage (downgraded)"
  #   regex: '(?i)\bsudo\s+'
```

If the file already exists, `init` exits with an error — it never overwrites existing rules.

## Workflow: Fixing a False Positive

A common reason to customize rules is when a legitimate skill triggers a built-in rule. Here's a step-by-step example:

**1. Run audit and see the false positive:**

```bash
$ skillshare audit ci-helper
[1/1] ! ci-helper    0.2s
      └─ HIGH: Destructive command pattern (SKILL.md:42)
         "sudo apt-get install -y jq"
```

**2. Identify the rule ID from the [built-in rules table](#built-in-rule-ids):**

The pattern `destructive-commands` with `sudo` matches rule `destructive-commands-2`.

**3. Create a custom rules file (if you haven't already):**

```bash
skillshare audit rules init
```

**4. Add a rule override to suppress or downgrade:**

```yaml
# ~/.config/skillshare/audit-rules.yaml
rules:
  # Downgrade sudo to MEDIUM for CI automation skills
  - id: destructive-commands-2
    severity: MEDIUM
    pattern: destructive-commands
    message: "Sudo usage (downgraded for CI automation)"
    regex: '(?i)\bsudo\s+'
```

Or disable it entirely:

```yaml
rules:
  - id: destructive-commands-2
    enabled: false
```

**5. Re-run audit to confirm:**

```bash
$ skillshare audit ci-helper
[1/1] ✓ ci-helper    0.1s   # Now passes (or shows MEDIUM instead of HIGH)
```

### Validate Changes

After editing rules, re-run audit to verify:

```bash
skillshare audit                     # Check all skills
skillshare audit <name>              # Check a specific skill
skillshare audit --json | jq '.skills[].findings'  # Inspect findings programmatically
```

Summary interpretation:

- `Failed` counts skills with findings at or above the active threshold.
- `Warning` counts skills with findings below threshold but above clean (for example `HIGH/MEDIUM/LOW/INFO` when threshold is `CRITICAL`).

## Built-in Rule IDs

Use `id` values to override or disable specific built-in rules:

Source of truth for regex-based rules:
[`internal/audit/rules.yaml`](https://github.com/runkids/skillshare/blob/main/internal/audit/rules.yaml)

:::note Structural, tier, and cross-skill checks

`dangling-link`, `content-tampered`, `content-oversize`, `content-missing`, and `content-unexpected` are **structural checks** (filesystem lookups and hash comparisons, not regex). `low-analyzability` is an **analyzability finding** generated from the [Analyzability Score](/docs/understand/audit-engine#analyzability-score). `tier-stealth`, `tier-destructive-network`, `tier-network-heavy`, `tier-interpreter`, and `tier-interpreter-network` are **tier combination findings** generated from [Command Safety Tiering](/docs/understand/audit-engine#command-safety-tiering) profiles. `cross-skill-*` findings are generated from [Cross-Skill Interaction Detection](/docs/understand/audit-engine#cross-skill-interaction-detection). All of these appear in the table below but are not defined in `rules.yaml`.

:::

| ID | Pattern | Severity |
|----|---------|----------|
| `prompt-injection-0` | prompt-injection | CRITICAL |
| `prompt-injection-1` | prompt-injection | CRITICAL |
| `prompt-injection-2` | prompt-injection | HIGH |
| `prompt-injection-3` | prompt-injection | CRITICAL |
| `prompt-injection-4` | prompt-injection | CRITICAL |
| `hidden-unicode-1` | invisible-payload | CRITICAL |
| `data-exfiltration-0` | data-exfiltration | CRITICAL |
| `data-exfiltration-1` | data-exfiltration | CRITICAL |
| `data-exfiltration-2` | data-exfiltration | MEDIUM |
| `data-exfiltration-3` | data-exfiltration | HIGH |
| `credential-access-ssh-private-key` | credential-access | CRITICAL |
| `credential-access-env-file` | credential-access | CRITICAL |
| `credential-access-aws-credentials` | credential-access | CRITICAL |
| `credential-access-etc-shadow` | credential-access | CRITICAL |
| `credential-access-git-credentials` | credential-access | CRITICAL |
| `credential-access-netrc` | credential-access | CRITICAL |
| `credential-access-gnupg` | credential-access | CRITICAL |
| `credential-access-kube-config` | credential-access | CRITICAL |
| `credential-access-vault-token` | credential-access | CRITICAL |
| `credential-access-terraform-creds` | credential-access | CRITICAL |
| `credential-access-gnome-keyring` | credential-access | CRITICAL |
| `credential-access-npmrc` | credential-access | CRITICAL |
| `credential-access-pypirc` | credential-access | CRITICAL |
| `credential-access-gem-credentials` | credential-access | CRITICAL |
| `credential-access-ssl-private` | credential-access | CRITICAL |
| `credential-access-ssh-host-key` | credential-access | CRITICAL |
| `credential-access-pgpass` | credential-access | CRITICAL |
| `credential-access-mysql-cnf` | credential-access | CRITICAL |
| `credential-access-etc-passwd` | credential-access | MEDIUM |
| `credential-access-azure-creds` | credential-access | HIGH |
| `credential-access-gcloud-creds` | credential-access | HIGH |
| `credential-access-docker-config` | credential-access | HIGH |
| `credential-access-gh-cli-token` | credential-access | HIGH |
| `credential-access-password-store` | credential-access | HIGH |
| `credential-access-macos-keychain-user` | credential-access | HIGH |
| `credential-access-macos-keychain-sys` | credential-access | HIGH |
| `credential-access-terraformrc` | credential-access | HIGH |
| `credential-access-cargo-credentials` | credential-access | HIGH |
| `credential-access-op-cli` | credential-access | HIGH |
| `credential-access-age-keys` | credential-access | HIGH |
| `credential-access-shell-history` | credential-access | LOW |
| `credential-access-openvpn` | credential-access | LOW |
| `credential-access-auth-log` | credential-access | INFO |
| `credential-access-unknown-dotdir` | credential-access | INFO |

> **Note:** Each credential entry above also generates variant IDs per access method: `-copy`, `-redirect`, `-dd`, `-exfil` (e.g., `credential-access-ssh-private-key-copy`). To disable a specific variant, use its full ID in your `audit-rules.yaml`.

| ID | Pattern | Severity |
|----|---------|----------|
| `hidden-unicode-0` | hidden-unicode | HIGH |
| `hidden-unicode-2` | hidden-unicode | HIGH |
| `config-manipulation-0` | config-manipulation | HIGH |
| `hidden-comment-injection-1` | hidden-comment-injection | HIGH |
| `self-propagation-0` | self-propagation | HIGH |
| `destructive-commands-0` | destructive-commands | HIGH |
| `destructive-commands-1` | destructive-commands | HIGH |
| `destructive-commands-2` | destructive-commands | HIGH |
| `destructive-commands-3` | destructive-commands | HIGH |
| `destructive-commands-4` | destructive-commands | HIGH |
| `dynamic-code-exec-0` | dynamic-code-exec | HIGH |
| `dynamic-code-exec-1` | dynamic-code-exec | HIGH |
| `shell-execution-0` | shell-execution | HIGH |
| `hidden-comment-injection-0` | hidden-comment-injection | HIGH |
| `obfuscation-0` | obfuscation | HIGH |
| `fetch-with-pipe-0` | fetch-with-pipe | HIGH |
| `fetch-with-pipe-1` | fetch-with-pipe | HIGH |
| `fetch-with-pipe-2` | fetch-with-pipe | HIGH |
| `hardcoded-secret-0` | hardcoded-secret | HIGH |
| `hardcoded-secret-1` | hardcoded-secret | HIGH |
| `hardcoded-secret-2` | hardcoded-secret | HIGH |
| `hardcoded-secret-3` | hardcoded-secret | HIGH |
| `hardcoded-secret-4` | hardcoded-secret | HIGH |
| `hardcoded-secret-5` | hardcoded-secret | HIGH |
| `hardcoded-secret-6` | hardcoded-secret | HIGH |
| `hardcoded-secret-7` | hardcoded-secret | HIGH |
| `hardcoded-secret-8` | hardcoded-secret | HIGH |
| `hardcoded-secret-9` | hardcoded-secret | HIGH |
| `data-uri-0` | data-uri | MEDIUM |
| `escape-obfuscation-0` | escape-obfuscation | MEDIUM |
| `suspicious-fetch-0` | suspicious-fetch | MEDIUM |
| `ip-address-url-0` | ip-address-url | MEDIUM |
| `hidden-unicode-3` | hidden-unicode | MEDIUM |
| `untrusted-install-0` | untrusted-install | MEDIUM |
| `untrusted-install-1` | untrusted-install | MEDIUM |
| `insecure-http-0` | insecure-http | LOW |
| `external-link-0` | external-link | LOW |
| `dangling-link` | dangling-link | LOW |
| `content-tampered` | content-tampered | MEDIUM |
| `content-oversize` | content-oversize | MEDIUM |
| `content-missing` | content-missing | LOW |
| `content-unexpected` | content-unexpected | LOW |
| `shell-chain-0` | shell-chain | INFO |
| `low-analyzability` | low-analyzability | INFO |
| `tier-stealth` | tier-stealth | CRITICAL |
| `tier-destructive-network` | tier-destructive-network | HIGH |
| `tier-network-heavy` | tier-network-heavy | MEDIUM |
| `tier-interpreter` | tier-interpreter | INFO |
| `tier-interpreter-network` | tier-interpreter-network | MEDIUM |
| `cross-skill-exfiltration` | cross-skill-exfiltration | HIGH |
| `cross-skill-privilege-network` | cross-skill-privilege-network | MEDIUM |
| `cross-skill-stealth` | cross-skill-stealth | HIGH |
| `cross-skill-cred-interpreter` | cross-skill-cred-interpreter | MEDIUM |

## Subcommands

| Subcommand | Description |
|-----------|-------------|
| `rules` | Browse, enable, and disable audit rules |
| `rules disable <id>` | Disable a single rule by ID |
| `rules disable --pattern <p>` | Disable all rules matching a pattern |
| `rules enable <id>` | Re-enable a single rule by ID |
| `rules enable --pattern <p>` | Re-enable all rules matching a pattern |
| `rules severity <id> <level>` | Override severity for a single rule |
| `rules severity --pattern <p> <level>` | Override severity for all rules in a pattern group |
| `rules reset` | Remove all custom rules (restore built-in defaults) |
| `rules init` | Create a starter `audit-rules.yaml` (same as `audit --init-rules`) |

## See Also

- [`audit`](/docs/reference/commands/audit) — Main audit command reference
- [Audit Engine](/docs/understand/audit-engine) — How the engine works (threat model, risk scoring, tiering)
- [Securing Your Skills](/docs/how-to/advanced/security) — Security guide for teams
- [CI/CD Skill Validation](/docs/how-to/recipes/ci-cd-skill-validation) — Pipeline automation recipe
