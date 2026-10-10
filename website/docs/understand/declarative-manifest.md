---
sidebar_position: 8
---

# Declarative Skill Manifest

Define your skill collection as code — install, share, and reproduce setups from a single manifest file.

:::tip When does this matter?
Use the declarative manifest when you want reproducible skill setups across machines, team onboarding with a single command, or open-source project bootstrap.
:::

## What Is a Skill Manifest?

A skill manifest is a **portable declaration** of your skill collection. Instead of manually installing skills one by one, you list them in a manifest file and run `skillshare install` to bring everything up.

The manifest location depends on the mode:

| Mode | Manifest location | Committable? |
|------|------------------|-------------|
| **Project** | `.skillshare/config.yaml` (`skills:` section) | Yes — commit to share with team |
| **Global** | `~/.config/skillshare/skills/.metadata.json` | No — personal machine state |

### Project Mode Manifest

In project mode, `skills:` lives in `config.yaml` alongside `targets:`:

```yaml
# .skillshare/config.yaml
targets:
  - claude
  - cursor

skills:
  - name: react-best-practices
    source: anthropics/skills/skills/react-best-practices
    group: frontend
  - name: _team-skills
    source: my-org/shared-skills
    tracked: true
  - name: commit
    source: anthropics/skills/skills/commit
```

This file is committed to git — teammates clone the repo and run `skillshare install -p` to install all listed skills.

The manifest records *what* to install. The exact commit each skill resolved to is recorded next to it in `.skillshare/skills.lock.json`, which is written automatically and should be committed too. With both files, `skillshare install -p` gives every teammate the same commit even after upstream moves on. See [Lockfile](./project-skills.md#lockfile).

### Global Mode Manifest

In global mode, skill records are stored in `.metadata.json` (the centralized metadata store). This file also contains runtime tracking data (hashes, timestamps) and is auto-managed.

## How It Works

### Install from Manifest

Running `skillshare install` with **no arguments** reads the manifest and installs all listed skills:

```bash
# Global mode — installs all skills from ~/.config/skillshare/skills/.metadata.json
skillshare install

# Project mode — installs all skills from .skillshare/config.yaml skills: section
skillshare install -p

# Preview without installing
skillshare install --dry-run
```

Skills that already exist are skipped automatically. In project mode, a skill whose installed commit differs from the lockfile is brought to the locked commit instead.

### Automatic Reconciliation

The manifest stays in sync with your actual skill collection:

- **`skillshare install <source>`** — adds the installed skill to the manifest automatically
- **`skillshare uninstall <name>...`** — removes the entry from the manifest automatically
- **`skillshare move <name>... <folder>`** — updates the entry's `group` (project mode) or moves its `.metadata.json` record (global mode)

In project mode, `config.yaml` and `skills.lock.json` are updated. In global mode, `.metadata.json` is updated. You never need to edit the manifest manually (though you can).

## Skill Entry Fields

Each entry in the `skills:` list has these fields:

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Skill name (directory name in source) |
| `source` | Yes | Install source (GitHub shorthand, HTTPS URL, SSH URL) |
| `tracked` | No | `true` for tracked repositories (preserves `.git`) |
| `group` | No | Subdirectory path (e.g. `frontend` or `frontend/vue`). Corresponds to `--into` during install. |

## Use Cases

### Personal Setup

Maintain your personal skill collection across machines:

```bash
# On machine A — skills are already installed and tracked in registry
skillshare push   # backup config + registry to git

# On machine B — fresh machine
skillshare pull   # restore config + registry from git
skillshare install  # install all skills from manifest
skillshare sync   # distribute to all targets
```

### Team Onboarding

New team members get the same AI context in one command:

```bash
# .skillshare/config.yaml skills: section is committed to the repo
git clone <project-repo>
cd <project-repo>
skillshare install -p   # installs all declared skills
skillshare sync -p      # links to project targets
```

### Open-Source Bootstrap

Project maintainers declare recommended skills in `config.yaml`:

```yaml
# .skillshare/config.yaml
targets:
  - claude
  - cursor

skills:
  - name: react-best-practices
    source: anthropics/skills/skills/react-best-practices
    group: frontend
  - name: commit
    source: anthropics/skills/skills/commit
```

:::info Group field and `--into`
When you install with `--into`, the group is recorded automatically:

```bash
skillshare install anthropics/skills/skills/pdf --into frontend -p
# config.yaml will contain: name: pdf, group: frontend
```

Running `skillshare install -p` (no args) recreates the same directory structure from the manifest.
:::

Contributors clone and run `skillshare install -p` to get project-specific AI context immediately.

## Workflow Summary

```
Project mode:
1. Install skills normally      →  config.yaml skills: auto-updates
2. Commit config.yaml and skills.lock.json via git  →  same skills, same commits for the team
3. Run `skillshare install -p`  →  reproduce on clone
4. Run `skillshare sync`        →  distribute to all targets

Global mode:
1. Install skills normally      →  .metadata.json auto-updates
2. Push/pull config via git     →  portable across machines
3. Run `skillshare install`     →  reproduce on new machine
4. Run `skillshare sync`        →  distribute to all targets
```

## Extras Configuration

In addition to skills, `config.yaml` can declare **extras** — non-skill resources (rules, commands, prompts) synced to separate directories. Extras are configured in the `extras:` section of `config.yaml` (both global and project):

```yaml
extras:
  - name: rules
    targets:
      - path: ~/.claude/rules
      - path: ~/.cursor/rules
        mode: copy
```

See [sync extras](/docs/reference/commands/sync#sync-extras) for details.

## Related

- [Install command](/docs/reference/commands/install) — `skillshare install` with and without arguments
- [Push/Pull](/docs/reference/commands/push) — backup and restore config via git
- [Project Skills](./project-skills.md) — project-level manifests
