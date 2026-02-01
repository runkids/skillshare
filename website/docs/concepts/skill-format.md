---
sidebar_position: 5
---

# Skill Format

The structure and metadata of a skillshare skill.

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
- Must start with a letter or underscore
- Should be unique across all skills

**Examples:**
```yaml
name: code-review
name: pdf-tools
name: acme:frontend-ui  # Namespaced for teams
```

---

## Optional Fields

### `description`

Brief description shown in skill lists and search results.

```yaml
description: Reviews code for bugs, style issues, and improvements
```

---

## Custom Metadata

You can add any custom fields:

```yaml
---
name: my-skill
description: My custom skill
author: Your Name
version: 1.0.0
tags: [productivity, code-review]
---
```

These are stored but not used by skillshare itself.

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

When you install a skill, skillshare creates a `.skillshare.yaml` file:

```yaml
# ~/.config/skillshare/skills/pdf/.skillshare.yaml
source: github.com/anthropics/skills/skills/pdf
installed_at: 2026-01-20T15:30:00Z
type: git
```

This is used by `skillshare update` to know where to fetch updates from.

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

## Related

- [Creating Skills](/docs/guides/creating-skills) — Step-by-step guide
- [Commands: new](/docs/commands/new) — Create command
- [Commands: doctor](/docs/commands/doctor) — Diagnose issues
