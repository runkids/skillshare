---
slug: team-sharing-patterns
title: 3 Patterns for Team Skill Sharing
authors: [runkids]
tags: [team, tutorial]
---

Every team eventually faces the question: "How do we share AI skills across the team?" Here are three patterns, from simplest to most structured, with skillshare handling the mechanics.

<!-- truncate -->

## Pattern 1: Shared Git Repository

**Best for:** Small teams (2-5 people) who want a quick start.

Create a Git repository with your team's skills:

```
team-skills/
├── code-review/
│   └── SKILL.md
├── pr-description/
│   └── SKILL.md
└── testing-standards/
    └── SKILL.md
```

Each team member installs it:

```bash
skillshare install your-org/team-skills
skillshare sync
```

When someone updates a skill, others pull changes:

```bash
skillshare check              # "team-skills: 2 skills updated"
skillshare update --all       # Apply all available updates
skillshare sync
```

**Pros:** Simple, familiar Git workflow, works with any Git host.

**Cons:** Everyone gets all skills, no per-project customization.

## Pattern 2: Organization-Wide + Project-Scoped

**Best for:** Medium teams (5-20 people) with different projects.

Split skills into two layers:

### Organization Layer (global)

Shared standards that apply everywhere:

```bash
skillshare install your-org/org-skills
```

These live in `~/.config/skillshare/skills/` and sync to all tools on every machine.

### Project Layer (scoped)

Project-specific skills committed to the project repo:

```bash
cd your-project
skillshare init -p
skillshare install your-org/frontend-skills -p
git add .skillshare/
git commit -m "Add project skills"
```

Team members get project skills automatically when they clone and run:

```bash
skillshare sync -p
```

**Pros:** Organization standards + project flexibility. Skills travel with the code.

**Cons:** Two sync commands (global + project). Requires project setup.

## Pattern 3: Hub Index (Registry)

**Best for:** Large teams or open-source communities.

Generate a hub index from your source skills:

```bash
skillshare hub index --source ./skills --output ./skillshare-hub.json
```

This creates a JSON file listing all available skills with metadata. Host it on GitHub Pages or any HTTPS URL, then register it:

```bash
skillshare hub add https://your-org.github.io/skillshare-hub.json --label team
skillshare hub default team
```

Team members can then browse and install:

```bash
skillshare search --hub https://your-org.github.io/skillshare-hub.json
skillshare install your-org/skills --skill code-review
```

**Pros:** Discoverable, self-service, scales to hundreds of skills.

**Cons:** Requires maintaining the index. More setup upfront.

## Choosing a Pattern

| Factor | Pattern 1 | Pattern 2 | Pattern 3 |
|--------|-----------|-----------|-----------|
| Setup time | 5 min | 15 min | 30 min |
| Team size | 2-5 | 5-20 | 20+ |
| Per-project skills | No | Yes | Yes |
| Self-service discovery | No | No | Yes |
| Maintenance | Low | Medium | Medium |

Most teams start with Pattern 1 and evolve to Pattern 2 when they need project-specific skills. Pattern 3 is for organizations that want a curated skill catalog.

## Getting Started

Regardless of pattern, the workflow is the same:

```bash
skillshare install <repo>    # Add skills
skillshare sync              # Push to tools
skillshare check             # Detect updates
skillshare update --all      # Apply updates
```

## Resources

- [Organization sharing guide](/docs/how-to/sharing/organization-sharing)
- [Project setup guide](/docs/how-to/sharing/project-setup)
- [Hub index guide](/docs/how-to/sharing/hub-index)
