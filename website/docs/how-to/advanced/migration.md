---
sidebar_position: 5
---

# Migration

Migrate from other skill management approaches to skillshare.

## From Manual Management

If you've been manually copying skills between AI CLIs:

### Step 1: Initialize skillshare

```bash
skillshare init
```

### Step 2: Collect existing skills

```bash
# Collect from each AI CLI
skillshare collect claude
skillshare collect pi
skillshare collect codex

# Or collect from all at once
skillshare collect --all
```

### Step 3: Handle duplicates

If the same skill exists in multiple places, `collect` warns you. Choose which to keep.

### Step 4: Sync

```bash
skillshare sync
```

Now all targets are symlinked to your single source.

---

## From Other Install Tools

If you've used `npx install-skill` or similar:

### Step 1: Initialize skillshare

```bash
skillshare init
```

### Step 2: Backup existing skills

```bash
skillshare backup
```

### Step 3: Collect or reinstall

**Option A: Collect existing** (keeps current versions)
```bash
skillshare collect --all
```

**Option B: Reinstall from source** (gets latest versions)
```bash
# Check the metadata
cat ~/.config/skillshare/skills/.metadata.json

# Reinstall
skillshare install anthropics/skills/skills/pdf
```

### Step 4: Sync

```bash
skillshare sync
```

---

## From `npx skills` {#from-npx-skills}

You can keep using `npx skills` for skills it already manages, or hand selected skills over to Skillshare. Both tools support multiple agents and global/project installs; migration is useful when you want Skillshare's broader resource management, not because the other tool can only install. See the [comparison](/docs/understand/philosophy/comparison).

**There is no automatic npx lock import or ownership transfer.** Collecting preserves local skill files, not their upstream tracking. Reinstalling from a repository creates Skillshare tracking, but may fetch different content from your existing installation.

The paths and lock fields below refer to **skills v1.8.1**: [global lock source](https://github.com/vercel-labs/skills/blob/e781cb4d07433dfe1d19e924ed45cc61738c55ec/src/skill-lock.ts), [project lock source](https://github.com/vercel-labs/skills/blob/e781cb4d07433dfe1d19e924ed45cc61738c55ec/src/local-lock.ts), and [CLI reference](https://github.com/vercel-labs/skills/blob/e781cb4d07433dfe1d19e924ed45cc61738c55ec/docs/commands.md).

| Item | Global | Project |
|------|--------|---------|
| Shared skill location | `~/.agents/skills/<name>/` | `.agents/skills/<name>/` |
| Agent directories | Usually links to the shared skills; `--copy` creates independent copies | Same |
| Lock file | `$XDG_STATE_HOME/skills/.skill-lock.json` when set; otherwise `~/.agents/.skill-lock.json` | `skills-lock.json` |

Skills linked by `experimental_sync` can point into npm dependencies rather than being real directories. Inspect the actual paths before collecting.

### Step 1: Decide scope and preserve the original setup

The commands in Steps 2–5 explicitly use **global mode** (`-g`). For project installs, use the project notes below instead.

Before initialization or sync:

- Decide which skills each manager will own. Pause updates, removals, and automatic sync jobs while migrating.
- Make a separate snapshot of the original skill directories, their real contents, and their symlink layout. Include any existing Skillshare source you might replace.
- Save the applicable npx lock file separately. For npm projects, also preserve `package.json` and the package-manager lock file. These record sources and dependencies, not a backup of local edits.

Keep this snapshot until you have verified the handover. Do not delete the old lock or run `npx skills remove` as a cleanup shortcut.

### Step 2: Initialize without copying, then check backups

If Skillshare is not initialized, run the following in an **interactive terminal with both stdin and stdout attached**. Do not pipe or automate this command: these flags do not disable initial sync, and non-interactive initialization syncs automatically when targets are selected.

```bash
skillshare init -g --no-copy --no-skill --no-git
```

Select the targets you intend to manage, including `universal` for `~/.agents/skills`, and use merge mode for the workflow below. Decline an initial sync if prompted. For an existing configuration, inspect its source, [targets](/docs/reference/commands/target), and modes instead of resetting it.

```bash
skillshare backup -g --dry-run
skillshare backup -g
skillshare backup -g --list
```

Check the output for failures and confirm that the real skill files are covered. Target backups **skip symlinks**, do not back up their referents, and do not include the npx lock outside the target directory. They supplement, not replace, the snapshot from Step 1.

### Step 3: Choose local copies or upstream tracking

**Keep the current files:** preview and collect from the target holding real directories:

```bash
skillshare collect universal -g --dry-run
skillshare collect universal -g
```

This collects all eligible skills in that target, not one selected skill. For copies in an individual agent directory, substitute that configured target name. Use `collect --all -g` only if you intend to collect from every configured target. For a partial handover, copy only the chosen real skill directories into Skillshare's source instead.

Collection skips skill symlinks, including links into `node_modules`. For those, keep npm management or deliberately copy the resolved skill directory as a local snapshot; that copy will no longer follow package updates. Collection also skips names already present in the source unless forced. Inspect skipped/failed results and compare duplicate contents yourself—the preview is not a lock-aware conflict report. Do not use `collect --force` to choose a winner blindly.

Collected skills have no imported repository tracking, so `skillshare update` cannot update them from their original sources.

**Keep upstream updates:** inspect each saved lock entry before installing from its source. `sourceType` distinguishes Git, npm, local, and other sources; `sourceUrl`, `ref`, and `skillPath` may provide additional information. Not every entry has a repository or `skillPath`.

For example, a GitHub entry with `source: "anthropics/skills"` and `skillPath: "skills/pdf/SKILL.md"` maps to the containing skill directory:

```bash
skillshare install anthropics/skills/skills/pdf -g --dry-run
skillshare install anthropics/skills/skills/pdf -g
```

If the lock records a Git ref, add `--branch <recorded-ref>` to both commands above (see [install](/docs/reference/commands/install)). Without it, the example uses the remote default branch. This is a fresh installation, not a replay of the npx content hash. Compare it with your saved files and reconcile local edits. If a collected skill already occupies the destination, stop and resolve it first; do not blindly force an install, which can overwrite edits and override audit blocking. npm and local entries need their own source decision, not an `owner/repo` conversion.

### Step 4: Preview and explicitly hand over ownership

```bash
skillshare sync -g --dry-run
```

Review **all configured targets**, not just `universal`. In merge mode, same-name local directories are preserved, while same-name agent symlinks can be repointed to Skillshare's source even without force. Confirm that this affects only skills you intend to hand over before applying:

```bash
skillshare sync -g
```

If you also want the preserved real directories replaced by links, first verify the source contains the versions you want and the original snapshot is complete:

```bash
skillshare sync -g --force --dry-run
# Apply only after reviewing every replacement:
skillshare sync -g --force
```

Force applies across the configured targets; it is not a per-skill migration switch. Stop if the preview includes resources still owned by npx skills. Keep the tools' managed skill sets separate; see [coexistence](/docs/troubleshooting/faq#using-universal-alongside-npx-skills).

### Step 5: Verify and keep one owner

Check the resulting files/links and confirm the intended skills load in each AI tool. Run `skillshare status -g` and review preserved local skills or warnings; a successful sync alone does not prove every original skill was transferred.

Skillshare does not remove the old npx lock entries. After handover, do not use `npx skills update`, `remove`, `experimental_install`, or `experimental_sync` on the migrated skills. For partial migrations, avoid unqualified bulk commands that include them.

To roll back, pause both managers and restore the saved original files, link layout, and lock together. Skillshare target backups can help recover local files, but cannot recreate the original npx symlinks or lock by themselves.

### Project installs and npm synchronization

Preserve the project's files and lock before changes, including uncommitted or ignored local edits. Initialize with `skillshare init -p`, configure the project targets, then use `collect`, `install`, and `sync` with `-p` rather than the global commands above. The same source-tracking and symlink limitations apply. Skillshare does not create skill-target backups in project mode; use a verified project snapshot or version control that actually includes the files.

If `experimental_sync` is called from `package.json` scripts or CI, adjust those jobs and skill declarations before handing over overlapping skills. Otherwise a later dependency installation can relink them. Keeping npm-managed skills with npx skills is valid; do not copy links into `.skillshare/skills/` and assume they have become independently managed.

---

## From Git Submodules

If you've been using git submodules:

### Step 1: Export submodule contents

```bash
# In your existing skills repo
git submodule foreach 'cp -r $toplevel/$sm_path ~/temp-skills/$name'
```

### Step 2: Initialize skillshare

```bash
skillshare init
```

### Step 3: Import skills

```bash
# Copy to source
cp -r ~/temp-skills/* ~/.config/skillshare/skills/

# Or install as tracked repos
skillshare install github.com/org/skill-repo --track
```

### Step 4: Sync

```bash
skillshare sync
```

---

## From Committed Project Skills

If your repo already has skills committed in `.claude/skills/`, `.cursor/skills/`, or similar directories:

### Step 1: Initialize project mode

```bash
cd my-project
skillshare init -p
```

### Step 2: Move skills to `.skillshare/skills/`

```bash
# Copy existing skills to skillshare source
cp -r .claude/skills/my-skill .skillshare/skills/
cp -r .claude/skills/api-guide .skillshare/skills/

# Remove originals (sync will recreate as symlinks)
rm -rf .claude/skills/my-skill .claude/skills/api-guide
```

### Step 3: Sync

```bash
skillshare sync
```

Now `.claude/skills/my-skill` is a symlink to `.skillshare/skills/my-skill` — and all other targets (Pi, Windsurf, etc.) get the same skills automatically.

### Step 4: Commit the migration

```bash
git add .skillshare/ .claude/skills/ .cursor/skills/
git commit -m "Migrate project skills to skillshare"
```

:::tip Multi-Tool Benefit
Before: skills only worked in one AI CLI. After: the same skills are automatically available in every configured target.
:::

---

## From Team-Specific Solutions

If your team has custom skill sharing:

### Step 1: Identify current approach

- Where are skills stored?
- How are they shared?
- How are they updated?

### Step 2: Choose your migration path

**Option A: Global mode** — skills available across all projects on each machine.

```bash
# Create team skills repo
cp -r /current/team/skills ~/new-team-skills
cd ~/new-team-skills && git init && git add . && git commit -m "Migrate to skillshare"
git push origin main

# Team members install globally
skillshare install github.com/org/team-skills --track && skillshare sync
```

**Option B: Project mode** — skills scoped to a specific repo, shared via git.

```bash
cd my-project
skillshare init -p

# Move team skills into project source
cp -r /current/team/skills/* .skillshare/skills/

# Sync and commit
skillshare sync
git add .skillshare/
git commit -m "Add team skills via skillshare"
```

New team members get everything with:
```bash
git clone github.com/org/my-project
cd my-project
skillshare install -p && skillshare sync
```

**Option C: Both** — organization-wide standards globally, project-specific skills per-repo.

```bash
# Organization standards (global)
skillshare install github.com/org/standards --track && skillshare sync

# Project-specific skills (project mode)
cd my-project
skillshare init -p
skillshare install github.com/org/project-skills -p && skillshare sync
```

:::tip Which to Choose?
- **Global**: coding standards, security audits — things every project needs
- **Project**: API conventions, domain rules, deployment guides — things specific to one repo
- **Both**: most teams end up here as they grow
:::

---

## From Global to Project

If you have skills in global mode that belong to a specific project:

### Step 1: Initialize project mode

```bash
cd my-project
skillshare init -p
```

### Step 2: Copy skills from global source

```bash
# Copy specific skills
cp -r ~/.config/skillshare/skills/api-guide .skillshare/skills/
cp -r ~/.config/skillshare/skills/deploy-rules .skillshare/skills/
```

### Step 3: Remove from global (optional)

```bash
skillshare uninstall api-guide
skillshare uninstall deploy-rules
skillshare sync   # Clean up global symlinks
```

### Step 4: Sync and commit

```bash
skillshare sync   # Auto-detects project mode
git add .skillshare/
git commit -m "Move project-specific skills to project mode"
```

After this, the skills are scoped to this repo and shared with the team via git — no longer cluttering your global setup.

---

## Preserving History

If you want to keep git history:

### For personal skills

```bash
# Clone your existing repo to skillshare location
git clone your-existing-repo ~/.config/skillshare/skills

# Initialize skillshare with existing source
skillshare init --source ~/.config/skillshare/skills
```

### For team repos

```bash
# Use --track to preserve .git
skillshare install github.com/team/skills --track
```

---

## Rollback

If migration goes wrong:

### Restore from backup

```bash
skillshare restore claude
skillshare restore cursor
```

### Start fresh

```bash
rm ~/.config/skillshare/config.yaml
skillshare init
```

---

## Checklist

Before migrating:

- [ ] List all current skill locations
- [ ] Identify duplicates
- [ ] Note any custom configurations
- [ ] Create backups

After migrating:

- [ ] Verify all skills appear in `skillshare list`
- [ ] Test skills in each AI CLI
- [ ] Set up git remote (if desired)
- [ ] Share new workflow with team

---

## See Also

- [From Existing Skills](/docs/getting-started/from-existing-skills) — Quick migration path
- [collect](/docs/reference/commands/collect) — Collect from targets
- [Comparison](/docs/understand/philosophy/comparison) — Compare approaches
