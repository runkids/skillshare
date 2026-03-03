---
sidebar_position: 2
---

# Recipe: CI/CD Skill Validation

> Audit and sync skills automatically in your CI pipeline.

## Scenario

You have a team skill repository and want to ensure every PR:
- Passes security audit (no prompt injection, credential theft, etc.)
- Validates SKILL.md format
- Syncs without errors

## Solution

### GitHub Actions

Create `.github/workflows/skill-validation.yml`:

```yaml
name: Skill Validation
on:
  pull_request:
    paths:
      - 'skills/**'

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install skillshare
        run: curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh

      - name: Initialize
        run: skillshare init

      - name: Install skills from this repo
        run: skillshare install . --into ci-check

      - name: Security audit
        run: skillshare audit --threshold high --format json

      - name: Dry-run sync
        run: skillshare sync --dry-run
```

### GitHub Actions with SARIF Upload

To get inline PR annotations via [GitHub Code Scanning](https://docs.github.com/en/code-security/code-scanning), add SARIF output alongside the JSON report:

```yaml
name: Skill Security Scan
on:
  pull_request:
    paths: ['skills/**']
  push:
    branches: [main]

jobs:
  validate:
    runs-on: ubuntu-latest
    permissions:
      security-events: write
    steps:
      - uses: actions/checkout@v4

      - name: Install skillshare
        run: curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh

      - name: Initialize
        run: skillshare init

      - name: Install skills from this repo
        run: skillshare install . --into ci-check

      - name: Security audit (JSON + SARIF)
        run: |
          skillshare audit --threshold high --format json > audit-report.json
          skillshare audit --threshold high --format sarif > results.sarif || true

      - name: Upload SARIF to Code Scanning
        if: always()
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: results.sarif
          category: skillshare-audit

      - name: Upload JSON report
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: audit-report
          path: audit-report.json

      - name: Dry-run sync
        run: skillshare sync --dry-run
```

### GitLab CI

Create `.gitlab-ci.yml`:

```yaml
skill-validation:
  image: ghcr.io/runkids/skillshare-ci:latest
  stage: test
  script:
    - skillshare init
    - skillshare install . --into ci-check
    - skillshare audit --threshold high --format json
    - skillshare sync --dry-run
  rules:
    - changes:
        - skills/**/*
```

### Using the CI Docker Image

For faster pipeline startup, use the pre-built CI image:

```yaml
# GitHub Actions
jobs:
  validate:
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/runkids/skillshare-ci:latest
    steps:
      - uses: actions/checkout@v4
      - run: skillshare init && skillshare audit --format json
```

## Output Formats

The `audit` command supports multiple output formats for different CI/CD integration needs.

### Exit Codes

```bash
# Block deployment if any skill has findings at or above threshold
skillshare audit --threshold high
echo $?  # 0 = clean, 1 = findings found
```

### SARIF Output

[SARIF (Static Analysis Results Interchange Format)](https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html) is an OASIS standard consumed by GitHub Code Scanning, VS Code SARIF Viewer, Azure DevOps, SonarQube, and other static analysis tools.

```bash
skillshare audit --format sarif              # Output to stdout
skillshare audit --format sarif > results.sarif  # Save to file
```

The SARIF output includes:
- **Tool metadata** — tool name (`skillshare`), version, and information URI
- **Rules** — deduplicated rule descriptors with `security-severity` scores
- **Results** — each finding mapped to a SARIF result with file location and severity level

Severity mapping to SARIF levels:

| skillshare Severity | SARIF Level | security-severity |
|---------------------|-------------|-------------------|
| CRITICAL | `error` | 9.0 |
| HIGH | `error` | 7.0 |
| MEDIUM | `warning` | 4.0 |
| LOW | `note` | 2.0 |
| INFO | `note` | 0.5 |

### Markdown Report

Generate a self-contained Markdown report suitable for pasting into GitHub Issues, Pull Requests, or documentation:

```bash
skillshare audit --format markdown               # Print to stdout
skillshare audit --format markdown > report.md   # Save to file
skillshare audit -p --format markdown > report.md  # Project mode
```

The report includes:
- **Header** — scanned count, mode, and threshold
- **Summary table** — passed/warning/failed counts, severity breakdown, risk score, analyzability
- **Findings** — per-skill tables with severity, pattern, message, and location; collapsible snippets
- **Clean Skills** — comma-separated list of skills with no findings

### JSON Output with jq

```bash
# List all skills with CRITICAL findings
skillshare audit --json | jq '[.skills[] | select(.findings[] | .severity == "CRITICAL")]'

# Extract risk scores for all skills
skillshare audit --json | jq '.skills[] | {name: .skillName, score: .riskScore, label: .riskLabel}'

# Count findings by severity
skillshare audit --json | jq '[.skills[].findings[].severity] | group_by(.) | map({(.[0]): length}) | add'
```

## Verification

- PR check passes: audit exits 0 (no findings at/above threshold)
- Audit JSON output can be parsed by downstream tools
- SARIF upload shows findings as inline annotations on PR diffs
- Sync dry-run shows expected symlink operations

## Variations

- **Block on HIGH severity**: Add `--threshold HIGH` (or `-T HIGH`) to `audit` — any HIGH+ finding exits non-zero
- **SARIF for Code Scanning**: Use `--format sarif` with `github/codeql-action/upload-sarif@v3` for inline PR annotations
- **Parallel validation**: Run audit and sync in separate CI jobs for faster feedback
- **Scheduled audits**: Run nightly to catch newly detected patterns in existing skills

## Related

- [Security audit guide](/docs/how-to/advanced/security)
- [`audit` command reference](/docs/reference/commands/audit)
- [`audit rules` reference](/docs/reference/commands/audit-rules)
- [Audit Engine](/docs/understand/audit-engine) — How the engine works
- [Docker sandbox guide](/docs/how-to/advanced/docker-sandbox)
