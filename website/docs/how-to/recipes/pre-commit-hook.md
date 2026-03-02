---
sidebar_position: 3
---

# Recipe: Pre-commit Hook

> Run `skillshare audit` automatically on every commit using the [pre-commit](https://pre-commit.com/) framework.

## When to Use

The pre-commit hook is most valuable when:

- **Multiple contributors edit skills** — team members may inadvertently introduce dangerous commands (`curl | bash`, `sudo rm -rf`). The hook catches these before they enter version control.
- **Skills come from external sources** — copying skills from GitHub, community repos, or AI-generated content makes manual review difficult. Automated scanning provides a safety net.
- **You want instant feedback** — CI catches issues too, but only after push. The hook gives developers immediate, local feedback in seconds.

You can skip it when:

- You are the sole author and trust all your skills
- Skills rarely change (the hook only runs when `.skillshare/` or `skills/` files are modified)

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
