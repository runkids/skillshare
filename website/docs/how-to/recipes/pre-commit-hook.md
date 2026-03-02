---
sidebar_position: 3
---

# Recipe: Pre-commit Hook

> Run `skillshare audit` automatically on every commit using the [pre-commit](https://pre-commit.com/) framework.

## Scenario

You want to catch security issues in skills **before** they're committed, rather than relying solely on CI to catch problems after the fact.

## Setup

Add to your project's `.pre-commit-config.yaml`:

```yaml
repos:
  - repo: https://github.com/runkids/skillshare
    rev: v0.16.8  # use latest release tag
    hooks:
      - id: skillshare-audit
```

Then install the hook:

```bash
pre-commit install
```

## How It Works

The hook runs `skillshare audit -p` whenever you commit changes to files matching `.skillshare/` or `skills/` directories. If any findings exceed the configured threshold, the commit is blocked.

## Configuration

The hook respects your project's `.skillshare/config.yaml` settings:

```yaml
audit:
  block_threshold: high  # block on HIGH+ findings
```

## Skipping the Hook

For a one-time skip:

```bash
SKIP=skillshare-audit git commit -m "your message"
```

## Requirements

- `skillshare` CLI must be installed and available in `PATH`
- Project must be initialized with `skillshare init -p`

## Combining with CI

The pre-commit hook catches issues locally, while [CI/CD validation](ci-cd-skill-validation.md) provides a safety net for the whole team. Use both for defense in depth.
