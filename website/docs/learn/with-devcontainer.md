---
sidebar_position: 5
---

# Using skillshare in Dev Containers

> Open in VS Code, skills are ready — no local install needed.

## Prerequisites

- [VS Code](https://code.visualstudio.com/) with [Dev Containers extension](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers)

## How It Works

VS Code Dev Containers let you develop inside a Docker container. You define the environment in `.devcontainer/`, and VS Code handles the rest — open the project, click "Reopen in Container", and everything is ready.

skillshare fits naturally into this workflow. Add it to `postCreateCommand` and skills are installed and synced when the container starts.

## Setup

Add two things to your `.devcontainer/devcontainer.json`:

```json
{
  "postCreateCommand": "curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh && skillshare init --no-copy --all-targets --no-skill && skillshare sync"
}
```

That's it. When a team member opens the project in VS Code and clicks "Reopen in Container":

1. skillshare is installed automatically
2. `init` runs non-interactively — adds all detected AI CLI targets, skips copy prompts and built-in skill installation
3. `sync` delivers skills to all targets

## Adding Project Skills

For team-shared skills, commit a `.skillshare/` config to the repo:

```bash
# Inside the container
skillshare init -p
skillshare install your-org/team-skills -p
```

Then commit and update `postCreateCommand` to also sync project skills:

```json
{
  "postCreateCommand": "curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh && skillshare init --no-copy --all-targets --no-skill && skillshare sync && skillshare sync -p"
}
```

Now every team member gets the same skills when they open the container.

## GitHub Codespaces

The same `.devcontainer/` config works in Codespaces with no changes. Codespaces runs `postCreateCommand` the same way VS Code does.

## What's Next?

- [Project skill setup →](/docs/how-to/sharing/project-setup)
- [Team sharing →](/docs/how-to/sharing/organization-sharing)
- [Sync modes explained →](/docs/understand/philosophy/sync-modes-explained)
