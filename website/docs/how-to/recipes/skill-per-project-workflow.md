---
sidebar_position: 4
---

# Recipe: Project Mode Workflow

> Manage project-scoped skills that travel with your codebase.

## Scenario

You want specific skills committed to your project repository so that:
- Every contributor gets the same AI instructions
- Skills are versioned alongside the code
- No manual setup beyond cloning the repo

## Solution

### Step 1: Initialize project mode

```bash
cd your-project
skillshare init -p
```

This creates `.skillshare/config.yaml` in your project root.

### Step 2: Install project-scoped skills

```bash
skillshare install anthropics/courses/prompt-eng -p
skillshare install your-org/team-skills --skill code-review -p
```

Skills are placed in `.skillshare/skills/`.

### Step 3: Sync to project targets

```bash
skillshare sync -p
```

This creates symlinks from `.skillshare/skills/` into project-level target directories (e.g., `.claude/skills/`, `.cursor/skills/`).

### Step 4: Commit to version control

```bash
git add .skillshare/
git commit -m "Add project skills"
```

### Step 5: Teammate setup

When a teammate clones the repo:

```bash
git clone your-org/your-project
cd your-project
skillshare sync -p
```

One command syncs all project skills to their local AI tools.

## Verification

- `.skillshare/config.yaml` exists in project root
- `.skillshare/skills/` contains installed skills
- `skillshare list -p` shows project skills
- After `sync -p`, target directories contain symlinks

## Variations

- **Dev container auto-sync**: Add `skillshare sync -p` to `.devcontainer/devcontainer.json` `postCreateCommand`
- **Mixed mode**: Use global skills for personal preferences + project skills for team standards
- **CI validation**: Add `skillshare audit -p` to CI pipeline to validate project skills

## Related

- [Project setup guide](/docs/how-to/sharing/project-setup)
- [Understanding project skills](/docs/understand/project-skills)
- [Dev container guide](/docs/learn/with-devcontainer)
