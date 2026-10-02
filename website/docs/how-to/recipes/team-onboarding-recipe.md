---
sidebar_position: 6
---

# Recipe: Team Onboarding

> Give new teammates the team's shared skills and project context.

## Scenario

A new developer joins your team. They need:
- Organization-wide skills (coding standards, review guidelines)
- Project-specific skills (domain knowledge, architecture rules)
- Everything working across their AI tools (Claude Code, Pi, etc.)

One teammate uses Claude Code, another uses Codex, and the new hire uses Pi. All three need a review checklist that says which legacy API to avoid. Copying that checklist into each tool creates separate versions that drift as the project changes.

Keep the project's local skills, `.skillshare/config.yaml` and `.skillshare/skills.lock.json` in the project repository. The config declares remote skills and targets; the lockfile records remote skill commits. Each teammate applies those files locally. Shared instructions give the team common context, while each tool retains its own permissions and behavior.

## Solution

### Step 1: Create an onboarding script

Save as `scripts/setup-skills.sh` in your team wiki or repo:

```bash
#!/bin/bash
set -e

echo "Installing skillshare..."
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"

echo "Initializing..."
skillshare init -g

echo "Installing organization skills..."
skillshare install github.com/your-org/org-skills --track -g

echo "Running security audit..."
skillshare audit -g --threshold high

echo "Syncing to all AI tools..."
skillshare sync -g

echo "Done! Run 'skillshare list -g' to see installed skills."
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

The maintainer first follows [Project Setup](/docs/how-to/sharing/project-setup) and commits the project config, local skills and lockfile. When the new hire clones that project:

```bash
cd your-project
skillshare install -p
skillshare audit -p --threshold high
skillshare sync -p
```

`install -p` installs the remote skills declared in `.skillshare/config.yaml`, using locked commits when present. `audit -p` reviews the project skills, including committed local skills. `sync -p` distributes them to the configured targets. Run these commands in order and stop if a step fails; a blocked audit needs review before syncing.

After pulling project updates, repeat this sequence. Git transfers the configuration and lockfile; it does not install missing remote skills or refresh target copies by itself. Review intentional skill updates in a PR and commit the resulting lockfile changes.

### Step 4: Verify everything works

```bash
# Check global skills
skillshare list -g

# Check project skills
skillshare list -p

# Check sync status
skillshare status -p
```

## Verification

- `skillshare list -g` shows organization skills
- `skillshare list -p` shows the project's local and installed remote skills
- `skillshare status -p` shows the configured project targets are synced
- Opening a configured AI tool shows the expected skills; try the review checklist against a known legacy-API example

## Variations

- **Dev container onboarding**: If your team uses dev containers, add skillshare to `.devcontainer/Dockerfile` and `postCreateCommand` — skills are ready when the container starts
- **Homebrew-based install**: Replace `curl | sh` with `brew install skillshare` for macOS/Linux teams
- **Hub discovery**: Point new hires to your hub: `skillshare search --hub https://your-org.github.io/skillshare-hub.json`

## Related

- [Getting started guide](/docs/getting-started)
- [Organization sharing](/docs/how-to/sharing/organization-sharing)
- [Project setup](/docs/how-to/sharing/project-setup)
- [Dev container guide](/docs/learn/with-devcontainer)
