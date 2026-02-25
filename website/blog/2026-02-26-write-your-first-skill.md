---
slug: write-your-first-skill
title: "Writing Your First Skill: A Step-by-Step Guide"
authors: [runkids]
tags: [skill-authoring, tutorial]
---

Skills are just Markdown files with frontmatter. If you can write a README, you can write a skill. Here's how to create one from scratch, audit it, and share it.

<!-- truncate -->

## What Is a Skill?

A skill is a `SKILL.md` file inside a named directory. It contains instructions that AI coding tools (Claude Code, Cursor, Codex, etc.) load as context when working on your code.

```
my-skill/
└── SKILL.md
```

That's the minimum. The `SKILL.md` file has two parts: frontmatter (metadata) and body (the actual instructions).

## Step 1: Scaffold with `skillshare new`

```bash
skillshare new my-code-review
```

This creates `~/.config/skillshare/skills/my-code-review/SKILL.md` with a template:

```markdown
---
name: my-code-review
description: >-
  Describe what this skill does. Use when user asks to
  "trigger phrase 1", "trigger phrase 2", or needs help
  with a specific task.
# targets: []           # e.g. [claude, cursor] — omit for all targets
# metadata:
#   author: Your Name
#   version: 1.0.0
---

# My Code Review

Brief overview of what this skill does and its value.
```

## Step 2: Write the Instructions

Replace the template body with clear, actionable instructions. Good skills are:

- **Specific** — tell the AI exactly what to do, not vague guidelines
- **Scoped** — one skill, one concern (code review, testing, commit messages)
- **Contextual** — explain when to apply the instructions

Here's an example code review skill:

```markdown
---
name: my-code-review
description: >-
  Enforce team code review standards. Check for security,
  error handling, input validation, and style compliance.
targets: [claude, cursor]
---

# Code Review Standards

When reviewing code (PR reviews, code suggestions, refactoring):

## Must Check
- No hardcoded secrets or credentials
- Error handling for all external calls (API, DB, file I/O)
- Input validation at system boundaries
- Tests for new public functions

## Style
- Functions under 30 lines
- No more than 3 parameters per function
- Early returns over nested conditionals
- Descriptive variable names (no single letters except loop counters)

## Response Format
- List issues by severity: critical > warning > suggestion
- Include file path and line number for each issue
- Suggest fixes, don't just flag problems
```

## Step 3: Security Scan

Before sharing, audit your skill:

```bash
skillshare audit
```

This checks for patterns that could be harmful: prompt injection, data exfiltration, destructive commands. Even well-intentioned skills can accidentally trigger these patterns.

## Step 4: Sync and Test

```bash
skillshare sync
```

Now open your AI tool and test the skill. Ask it to review some code and check whether it follows your instructions.

## Step 5: Share

### Option A: Commit to a shared repo

```bash
# Copy to a team repository
cp -r ~/.config/skillshare/skills/my-code-review /path/to/team-skills/
cd /path/to/team-skills
git add my-code-review/
git commit -m "Add code review skill"
git push
```

Team members install with:

```bash
skillshare install your-org/team-skills --skill my-code-review
```

### Option B: Project-scoped skill

```bash
cd your-project
skillshare init -p
cp -r ~/.config/skillshare/skills/my-code-review .skillshare/skills/
git add .skillshare/
git commit -m "Add project code review skill"
```

Anyone who clones the repo gets the skill automatically.

## Controlling Where Skills Sync

### Per-skill target restriction

Add a `targets` field to SKILL.md frontmatter to limit which AI tools receive this skill:

```yaml
---
name: claude-only-skill
description: Only syncs to Claude Code
targets: [claude]
---
```

Skills without a `targets` field sync to **all** configured targets.

### Per-target filtering

In your config, use `include`/`exclude` patterns to control which skills each target receives:

```yaml
targets:
  claude:
    path: ~/.claude/skills
    exclude:
      - "experimental-*"
  cursor:
    path: ~/.cursor/skills
    include:
      - "coding-*"
      - "review-*"
```

### `.skillignore` for repositories

If you publish a skill repository, add a `.skillignore` file to exclude internal/test skills from installation:

```
# .skillignore — skills to exclude during install
internal-testing
experimental
debug-*
```

Patterns support exact names, group prefixes, and trailing wildcards.

## Tips for Effective Skills

1. **Keep the description under 1024 characters** — Codex enforces this limit
2. **Use headers for structure** — AI tools parse Markdown headers for context
3. **Be imperative** — "Check for X" not "It would be good to check for X"
4. **Include examples** — Show the AI what good output looks like
5. **Use `targets` to scope** — Not every skill makes sense for every tool

## Resources

- [Creating skills guide](/docs/how-to/daily-tasks/creating-skills)
- [Skill format reference](/docs/understand/skill-format)
- [Best practices](/docs/how-to/daily-tasks/best-practices)
