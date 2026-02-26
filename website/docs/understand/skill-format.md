---
sidebar_position: 5
---

# Skill Format

The structure and metadata of a skillshare skill.

:::tip When does this matter?
The SKILL.md format determines how AI CLIs discover and load your skill. The `description` field is especially critical — it's what the AI uses to decide when to activate your skill.
:::

## Overview

A skill is a directory containing at least a `SKILL.md` file:

```
my-skill/
└── SKILL.md
```

The `SKILL.md` file has two parts:
1. **YAML frontmatter** — Metadata
2. **Markdown body** — Instructions for the AI

---

## Basic Structure

```markdown
---
name: my-skill
description: Brief description of what this skill does
---

# My Skill

Instructions for the agent when this skill is activated.

## When to Use

Describe when this skill should be used.

## Instructions

1. First step
2. Second step
3. Additional steps as needed
```

---

## Required Fields

### `name`

The skill identifier. Used for:
- Invoking the skill (e.g., `/skill:my-skill`)
- Collision detection
- Display in skill lists

```yaml
name: my-skill
```

**Rules:**
- Lowercase letters, numbers, hyphens, underscores
- Must start with a letter or number
- Should be unique across all skills

**Examples:**
```yaml
name: code-review
name: pdf-tools
name: acme-frontend-ui  # Namespaced for teams
```

---

## Optional Fields

### `description`

Brief description shown in skill lists and search results.

```yaml
description: Reviews code for bugs, style issues, and improvements
```

---

## Optional Fields

### `tags`

Classification tags for filtering and grouping in hub indexes. When you run `skillshare hub index`, tags from SKILL.md frontmatter are included in the generated `skillshare-hub.json`.

```yaml
tags: git, workflow
```

Tags are also searchable — `skillshare search workflow --hub ...` matches skills tagged with "workflow".

### `targets`

Restrict which targets this skill syncs to. When omitted, the skill syncs to **all** targets.

```yaml
targets: [claude, cursor]
```

| Value | Behavior |
|-------|----------|
| *(omitted)* | Syncs to all targets (default) |
| `[claude]` | Syncs only to targets matching "claude" |
| `[claude, cursor]` | Syncs to targets matching either name |

**Cross-mode matching:** A skill declaring `targets: [claude]` also matches the project target `claude`, because both refer to the same AI CLI. Matching uses the [target registry](/docs/reference/targets/supported-targets).

**Interaction with config filters:** Skill-level `targets` is applied **after** config-level `include`/`exclude`. Both must pass for a skill to be synced. See [Configuration](/docs/reference/targets/configuration#skill-level-targets).

**Example — Claude-only skill:**

```markdown
---
name: claude-prompts
description: Prompt patterns for Claude Code
targets: [claude]
---

# Claude Prompts
...
```

This skill will only appear in Claude Code's skills directory, even if you have Cursor, Codex, and other targets configured.

### `license`

The skill's license identifier. Displayed during installation to help with compliance decisions.

```yaml
license: MIT
```

When present, `skillshare install` shows the license in the skill selection prompt and confirmation screen:

- **Single skill**: Displayed as `License: MIT` in the skill info box
- **Multi-skill repo**: Appended to the skill name in the selection list (e.g., `my-skill (MIT)`)

This is purely informational — it does not block installation. Common values: `MIT`, `Apache-2.0`, `GPL-3.0`, `BSD-3-Clause`, `ISC`.

---

## Custom Metadata

You can add any custom fields:

```yaml
---
name: my-skill
description: My custom skill
author: Your Name
version: 1.0.0
---
```

Custom fields are stored in the frontmatter but not used by skillshare itself.

---

## Markdown Body

The body contains instructions for the AI. Write it as if you're instructing a human assistant.

**Good practices:**
- Clear, specific instructions
- Examples of inputs and expected outputs
- Edge cases and error handling
- When to use (and when NOT to use)

**Example:**
```markdown
# Code Review

You are a code reviewer. Analyze code for:
- Bugs and potential issues
- Style and consistency
- Performance concerns
- Security vulnerabilities

## When to Use

Use this skill when the user asks you to review code, find bugs, or improve code quality.

## Instructions

1. Read the provided code carefully
2. Identify issues in order of severity
3. Suggest specific improvements with code examples
4. Be constructive and explain your reasoning

## Example

User: "Review this function"
```python
def add(a, b):
  return a + b
```

Response: "The function looks correct but could benefit from type hints..."
```

---

## Skill Metadata File

When you install a skill, skillshare creates a `.skillshare-meta.json` file:

```json
{
  "source": "anthropics/skills/skills/pdf",
  "type": "github",
  "installed_at": "2026-01-20T15:30:00Z",
  "repo_url": "https://github.com/anthropics/skills.git",
  "subdir": "skills/pdf",
  "version": "abc1234"
}
```

| Field | Description |
|-------|-------------|
| `source` | Original install source input |
| `type` | Source type (`github`, `local`, etc.) |
| `installed_at` | Installation timestamp |
| `repo_url` | Git clone URL (git sources only) |
| `subdir` | Subdirectory path (monorepo sources only) |
| `version` | Git commit hash at install time |

This is used by `skillshare update` and `skillshare check` to know where to fetch updates from.

**Don't edit this file manually.**

---

## Creating a Skill

```bash
skillshare new my-skill
```

This creates:
```
~/.config/skillshare/skills/my-skill/
└── SKILL.md  (with template)
```

Edit the generated `SKILL.md` and run `skillshare sync` to deploy.

---

## Validating Skills

```bash
skillshare doctor
```

Checks for:
- Valid SKILL.md format
- Required `name` field
- Valid frontmatter YAML
- Name collisions

---

## See Also

- [new](/docs/reference/commands/new) — Create a skill with the correct template
- [Creating Skills](/docs/how-to/daily-tasks/creating-skills) — Full guide to writing skills
- [Best Practices](/docs/how-to/daily-tasks/best-practices) — Naming and organization tips
