---
sidebar_position: 4
---

# new

Create a new skill with a SKILL.md template.

```bash
skillshare new <name>            # Create a new skill
skillshare new <name> -p         # Create in project (.skillshare/skills/)
skillshare new <name> --dry-run  # Preview without creating
```

## When to Use

- Create a new skill from scratch with the recommended template structure
- Start with the correct SKILL.md format (name, description, frontmatter)

**What happens:**
```mermaid
flowchart TD
    CMD["skillshare new my-skill"]
    VALIDATE["1. Validate skill name"]
    CREATE["2. Create skill directory"]
    GENERATE["3. Generate SKILL.md template"]
    CMD --> VALIDATE --> CREATE --> GENERATE
```

---

## Options

| Flag | Description |
|------|-------------|
| `--project`, `-p` | Create in project (`.skillshare/skills/`) |
| `--global`, `-g` | Create in global (`~/.config/skillshare/skills/`) |
| `--dry-run`, `-n` | Preview without creating files |
| `--help`, `-h` | Show help |

Auto-detection: if `.skillshare/config.yaml` exists in the current directory, defaults to project mode.

---

## Skill Name Rules

- Lowercase letters, numbers, hyphens, underscores
- Must start with a letter or underscore
- Examples: `my-skill`, `code_review`, `pdf-tools`

---

## Template Structure

The generated SKILL.md follows [Anthropic's skill-building best practices](https://www.anthropic.com/engineering/building-skills-for-claude):

```markdown
---
name: my-skill
description: >-
  Describe what this skill does. Use when user asks to
  "trigger phrase 1", "trigger phrase 2", or needs help
  with a specific task.
# ── Optional fields ──────────────────────────────────
# license: MIT
# allowed-tools: "Bash(python:*) WebFetch"
# metadata:
#   author: Your Name
#   version: 1.0.0
---

# My Skill

Brief overview of what this skill does and its value.

## When to Use

Use this skill when the user:
- Asks to "specific trigger phrase"
- Mentions specific keywords or file types
- Needs help with a particular task

Do NOT use this skill for:
- Unrelated tasks (clarify scope boundaries)

## Instructions

### Step 1: Gather Context
### Step 2: Execute
### Step 3: Validate

## Examples

**Example:** Common scenario
User says: "Help me with <my-skill-related task>"

## Troubleshooting

**Error:** Common error message
**Cause:** Why it happens
**Solution:** How to fix it
```

### Key design choices

The template follows Anthropic's [three-level progressive disclosure](https://www.anthropic.com/engineering/building-skills-for-claude) model:

| Level | What | Loaded when |
|-------|------|-------------|
| **1. Frontmatter** | `name` + `description` | Always (system prompt) |
| **2. SKILL.md body** | Full instructions | When skill is relevant |
| **3. Linked files** | `references/`, `scripts/` | On demand |

**Description must include WHAT + WHEN** — This is the single most important field. Claude uses it to decide whether to load your skill. Bad: `"Helps with projects"`. Good: `"Manages sprint planning. Use when user says 'plan sprint' or 'create tickets'."` See [Anthropic's guide](https://www.anthropic.com/engineering/building-skills-for-claude) for more examples.

---

## Examples

### Create a simple skill

```bash
skillshare new code-review
```

Output:
```
New Skill Created
─────────────────────────────────────────────
✓ Created: ~/.config/skillshare/skills/code-review/SKILL.md

Next steps:
  1. Edit ~/.config/skillshare/skills/code-review/SKILL.md
  2. Run 'skillshare sync' to deploy
```

### Create in a project

```bash
skillshare new code-review -p
```

Output:
```
New Skill Created (project)
─────────────────────────────────────────────
✓ Created: .skillshare/skills/code-review/SKILL.md

Next steps:
  1. Edit .skillshare/skills/code-review/SKILL.md
  2. Run 'skillshare sync' to deploy
```

### Preview before creating

```bash
skillshare new my-skill --dry-run
```

Output:
```
New Skill (dry-run)
─────────────────────────────────────────────
→ Would create: ~/.config/skillshare/skills/my-skill
→ Would write: ~/.config/skillshare/skills/my-skill/SKILL.md

Template preview:
─────────────────────────────────────────────
---
name: my-skill
description: >-
  Describe what this skill does. Use when user asks to ...
---
...
```

---

## Next Steps

After creating a skill:

1. **Edit the SKILL.md** — Focus on the `description` field first (WHAT + WHEN)
2. **Add instructions** — Use step-based format with clear actions
3. **Sync to targets** — `skillshare sync`
4. **Test triggering** — Ask your AI CLI a related question and check if the skill loads
5. **Iterate** — Refine trigger phrases based on over/under-triggering

---

## See Also

- [install](/docs/reference/commands/install) — Install skills from repos
- [sync](/docs/reference/commands/sync) — Sync skills to targets
- [Configuration](/docs/reference/targets/configuration) — Config reference
