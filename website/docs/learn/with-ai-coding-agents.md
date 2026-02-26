---
sidebar_position: 7
---

# AI-Assisted Development

> Use AI coding agents to contribute to skillshare with pre-built project skills.

## Prerequisites

- An AI coding agent (Claude Code, Codex, etc.)
- The skillshare repository cloned locally

## Setup

The repo ships project-mode skills in `.skillshare/skills/`. Sync them to your agent:

```bash
skillshare sync -p
```

Your AI agent now has access to specialized skills for working on this codebase.

## Available Skills

| Skill | What it does |
|-------|-------------|
| `implement-feature` | Implement a feature from a spec file or description using TDD workflow |
| `update-docs` | Update website docs to match recent code changes, cross-validating every flag against source |
| `codebase-audit` | Cross-validate CLI flags, docs, tests, and targets for consistency across the codebase |
| `cli-e2e-test` | Run isolated E2E tests in devcontainer from runbooks |
| `changelog` | Generate a CHANGELOG.md entry from recent commits in conventional format |

## Typical Workflow

1. **Start a feature** — ask your agent to use `implement-feature` with a spec
2. **Update docs** — after code changes, invoke `update-docs` to sync website docs
3. **Audit consistency** — run `codebase-audit` to catch flag/doc mismatches
4. **Run E2E tests** — use `cli-e2e-test` to verify in a sandbox
5. **Write changelog** — invoke `changelog` before release

## What's Next?

- [Dev Containers setup →](/docs/learn/with-devcontainer)
- [Interactive Playground →](/docs/learn/with-playground)
- [Contributing guide →](https://github.com/runkids/skillshare/blob/main/CONTRIBUTING.md)
