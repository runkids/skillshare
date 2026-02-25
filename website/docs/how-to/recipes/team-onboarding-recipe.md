---
sidebar_position: 6
---

# Recipe: Team Onboarding

> Set up a new team member's AI skill environment in under 5 minutes.

## Scenario

A new developer joins your team. They need:
- Organization-wide skills (coding standards, review guidelines)
- Project-specific skills (domain knowledge, architecture rules)
- Everything working across their AI tools (Claude Code, Cursor, etc.)

## Solution

### Step 1: Create an onboarding script

Save as `scripts/setup-skills.sh` in your team wiki or repo:

```bash
#!/bin/bash
set -e

echo "Installing skillshare..."
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh

echo "Initializing..."
skillshare init

echo "Installing organization skills..."
skillshare install your-org/org-skills

echo "Running security audit..."
skillshare audit

echo "Syncing to all AI tools..."
skillshare sync

echo "Done! Run 'skillshare list' to see installed skills."
```

### Step 2: New hire runs the script

```bash
curl -fsSL https://your-org.github.io/setup-skills.sh | sh
```

Or if the script is in the team repo:

```bash
git clone your-org/team-tools
./team-tools/scripts/setup-skills.sh
```

### Step 3: Project-specific setup

When the new hire clones a project:

```bash
cd your-project
skillshare sync -p
```

This picks up project-scoped skills automatically.

### Step 4: Verify everything works

```bash
# Check global skills
skillshare list

# Check project skills
skillshare list -p

# Check sync status
skillshare status
```

## Verification

- `skillshare list` shows organization skills
- `skillshare status` shows all targets are synced
- Opening Claude Code / Cursor shows skills are loaded

## Variations

- **Dev container onboarding**: If your team uses dev containers, add skillshare to `.devcontainer/Dockerfile` and `postCreateCommand` — skills are ready when the container starts
- **Homebrew-based install**: Replace `curl | sh` with `brew install skillshare` for macOS/Linux teams
- **Hub discovery**: Point new hires to your hub: `skillshare search --hub https://your-org.github.io/skillshare-hub.json`

## Related

- [Getting started guide](/docs/getting-started)
- [Organization sharing](/docs/how-to/sharing/organization-sharing)
- [Project setup](/docs/how-to/sharing/project-setup)
- [Dev container guide](/docs/learn/with-devcontainer)
