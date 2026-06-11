# skillshare v0.20.12 Release Notes

## TL;DR

1. **Droid syncs custom droids as agents** — the `droid` target now distributes custom droids alongside skills, and accepts `factory` as an alias.
2. **Project-mode agent symlinks are now relative** — agent symlinks created in project mode survive moving or re-checking out the repository.
3. **Factory alias and web UI agent filters now match CLI behavior** — `factory` resolves Droid's agents path, and dashboard sync respects target-level agent include/exclude filters.

## New feature: Droid custom droids sync as agents

The `droid` target already synced skills to `~/.factory/skills`. It now also syncs custom droids — `.md` files with YAML frontmatter — through the existing agents sync, mapping them to `~/.factory/droids` (global) and `.factory/droids` (project). Project-level droids override personal ones, matching Droid's own resolution.

The target is also reachable by its brand name via the new `factory` alias:

```bash
skillshare target add factory   # same as: skillshare target add droid
skillshare sync agents
```

Refs: #213.

## Bug fix: project-mode agent symlinks are now relative

`skillshare sync agents` created absolute symlinks in project mode, which broke when the repository was moved to a new path or checked out on another machine. Agent symlinks now use relative paths, matching how project skill symlinks already work, so a synced project stays portable.

## Bug fix: Factory alias syncs Droid agents correctly

Adding the Droid target by its `factory` alias now resolves both the skills path and the built-in agents path. `skillshare sync agents` writes custom droids to `~/.factory/droids` in global mode and `.factory/droids` in project mode instead of skipping `factory` as an agentless target.

## Bug fix: Web UI sync respects agent filters

Dashboard sync now honors target-level `agents.include` and `agents.exclude` filters before syncing agents, matching `skillshare sync` from the CLI. The same filtered agent set is used for pruning, so agents that become excluded are removed from target directories on the next sync.

Refs: #211.
