---
sidebar_position: 3
---

# From Existing Skills

You already have skills scattered across `~/.claude/skills/`, `~/.cursor/skills/`, or other AI CLI directories. This guide consolidates them into a single source and replaces the originals with symlinks.

```text
BEFORE                                  AFTER
─────────────────────────────────────────────────────────────────
~/.claude/skills/                       Source (one source of truth)
  ├── skill-a/                          ~/.config/skillshare/skills/
  └── skill-b/                            ├── skill-a/
                                          ├── skill-b/
~/.cursor/skills/                         ├── skill-c/
  ├── skill-b/  (duplicate!)              └── skill-d/
  └── skill-c/
                                        Targets (symlinked back)
~/.codex/skills/                        ~/.claude/skills/ → source
  └── skill-d/                          ~/.cursor/skills/ → source
                                        ~/.codex/skills/  → source
```

:::caution Back up first
`collect` mutates target directories — local skills get replaced with symlinks. Always run `skillshare backup` before collecting so `skillshare restore <target>` can undo it if anything looks wrong afterwards.
:::

## Which path applies

| Your situation | Path |
|---|---|
| Skills live in one CLI only | [Single-CLI migration](#single-cli-migration) |
| Skills are spread across multiple CLIs | [Multi-CLI consolidation](#multi-cli-consolidation) |
| You already have a skills git repo elsewhere | [Connect an existing repo](#connect-an-existing-repo) |

---

## Single-CLI migration

If every skill lives in a single target (say, Claude), `init --copy-from` handles it in one step:

```bash
skillshare init --copy-from claude
skillshare sync
```

`--copy-from claude` copies every skill from `~/.claude/skills/` into source during init. The subsequent `sync` replaces the originals with symlinks pointing back at source.

---

## Multi-CLI consolidation

Skills are scattered across multiple targets. Initialize empty, snapshot, then `collect` from each target.

```bash
# 1. Initialize empty
skillshare init --no-copy

# 2. Snapshot every target before mutating it
skillshare backup

# 3. Collect — once for everything, or per target
skillshare collect --all
#   or:
#   skillshare collect claude
#   skillshare collect cursor

# 4. Sync — targets now symlink back to source
skillshare sync
```

What `collect` does to each target:

1. Copies non-symlinked local skills into source (skips any `.git/` inside a skill).
2. Replaces the originals with symlinks pointing back at source.
3. Detects duplicates (the same skill name appearing in multiple targets) and reports them without overwriting.

### Resolving duplicates

When a skill exists in source and in a target you're collecting from, the target version is skipped and reported:

```
Warning: skill-b exists in source
  Source:  ~/.config/skillshare/skills/skill-b/
  Skipped: ~/.cursor/skills/skill-b/
```

Resolve by hand: diff the two copies, keep whichever you want in source, then either leave the target version alone (it'll be replaced by the symlink on next `sync`) or re-run `collect --force` if the target version is the one you'd rather keep.

---

## Connect an existing repo

If you already have a skills repo on GitHub (perhaps from a previous machine), don't `collect` — just clone it:

```bash
skillshare init --remote git@github.com:you/skills.git --all-targets --no-skill
skillshare sync
```

Tracked dependencies are gitignored and won't come down with the clone. Re-install them after init:

```bash
skillshare install https://github.com/your-company/skills --track --force
skillshare sync
```

---

## Push your migrated source to git

After migration, get source under version control so future machines can recover it the same way.

```bash
# Skip this if you already passed --remote during init.
cd ~/.config/skillshare/skills
git remote add origin git@github.com:you/skills.git

skillshare push -m "Initial commit: migrated skills"
```

From then on, `skillshare push` and `skillshare pull` move skills between machines.

---

## Verify

```bash
skillshare status     # every target should report 'synced'
skillshare list       # all collected skills should appear
skillshare doctor     # diagnostics — broken symlinks, missing targets, etc.
```

## Rollback

Because you ran `backup` first, `collect` is reversible:

```bash
skillshare restore claude
skillshare restore cursor
```

Each target returns to its pre-collect state — real files, no symlinks.

---

## See Also

- [Daily Workflow](/docs/how-to/daily-tasks/daily-workflow) — day-to-day after migration
- [Cross-Machine Sync](/docs/how-to/sharing/cross-machine-sync) — sync via git
- [Core Concepts](/docs/understand) — how source and targets relate
