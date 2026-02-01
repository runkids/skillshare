---
sidebar_position: 6
---

# Best Practices

Naming conventions, organization, and version control for skills.

## Naming

### Skill names

**Do:**
- Use lowercase with hyphens: `code-review`, `pdf-tools`
- Be descriptive: `react-component-generator` not `rcg`
- Namespace for teams: `acme:code-review`

**Don't:**
- Use spaces or special characters
- Use generic names: `helper`, `utils`, `tools`
- Conflict with common skill names

### Repository names

**For personal:**
```
my-skills
ai-skills
```

**For teams:**
```
<team>-skills
<org>-skills
```

---

## Organization

### Personal skills

```
~/.config/skillshare/skills/
├── code-review/
├── pdf-tools/
├── git-workflow/
└── _team-skills/      # Tracked repo
```

### Team repos

```
team-skills/
├── frontend/
│   ├── react/
│   ├── vue/
│   └── testing/
├── backend/
│   ├── api/
│   └── database/
├── devops/
│   ├── deploy/
│   └── monitoring/
└── README.md
```

### Skill directory

```
my-skill/
├── SKILL.md           # Required
├── README.md          # Optional: for humans
├── examples/          # Optional: example files
└── templates/         # Optional: code templates
```

---

## Version Control

### Commit messages

Follow conventional commits:
```
feat(code-review): add security check
fix(pdf-tools): handle empty files
docs(readme): update installation
```

### Branching

**For personal:**
- Single `main` branch is fine
- Use branches for experiments

**For teams:**
- `main` for stable skills
- Feature branches for development
- PR review before merge

### Tags

Tag stable releases:
```bash
git tag v1.0.0
git push --tags
```

---

## Skill Writing

### Structure

```markdown
---
name: skill-name
description: One-line description
---

# Skill Name

Brief overview.

## When to Use

Clear trigger conditions.

## Instructions

1. Step one
2. Step two

## Examples

Concrete input/output examples.

## When NOT to Use

Explicit exclusions.
```

### Content

**Do:**
- Write clear, actionable instructions
- Include examples
- Specify edge cases
- Keep it focused (one skill = one purpose)

**Don't:**
- Write vague instructions
- Include too many responsibilities
- Forget error handling
- Skip testing

---

## Team Collaboration

### Ownership

- Assign owners to skill categories
- Document in README who maintains what
- Review PRs before merging

### Documentation

```
team-skills/
├── README.md           # Setup instructions
├── CONTRIBUTING.md     # How to add skills
├── CHANGELOG.md        # What changed
└── skills/
    └── ...
```

### Communication

- Announce new skills in team chat
- Document breaking changes
- Gather feedback from users

---

## Maintenance

### Regular tasks

```bash
# Weekly
skillshare update --all     # Update tracked repos
skillshare doctor           # Check for issues
skillshare backup --cleanup # Remove old backups

# Monthly
skillshare list             # Review installed skills
# Remove unused: skillshare uninstall <name>
```

### Cleanup unused skills

```bash
# List all skills
skillshare list

# Remove ones you don't use
skillshare uninstall unused-skill
skillshare sync
```

### Update dependencies

```bash
# Update CLI
skillshare upgrade --cli

# Update built-in skill
skillshare upgrade --skill

# Update tracked repos
skillshare update --all
```

---

## Security

### Sensitive information

**Never put in skills:**
- API keys
- Passwords
- Personal information
- Internal URLs

**Instead:**
- Use environment variables
- Reference external configs
- Keep skills generic

### Review before installing

Before installing third-party skills:
- Check the source
- Read the SKILL.md
- Use `--dry-run` first

---

## Checklist

### New skill

- [ ] Descriptive name
- [ ] Clear description
- [ ] Actionable instructions
- [ ] Examples included
- [ ] Tested in AI CLI
- [ ] No name conflicts

### Team repo

- [ ] Clear folder structure
- [ ] README with setup instructions
- [ ] Namespaced skill names
- [ ] PR review process
- [ ] CHANGELOG maintained

---

## Related

- [Creating Skills](./creating-skills) — Skill creation guide
- [Team Sharing](./team-sharing) — Team workflow
- [Skill Format](/docs/concepts/skill-format) — Format specification
