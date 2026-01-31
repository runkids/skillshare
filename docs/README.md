# Skillshare Documentation

Welcome to the skillshare documentation. Choose a topic below.

## Quick Links

| I want to... | Go to |
|--------------|-------|
| Get started | [README](../README.md#quick-start) |
| **Search for skills** | [search.md](search.md) |
| Create a new skill | [new.md](new.md) |
| Install a skill | [install.md](install.md) |
| Sync skills to targets | [sync.md](sync.md) |
| Manage targets | [targets.md](targets.md) |
| Sync across machines | [cross-machine.md](cross-machine.md) |
| Share skills with team | [team-edition.md](team-edition.md) |
| Configure skillshare | [configuration.md](configuration.md) |
| Troubleshoot issues | [faq.md](faq.md) |

## Documentation Structure

```
docs/
├── README.md          ← You are here
├── search.md          ← Search and discover skills from GitHub
├── new.md             ← Create new skills from template
├── install.md         ← Install, update, upgrade, uninstall skills
├── sync.md            ← Sync, pull, push, backup, restore
├── targets.md         ← Add, remove, configure targets
├── cross-machine.md   ← Sync across multiple computers
├── team-edition.md    ← Tracked repos, nested skills, team sharing
├── configuration.md   ← Config file reference
└── faq.md             ← FAQ & troubleshooting
```

## Command Quick Reference

| Command | What it does | Docs |
|---------|--------------|------|
| `init` | First-time setup | [install.md](install.md#init) |
| `search` | Discover skills | [search.md](search.md) |
| `new` | Create a skill | [new.md](new.md) |
| `install` | Add a skill | [install.md](install.md#install) |
| `uninstall` | Remove a skill | [install.md](install.md#uninstall) |
| `update` | Update a skill | [install.md](install.md#update) |
| `upgrade` | Upgrade CLI/skill | [install.md](install.md#upgrade) |
| `sync` | Push to targets | [sync.md](sync.md#sync) |
| `pull` | Pull from target | [sync.md](sync.md#pull) |
| `push` | Push to git remote | [sync.md](sync.md#push) |
| `backup` | Backup targets | [sync.md](sync.md#backup) |
| `restore` | Restore from backup | [sync.md](sync.md#restore) |
| `target` | Manage targets | [targets.md](targets.md) |
| `list` | List skills | [install.md](install.md#list) |
| `status` | Show sync state | [sync.md](sync.md#status) |
| `diff` | Show differences | [sync.md](sync.md#diff) |
| `doctor` | Diagnose issues | [faq.md](faq.md#doctor) |

## Core Concepts

### Source vs Targets

```
┌─────────────────────────────────────────┐
│         SOURCE (edit here)              │
│   ~/.config/skillshare/skills/          │
│                                         │
│   my-skill/   another/   _team-repo/    │
└─────────────────────────────────────────┘
                    │
                    │ skillshare sync
                    ▼
┌─────────────────────────────────────────┐
│              TARGETS                    │
│   ~/.claude/skills/  (symlinks)         │
│   ~/.cursor/skills/  (symlinks)         │
│   ~/.codex/skills/   (symlinks)         │
└─────────────────────────────────────────┘
```

- **Source**: Single directory where you edit skills
- **Targets**: AI CLI skill directories (symlinked from source)
- **Sync**: Creates/updates symlinks from source to targets

### Sync Modes

| Mode | How it works |
|------|--------------|
| `merge` | Each skill symlinked individually. Local skills preserved. **(default)** |
| `symlink` | Entire directory is one symlink. All targets identical. |

See [sync.md](sync.md#sync-modes) for details.
