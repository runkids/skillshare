---
sidebar_position: 4
---

# .skillfollow (Experimental)

Opt in to discovering skills through named first-level symlinks or Windows junctions in the skills source. Keep an external working repository where you normally edit it; skillshare reads it through a logical source path without taking ownership of its files.

## Setup

Use the configured **skills source root**: normally `~/.config/skillshare/skills/` (Windows: `%AppData%\skillshare\skills\`) or `.skillshare/skills/` in project mode. Custom `sources.skills` paths work too. These files do not apply to agents or extras, and are not read from nested repository roots.

For example, an external repository contains `review/SKILL.md` and `.git`, but no root `SKILL.md`:

```text
~/work/team-skills/                 # Real repository, outside source and Git staging tree
~/.config/skillshare/skills/
├── _team-skills -> ~/work/team-skills/
├── .skillfollow
└── .gitignore
```

Create the first-level link yourself. On macOS/Linux:

```bash
ln -s "$HOME/work/team-skills" "$HOME/.config/skillshare/skills/_team-skills"
```

On Windows, a directory junction can be created from Command Prompt without symlink privileges (replace the external path):

```text
mklink /J "%AppData%\skillshare\skills\_team-skills" "C:\work\team-skills"
```

Add the **entry name**, not the external path:

```text title=".skillfollow"
# One direct child of the skills source per line
_team-skills
```

In the skills source's `.gitignore`, add anchored lines **without a trailing slash**:

```text title=".gitignore"
/_team-skills
/.skillfollow.local
```

`/_team-skills/` is not enough: Git stores a symlink as a file, not a directory. Commit `.skillfollow`; keep the link and `.skillfollow.local` untracked. If the link is already indexed, run the following from the skills source after reviewing the path (it removes the index entry, not the working link):

```bash
git rm --cached -- '_team-skills'
```

Then run `skillshare doctor`, `skillshare list --no-tui`, and `skillshare sync --dry-run`. Use `-g` or `-p` to select the intended scope; run `skillshare sync` when the preview is correct. Declaration and ignore files are hand-edited: discovery, status, doctor, and dry runs do not create or repair them. There is no `follow` or `unfollow` command yet.

A `_`-prefixed entry with `.git` is treated as a tracked repository; other followed directories are groups. Skills retain logical paths such as `_team-skills/review` (flat name `_team-skills__review`). Root and repository `.skillignore` rules still apply; following does not bypass filtering. An undeclared first-level link remains invisible to discovery, as before.

## File format

`.skillfollow.local` sits beside `.skillfollow` and adds machine-local names. The two files form a union, base file first; duplicates collapse. Unlike `.skillignore.local`, it has no negation or override rules.

- Trim surrounding whitespace; ignore blank lines and lines starting with `#`. Put comments on their own lines.
- Use a direct child name only. Reject `.`, `..`, absolute paths, drive/volume names such as `C:`, UNC paths, names containing `/` or `\`, or names changed by path cleaning.
- No glob or negation syntax: `*`, `?`, `[`, `]`, `{`, `}`, `!`, and NUL are rejected. Invalid lines produce warnings, not followed entries.
- Only declared first-level links are followed. Nested links inside a followed tree are not traversed.

## States and recovery {#states}

Safety checks use canonical paths. The first applicable state wins; overlap checks reject both entries, not just the later declaration.

| State | Meaning | What to do |
|---|---|---|
| `missing` | Entry absent, dangling, unreadable, or a safety boundary cannot be resolved; a read failure during traversal also makes discovery incomplete | Restore the drive/link/read access or fix the reported boundary; remove an abandoned declaration |
| `not-link` | Real directory, discovered normally | No repair required; declaring it does not change ordinary ownership |
| `invalid-target` | Link points to a non-directory, or entry is neither a link nor a directory | Replace it with a link to a directory, or remove the declaration |
| `cycle` | Resolved directory is the source, inside it, or an ancestor of it | Point to a separate external directory |
| `target-overlap` | Resolved directory equals, contains, or lies inside an enabled skills target | Separate the input directory from the configured output target |
| `inside-git-root` | Resolved directory is inside skillshare's effective Git staging tree | Move the external tree outside that staging tree; ignoring the link cannot hide its physical files |
| `entry-overlap` | Declared targets are equal or one contains the other | Keep only non-overlapping declarations or repoint the links |
| `single-skill` | Resolved root contains `SKILL.md` | Not supported yet; follow a containing group/repository instead, or remove the declaration |
| `followed` | Safe, readable group or tracked repository | Ready for discovery and sync |
| `undeclared-link` | First-level link not named in either declaration file | Leave it invisible, or declare it and add its ignore line |

`not-link` and `followed` pass doctor checks. Other declared states are warnings and pause cleanup. `undeclared-link` is informational and does not pause cleanup. Parser warnings are reported separately.

If `.skillfollow` or `.skillfollow.local` exists but cannot be read, discovery stops rather than continuing with a partial view: `sync` refuses and leaves existing targets in place, `check` and `status` report the read error instead of empty counts, and source Git staging is refused. Restore read access to the file or remove it.

## What commands show {#visibility}

- **`status`** adds `.skillfollow: N entries, M skipped`, with `(.local active)` when applicable, then each prune-pause recovery message. `status --json` adds `source.skillfollow` with `active`, `local_active`, `entry_count`, `followed_count`, `skipped_count`, declared `entries` (`name`, `state`, optional `resolved_target`, `reason`), optional `warnings`, and optional `prune_paused` recovery messages. With no declarations or declaration warnings, the field is omitted.
- **`doctor`** reports each declared state under `skillfollow`, and cleanup blockers under `skillfollow_prune`. Undeclared links remain `undeclared_source_links` info checks. In a Git repository it also reports indexed links, `not-ignored` links, and an unsafe `.skillfollow.local` without changing files.
- **Plain `list --no-tui`** shows followed tracked repositories with `→ <resolved>` (home paths may be shortened to `~`). Skill paths stay logical; the list JSON shape is unchanged.
- **`diff`** previews sync with the same rules. While an entry is unavailable it reports no removals, prints `<target>: prune paused; unavailable .skillfollow entry: <name> (<state>)`, and lists standard-naming managed copies that sync would keep as **Kept**. `diff --json` adds per-target `prune_paused` and `keep` items. Dashboard diff adds `prune_paused` and shows kept copies as `skip`.
- **Dashboard** skills, overview, check, update, audit, and hub discovery include followed skills under logical paths; audit scans followed skills through the resolved root. Content edits, uninstall, enable/disable, target overrides, and source URL changes through a followed tree are refused. Edit the external tree directly; to hide it, edit the **source-root `.skillignore`**. There is no dedicated `.skillfollow` editor tab yet. Dashboard sync shares CLI prune/copy safety and reports per-target `prune_paused`/`kept` plus warnings; Targets counts managed followed links as linked rather than local.

For `_team-skills`, doctor prints the actionable ignore diagnostics:

```text
_team-skills: not-ignored; add "/_team-skills" to <source>/.gitignore
_team-skills: indexed; run git rm --cached -- '_team-skills' and add "/_team-skills" to <source>/.gitignore
.skillfollow.local: tracked; run git rm --cached -- .skillfollow.local
.skillfollow.local: not-ignored; add "/.skillfollow.local" to <source>/.gitignore
```

## Cleanup safety {#cleanup}

While **any declared entry** is unavailable (anything except `followed` or `not-link`), skills-target prune pauses in merge and copy modes, even with `sync --force`. New links and copies can still be created. With standard target naming, an existing managed copy is kept if replacement cannot prove its origin; flat naming can proceed. Merge links may be replaced with a warning that the name could collide when the unavailable entry returns.

Status and doctor print this recovery sentence for each blocker:

```text
prune paused: <name> is <state>; restore or fix <path>, or remove <name> from .skillfollow[.local], to resume cleanup
```

Restore/fix the entry, or remove its name from **every** declaration file containing it, then sync again. An abandoned declaration keeps cleanup paused indefinitely. Removing a declaration does not delete the external tree. Managed orphan links through the logical source can then be pruned; a managed link pointing directly to a now-unfollowed external path is kept with `managed link resolves outside the source after unfollow; remove it or re-run with --force`.

## Update safety {#updates}

Followed tracked repositories are user-owned working copies. CLI, dashboard (including update-all/streaming), and `install --update` require a clean tree and use **fast-forward-only** pull (`--ff-only --no-rebase`). Explicit `--force` is refused, including dry runs. Dirty trees, status-check errors, or failed fast-forwards (including divergence) report an item failure with ``resolve in `<resolved path>` ``; independent batch items continue. Resolve changes/history in that external repository, not by retrying force. Ordinary installed repositories retain their existing update policy.

Regular skills below a followed entry are never reinstalled. `update` refuses each one as an item failure, `followed repository update refused: skill <path> is inside followed entry <name>`, in every selection mode (`--all`, names, globs, groups, project mode, dry runs) and in the dashboard's single update; other items continue. Only the followed repository itself is updated, through the policy above.

**Audit failures still hard-reset a followed repository to the pre-pull commit.** The audit scans the resolved root and reports logical paths; scan errors also block updates. Refusing force does not remove this rollback. Do not edit, repoint the link, or run another Git process in the repository during update: checks are snapshots, not locks, and rollback could discard concurrent changes. Changes pulled or edited outside skillshare are not automatically audited; run `skillshare audit` yourself.

## Source Git safety {#git-safety}

`commit`, `push`, their dry runs, dashboard staging, and init's source commit refuse a declared link that Git can reach if it is indexed or not ignored. Follow doctor's exact ignore line and `git rm --cached` instruction; skillshare never automatically untracks it. The guard follows physical Git reachability, so an unrelated agents/extras repository is not blocked merely because skills have declarations.

Source-repository **pull/reset/checkout** also refuse indexed declarations (even a missing indexed link), or an incoming revision that touches any working-tree path with a link component, **declared or undeclared**. Ignoring a link alone cannot prevent Git replacing it. The error names the incoming commit and path; untrack it if indexed and add its ignore line, or fix the remote revision before retrying. Pull fetches and checks a pinned revision; dashboard checkout checks the selected local or remote-tracking revision without adding an implicit fetch. An ignored followed link survives dashboard discard.

These guards protect skillshare operations, not Git commands you run yourself.

## Limits

Single-skill entries, `follow`/`unfollow`, and a dashboard declaration editor remain future work. Nested links are not followed. Windows junction classification has simulated coverage and Windows cross-compilation, but the feature's real Windows junction/Developer Mode runtime matrix has **not** been verified; neither a successful build nor the earlier standalone probes establish that runtime behavior.

## See also

- [Filtering](./filtering.md#skillignore) — hiding discovered skills
- [Source and targets](../understand/source-and-targets.md) — logical source layout
- [Update](./commands/update.md) — audit policy and rollback
- [Sync](./commands/sync.md) — target modes and naming
