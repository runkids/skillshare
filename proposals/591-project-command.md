# Feature Proposal: Manage global project registrations from the CLI

Issue: [#591](https://github.com/runkids/skillshare/issues/591).

## Problem

The global config's `projects:` mapping gives repositories and linked worktrees
their own targets and skill filters. Registration currently needs a YAML edit or
the dashboard's Projects page. Repository setup scripts and coding agents cannot
register, inspect, change, sync and remove these entries through CLI calls.

## Proposed Solution

Add `skillshare project` for the global `projects:` mapping, reusing lookup, validation and detach helpers with `GET/PUT/DELETE /api/projects`
and a bounded skills/agents sync runner. Shared project operations are extracted
only where both interfaces use them. Keep HTTP envelopes, status codes and dashboard composition
stable. Deliberate safety corrections validate the effective YAML candidate
before writing, use native Windows root comparison, serialize project writers,
detach departing edit paths, preserve surviving ownership and expose completed
work on partial failures.
No new runtime dependencies or config schema changes.

### Commands and settings

```text
skillshare project                       # same as list
skillshare project list [--json]
skillshare project show <project> [--json]
skillshare project add <root> --target T [--target T]... [settings] [--sync] [-n|--dry-run] [--json]
skillshare project edit <project> [settings] [--sync] [-n|--dry-run] [--json]
skillshare project sync <project> [-n|--dry-run] [--json]
skillshare project remove <project> [--yes] [-n|--dry-run] [--json]
```

This command always uses global mode, including from either project-config
directory style. `-g/--global` is accepted; explicit `-p/--project` fails with
`projects belong in the global config`. Options work before or after operands,
and `--` ends option parsing.

| Setting | Add | Edit |
|---|---|---|
| `--target T` (repeatable or comma list) | Required, at least one | Replaces targets |
| `--name N` | Optional custom name | Sets name; empty resets to root basename |
| `--skills` / `--no-skills` | Skills on by default | Enables, preserving settings, or disables |
| `--agents` / `--no-agents` | Agents off by default | Enables, preserving settings, or disables |
| `--skills-mode`, `--agents-mode` | `merge`, `symlink`, or `copy` | Sets mode; empty restores inheritance |
| `--skills-naming` | Sets naming | Sets naming; empty restores inheritance |
| `--skills-include`, `--skills-exclude`, `--agents-include`, `--agents-exclude` | Repeatable patterns | Replaces that list; empty clears it |

Only supplied edit settings change. Any resource setting enables that resource
if it was off, including clearing an existing `enabled:false`. Omitted settings
preserve disabled state. A disable flag combined with any setting for the same
resource fails before side effects. There is no `--agents-naming`.
The CLI rejects agent filters with agents mode `symlink`; HTTP retains its
current validation. Help explicitly distinguishes these replace-list flags from
`target`'s incremental flags. Add requires an existing directory; editing a
registration whose directory is missing is allowed, with sync reporting it.

### Lookup, identity and stored spelling

One resolver serves show, edit, sync and remove. Effective names are the custom
name or root basename. Tokens beginning with `./`, `../`, `/`, `~`, a Windows
drive or UNC prefix are roots. Other tokens are tried as both name and relative
root; different matches fail with both candidates. Add accepts only a root.

Identity expands `~`/`~/`, resolves relative paths from the working directory,
cleans paths lexically and trims trailing separators while retaining volume
roots. It uses `utils.PathsEqual`: native separators and case-insensitive Windows
comparison. Symlinks and junctions are not resolved, so aliases remain distinct
registrations. Reject `~user` and Windows drive-relative paths. For legacy
case-different Windows keys, root lookup reports both; a unique name selects one.

Add and edit reject custom names that would classify as roots (leading `./`,
`../`, `/`, `~`, a drive or UNC prefix). A derived basename can still look
like a root token, such as `~scratch`; select that registration by its root
spelling or pass `--name` when adding it.

Edit and remove preserve the selected declared key. Add retains tilde and
absolute arguments as typed except trailing separators; a relative root is
stored as absolute, folded to `~/...` when under the home directory. This rule is
independent of `preserve_tilde_on_save`, which does not rewrite project keys.
Duplicate roots or names fail rather than selecting an arbitrary entry.

### Repeated runs and concurrent writers

Setup scripts can run `add` on every invocation. If the root is already
registered and the requested settings produce the same entry, `add` succeeds
with `applied:false, unchanged:true` and still runs `--sync`. If the settings
differ, it fails with `project is already declared` and points at `project
edit`, so a rerun never silently rewrites an entry. A new root whose effective
name collides with another registration fails before writing; worktree scripts
pass `--name` (for example `<repo>-<branch>`). `edit` changes only supplied
settings, so a rerun with the same flags is a no-op.

Separate worktree setup scripts can register at the same time. Every project
mutation holds an exclusive lock file next to the global config from load
through write, so two writers serialize instead of the second overwriting the
first. The lock uses the existing `gofrs/flock` dependency and the resolved config
destination, so config symlinks share one lock. Acquisition waits up to 10
seconds; timeout fails before changing the registration. CLI and dashboard
project mutations reload the latest config after acquisition, before lookup,
patching or validation, and hold the lock through detach and persistence.
Dashboard project conversion takes the same lock. Read-only commands and dry
runs never create or acquire a lock. Other config writers, including `target
add`, MCP, hooks and ordinary `Config.Save()` callers, remain outside this
protection; concurrent edits through those paths can still overwrite changes.

### Shared helpers, project persistence and previews

HTTP-independent `internal/projects` helpers provide lookup, validated project
candidates and the project operations actually reused by both adapters. HTTP passes replacement values unchanged, retaining
root-only lookup and null-means-disabled resource semantics. The CLI converts
patch flags and defaults into a full value. Prompts, rendering, HTTP decoding and
status mapping, and operation logging stay in the adapters.

`internal/config.PrepareSave` prepares exact YAML bytes and a fresh expanded
config without mutating its caller. It uses the existing save pipeline: omit
synthetic project targets, apply tilde policy, write the schema header, and
preserve comments, anchors and aliases. Decode and validate the effective project
set and filter patterns before any write. Anchor or merge effects on other
entries appear in the preview; an invalid effective candidate changes nothing.
`SaveCandidate.Write` atomically writes those same bytes without serializing
again. This persistence contract is scoped to project operations; ordinary
`Config.Save()` callers keep their existing serializer and write behavior.
The server installs the expanded candidate
under its existing lock and refreshes dependent state after success.

Dashboard project routes bypass the persisting pre-handler auto-reload. Reads
reload without locking; mutations reload without migration after acquiring the
project lock.

List, show and previews use non-persisting loads: legacy migration is in memory,
with no config/registry relocation, target writes or operation log entries.
Before startup maintenance, classify list, show, every dry run and every JSON
project invocation so they skip Windows config migration, XDG backup/trash/log
migration, old-binary cleanup and version checks/cache writes. Legacy Windows
config is read in place. JSON has no migration notices on stdout; requested
mutations still apply when not a dry run.

### Sync and removal

Extract per-target execution into `internal/sync`, accepting expanded targets
and options for resource kinds, force, dry run, project mode and discovery.
Return existing server details, including per-target status, unmatched patterns,
folder conflicts, path overlap, discovery and ignore statistics. The server
keeps preparation and response building. No server construction or subprocesses.

The project CLI selects resource kinds from the declared project, not the
expanded targets: a project with skills off never runs the skills pass, even
when the global mode is `symlink` and the synthetic skills target would link
the whole source.

An empty intersection of requested and enabled resources succeeds without
discovery, backups or target writes. This does not change the shared runner's
empty-kind default.

The project CLI selects skills and agents with force off. After saving, sync
uses targets expanded from the saved candidate so renames and tool groups are
current. A dry run uses the unsaved candidate. Any target failure exits 1 while
reporting saved state and completed work, without rollback. `project sync` is
the standalone retry path; a missing folder cannot silently succeed.

An edit that drops targets, or disables skills, detaches each departing target
path with the removal primitive below before the new targets sync, and previews
it in a dry run. Paths that the candidate still owns are left in place.

Removal first prepares and validates the candidate, then detaches targets,
persists, refreshes and reports. Reuse one extracted primitive with dashboard
target removal, including its copy helper and failure-injection seam:

| Existing target | Detach result |
|---|---|
| Whole-folder link or junction | Remove the link |
| Merge | Replace links into the source with local copies where source content exists |
| Copy | Remove the manifest and keep existing copies |

Before detaching, check each departing path against the surviving targets, as
`target remove` does with `config.SkillsPathKeptBy`; a path another target
still writes is retained and reported, including in previews. Agent files stay
in place. Preview lists removed, copied-back and retained files.
Do not substitute `internal/sync.DetachSkills`, whose semantics differ.
There is no transaction: later-target or persistence failures report completed
detaches and whether registration was kept or removed. The same applies to
edits: failed saves return `applied:false`, `registration:"kept"`, the persisted
project and completed detach work; successful saves followed by failed sync
retain `applied:true`. No-op adds and edits skip project writes and logs while
still honoring `--sync`. Identical-add success is CLI-only; HTTP create keeps
its existing conflict behavior. Warn with the normalized
root's remaining `mcp.projects` and `hooks.projects` entries.

### Output and automation

Reuse the dashboard project fields: `root`, `path`, `name`, optional `customName`,
`targets`, `skills`, `agents`, `groups`, `missing`, `hasOwnConfig`; disabled
resources are null. CLI JSON envelopes are:

| Result | JSON contract |
|---|---|
| List / show | `{"projects":[item]}` / `{"project":item}`; empty list is `[]` |
| Mutation | `action`, `project`, `applied`, `affected` roots, `warnings`, optional `sync` |
| Dry run | Same result with `applied:false` |
| No-op edit | `applied:false`, `unchanged:true` |
| Standalone sync | `{"action":"sync","project":item,"sync":result}` |
| Partial removal | `action:"remove"`, `detach`, `registration:"kept"` or `"removed"`, `error` |
| Cancellation | `{"action":"remove","cancelled":true}` |

Failures before any change use the existing JSON error envelope and exit 1.
Results containing completed work retain that work and exit 1 on failure,
including a saved registration followed by failed sync. Sync previews exit 0.
Text output follows existing rows, next-step, dry-run and cancellation helpers.

Remove confirms with default No; `--yes` skips it. JSON removal requires
`--yes`, except dry runs, and never prompts. Cancellation follows `target remove`.
Each applied mutation logs one `project` operation (`add`, `update` or `remove`,
scope `cli`), with a separate `sync` operation when run. Partial removal logs
status `partial` and its error. Previews log nothing; shared services never log.

## Alternatives Considered

- Keep editing YAML or using the dashboard. Both work, but repository setup
  still needs a separate configuration-editing or browser step.
- Use `target add` for each tool path. This loses the project registration's
  tool grouping, effective name and shared resource settings.
- Call the dashboard over HTTP or create a server from the CLI. That adds a
  server dependency to repository setup and leaves domain logic in HTTP code.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Proposed implementation series, after agreement on scope and ownership:

1. **[Project command 1/2]** Extract shared lookup, project-scoped config candidates,
   read-only loading, removal and a bounded sync runner; move dashboard handlers
   onto them. Include candidate-before-write and Windows path comparison as
   explicit corrections. Keep the extraction and deliberate safety corrections distinct for review.
2. **[Project command 2/2]** Add all CLI commands, five-shell completions
   (subcommands, flags and enumerated values), documentation and existing
   translations, integration and E2E coverage. Stack on the first branch.
   Keep CLI-specific policy and output in the CLI adapters.

Coverage includes HTTP parity, anchor/alias candidates, byte-identical read-only
legacy layouts with a release-version binary, lookup ambiguity, confirmations,
partial sync/detach failures and bounded root-identity fuzzing. Add the projects
package to the Windows CI allow-list in the first PR; the second wires a native
Windows add → sync → remove smoke run into CI for merge, symlink and copy
modes, recording token and architecture. Native runtime evidence remains
pending until an actual Windows run; cross-compilation is not a substitute. Update source-write/walk inventories when
moving helpers. Existing command guidance and the built-in skill are updated
with the CLI. No dynamic project-list completion.

Out of scope: `mcp.projects` or `hooks.projects` mutation/sync, `project convert`,
a project TUI, per-repo project-config changes, schema changes, and a generic
configuration framework.

## Scope agreement

The maintainer approved implementation ownership and both PRs in
[the scope clarification](https://github.com/runkids/skillshare/issues/591#issuecomment-6319115409).
All six commands remain in scope. Persistence changes stay project-specific,
shared extraction follows actual reuse, synchronization uses the bounded runner,
and project writers share `gofrs/flock` plus post-lock reload. Repeated add with
different settings fails and points at `project edit`.
