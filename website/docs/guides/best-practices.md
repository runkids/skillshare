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

### License

Add a `license` field for published skills — especially important for corporate environments:

```yaml
---
name: code-review
description: Reviews code for quality
license: MIT
---
```

This is displayed during `skillshare install` so users can make informed compliance decisions.

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

### Use project mode (`-p`) for repo-specific skills

When skills are tightly coupled to one codebase (architecture, domain rules, deployment flow), prefer project mode:

```bash
skillshare init -p
skillshare install <source> -p
skillshare sync
```

**Why this helps:**
- **Reproducible onboarding**: `.skillshare/config.yaml` acts as a portable skill manifest for anyone who clones the repo.
- **Clear scope**: project skills stay in `.skillshare/skills/` instead of leaking into global personal workflows.
- **Safer collaboration**: changes are reviewed through normal git PR flow with the project code.
- **Lower noise in commits**: `.skillshare/logs/` is ignored by default in project mode.

Use global mode for personal cross-project skills; use `-p` for repo-specific team context.

### Use .skillignore for internal tools

If your team repo contains internal tooling or work-in-progress skills, add a `.skillignore` to prevent accidental discovery:

```text title=".skillignore"
# Hide from public discovery
_internal-scripts
test-*
wip-feature
```

This ensures external contributors or automation running `skillshare install <repo> --all` won't pick up internal skills.

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
# Remove unused: skillshare uninstall <name>...
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
- [ ] `.skillignore` for internal tools
- [ ] PR review process
- [ ] CHANGELOG maintained

---

## See Also

- [Creating Skills](./creating-skills.md) — Skill creation guide
- [Skill Format](/docs/concepts/skill-format) — SKILL.md reference
- [Organization-Wide Skills](./organization-sharing.md) — Team sharing patterns
