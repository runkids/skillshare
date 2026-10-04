# Linked skills sources (experimental)

`.skillfollow` opts named first-level source symlinks/junctions into discovery.
Undeclared child links stay invisible. A linked source root is a separate,
pre-existing dotfiles use case. Agents and extras are outside this feature.

## Setup

Use the configured skills source root (default `~/.config/skillshare/skills/`,
Windows `%AppData%\skillshare\skills\`, project `.skillshare/skills/`, or
`sources.skills`). Create a first-level link such as `_team-skills` to an external
multi-skill repo/group. It must not contain a root `SKILL.md`, overlap the source
or an enabled skills target, or reside inside the effective Git staging tree.

Write `_team-skills` on its own line in `<source>/.skillfollow`. Add
`/_team-skills` and `/.skillfollow.local` to `<source>/.gitignore`, with **no
trailing slash**. If indexed, review and run `git rm --cached -- '_team-skills'`
from that source; this leaves the working link. Commit the declaration, not the
link or the local file. Check `doctor`, then preview `sync --dry-run` in the
correct `-g`/`-p` scope before syncing. Do not create links or edit declarations
unless the requested work authorizes that change.

`.skillfollow.local` adds names beside the base file; the files are a union and
duplicates collapse. Trim whitespace, ignore blank lines and full-line `#`
comments. Names must be direct children: no `.`, `..`, separators, absolute,
volume/UNC forms, path-cleaning changes, glob/negation (`*?[]{}!`), or NUL.
Invalid lines warn. There are no `follow`/`unfollow` commands yet.

A `_`-prefixed directory with `.git` is a tracked repo; other directories are
groups. Paths stay logical (`_team-skills/review`, flat `_team-skills__review`).
Root/repo `.skillignore` still applies. Nested links are not traversed.

## States and recovery

| State | Action |
|---|---|
| `missing` | Restore entry/link/read access or fix unresolved safety boundaries |
| `not-link` | Real directory; normal discovery and ownership, no repair |
| `invalid-target` | Replace non-directory entry/target with a directory link |
| `cycle` | Move target outside the source and its ancestors/descendants |
| `target-overlap` | Separate external input from enabled skills outputs |
| `inside-git-root` | Move external target outside effective staging tree |
| `entry-overlap` | Remove/repoint overlapping declarations; both are rejected |
| `single-skill` | Unsupported until the next rollout; use a containing group |
| `followed` | Ready |
| `undeclared-link` | Leave invisible or declare and ignore it; info only |

Declared states other than `followed`/`not-link` pause all merge/copy skills
prune, even with force. New links/copies may still be created. Standard-name
managed copies are kept when replacement cannot prove their origin; flat naming
can proceed. Replaced merge links warn of possible collisions when entries return.
Status/doctor say:

```text
prune paused: <name> is <state>; restore or fix <path>, or remove <name> from .skillfollow[.local], to resume cleanup
```

Remove an abandoned name from every declaration file containing it, then sync.
Removing a declaration does not delete its external tree. Managed orphan links
through the logical source can be pruned; fully external managed links after
unfollow are kept with `managed link resolves outside the source after unfollow;
remove it or re-run with --force`.

## Inspection and mutation

- Status adds entry/skipped counts and pause messages; JSON optionally adds
  `source.skillfollow` (`active`, `local_active`, counts, declared `entries`,
  optional `warnings`/`prune_paused`). Entries have `name`, `state`, optional
  `resolved_target`, and `reason`. Without declarations/warnings the field is omitted.
- Doctor uses `skillfollow` and `skillfollow_prune`; undeclared links remain
  `undeclared_source_links` info checks. `not-ignored` tells you the exact ignore
  line/file; indexed links require `git rm --cached`. A tracked/unignored local
  file is also warned about. These checks never fix files automatically.
- Plain list adds `→ <resolved>` for followed tracked repos. Dashboard reads
  show logical paths; content writes, uninstall, toggles, target overrides, and
  source URL edits through a followed tree are refused. The generic boundary
  says `<path> is a link; edit its target directly`. Hide using source-root
  `.skillignore`; do not promise that dashboard toggles work for followed skills.
  Dashboard sync shares prune/copy safety and reports `prune_paused`/`kept` with
  warnings; Targets counts managed followed links as linked, not local.

## Git and update safety

Followed tracked repos update with a clean-tree check and `--ff-only --no-rebase`.
Explicit `--force` is refused, even for dry runs. Dirty/status-error/diverged
items fail with `resolve in <resolved path>`; independent batch items continue.
CLI, dashboard including streaming/all, and `install --update` share the policy.
**Audit failure still hard-resets to the pre-pull commit.** Do not edit the repo,
repoint links, or run Git concurrently: checks are snapshots, not locks. Edits or
pulls outside skillshare are not automatically audited; run `audit` explicitly.

Commit/push, dashboard staging, and init source commits require reachable declared
links to be untracked and ignored. Source pull/reset/checkout rejects indexed
declarations, including missing indexed links, and incoming paths through any
working-tree link (declared or not). Ignoring alone cannot stop replacement: use
the reported untrack/ignore instruction or fix the incoming remote revision.
Checkout checks the selected existing revision without adding a fetch. An ignored
followed link survives dashboard discard. These guards do not cover external Git
commands; unrelated agents/extras Git scopes are not automatically blocked.

Single-skill entries and a declaration editor are future work. Real Windows
junction/Developer Mode feature-runtime verification is pending; simulated tests
and cross-compilation do not establish it.

Full setup/state reference: https://skillshare.runkids.cc/docs/reference/skillfollow
