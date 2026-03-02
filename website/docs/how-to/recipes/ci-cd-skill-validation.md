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
- [Docker sandbox guide](/docs/how-to/advanced/docker-sandbox)
