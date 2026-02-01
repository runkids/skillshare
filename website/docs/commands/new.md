---
sidebar_position: 4
---

# new

Create a new skill with a SKILL.md template.

```bash
skillshare new <name>            # Create a new skill
skillshare new <name> --dry-run  # Preview without creating
```

**What happens:**
```
┌─────────────────────────────────────────────────────────────────┐
│ skillshare new my-skill                                         │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. Validate skill name                                          │
│    → lowercase, numbers, hyphens only                           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Create skill directory                                       │
│    → ~/.config/skillshare/skills/my-skill/                      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Generate SKILL.md template                                   │
│    → ~/.config/skillshare/skills/my-skill/SKILL.md              │
└─────────────────────────────────────────────────────────────────┘
```

---

## Options

| Flag | Description |
|------|-------------|
| `--dry-run`, `-n` | Preview without creating files |
| `--help`, `-h` | Show help |

---

## Skill Name Rules

- Lowercase letters, numbers, hyphens, underscores
- Must start with a letter or underscore
- Examples: `my-skill`, `code_review`, `pdf-tools`

---

## Template Structure

The generated SKILL.md follows this format:

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
description: Brief description of what this skill does
---
...
```

---

## Next Steps

After creating a skill:

1. **Edit the SKILL.md** — Add your instructions
2. **Sync to targets** — `skillshare sync`
3. **Test in your AI CLI** — Use `/skill:my-skill` or mention it

---

## Related

- [install](/docs/commands/install) — Install skills from repos
- [sync](/docs/commands/sync) — Sync skills to targets
- [configuration](/docs/guides/configuration) — Config reference
