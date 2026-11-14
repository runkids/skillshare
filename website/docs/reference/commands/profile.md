---
sidebar_position: 22
---

# profile

Save named skill selections and switch the skills deployed to your tools. Installed source skills stay available for editing, updating and auditing. This command is separate from the security policy selected by `audit --profile`.

## Usage {#usage}

```bash
skillshare profile                          # Same as list
skillshare profile list --json
skillshare profile show frontend --json
skillshare profile create frontend --description 'UI work' --add-include 'frontend__*'
skillshare profile edit frontend --add-include design-review --add-exclude 'frontend__legacy-*'
skillshare profile add frontend frontend/button design-review
skillshare profile remove frontend design-review
skillshare profile rename frontend web
skillshare profile delete web                # Only while inactive here
skillshare profile use frontend --dry-run
skillshare profile use frontend
skillshare profile use --none
```

All commands accept `-g` / `--global` and `-p` / `--project`, with normal project auto-detection. `use` without a name opens a terminal picker; scripts must supply a name or `--none`.

## Definition options {#definition-options}

| Flag | Meaning |
|------|---------|
| `--description T` | One-line description for create/edit |
| `--add-include P`, `--remove-include P` | Add or remove an exact include pattern; repeatable |
| `--add-exclude P`, `--remove-exclude P` | Add or remove an exact exclude pattern; repeatable |
| `--json` | Structured output for reads, definition changes and switches |

Names use 1–64 lowercase letters, digits, underscores or hyphens, starting with a letter or digit. `none` is reserved. Patterns use Go `filepath.Match` on the **flat source name**, such as `frontend__button`, regardless of the target's naming mode. Quote globs so your shell does not expand them.

Create without patterns makes an empty selection. A definition with no `skills` key does not limit skills; `skills: {}`, empty includes and blank-only includes select nothing. Excludes require an include key: use `include: ['*']` for everything except the exclusions. Explicit `include: []` with excludes is valid; show marks those excludes as inert. Unknown keys are preserved on save, but make that definition unusable on this version.

Definition changes save without syncing. Changes affecting the active selection print `Next: skillshare sync`. Renaming the locally active definition follows its name in local state. Deleting it is refused until you switch away.

An unusable definition, including one with keys from a newer version, can still be renamed or deleted while inactive. Rename preserves its raw contents and comments, even when active; it does not make the definition usable. Active deletion, semantic edits and activation retain their validation.

## Assign skills {#assign-skills}

`profile add <name> <skill>...` and `profile remove <name> <skill>...` accept flat source names, source-relative paths, or unique base names. An ambiguous base name fails with its candidates. Assignment does not activate a profile, sync a target, or override `.skillignore`, target filters or skill routing.

| Operation | Rule |
|-----------|------|
| Add a selected skill | Report `already in`; no change |
| Add an unselected skill | Remove its exact exclude; add an exact include if still needed |
| Add through a matching glob exclude | Refuse the entire batch, naming the skill and pattern; nothing is saved |
| Remove an unselected skill | Report `not in`; no change |
| Remove a selected skill | Drop its exact include; add an exact exclude if a broader include still selects it |
| Remove from a definition with no `skills` key | Write `include: ['*']` and exclude that exact name |
| Remove the last exact include | Keep explicit `include: []` and all existing excludes; the selection is empty |

```bash
skillshare profile add frontend frontend/button frontend/dialog -p
skillshare profile remove frontend frontend__button --json -p
skillshare sync -p
```

JSON assignment results contain `profile`, `changed` and per-skill `skills` rows with `name` and `result`. Results are `added`, `removed`, `already-in` or `not-in`. Active membership changes print the next sync command in text mode.

## Membership hints {#membership-hints}

After successful `install` or `new`, skills outside the usable active profile get one hint: `Not in active profile "frontend": skillshare profile add frontend <flat-name> -g` (project: `-p`). Install uses final names including groups and nested tracked/local skills. A partial install hints only for successful publications; dry runs, skips, failures and agent installs produce none. Install JSON stays unchanged, with the hint on stderr.

`move` warns when a skill's old and new flat names change membership in any usable profile, including inactive definitions. Folder moves check every skill, even with no targets configured. Warnings do not rewrite profile patterns; inspect them before syncing.

## Switch options {#switch-options}

| Flag | Meaning |
|------|---------|
| `--dry-run`, `-n` | Plan without changing retained files or creating a lock |
| `--no-sync` | Record the selection as pending; run `skillshare sync` later |
| `--none` | Clear the active profile and restore the normal target selection |
| `--allow-empty` | Confirm zero selected skills when enabled source skills exist |
| `--discard-edits` | Snapshot and verify the listed modified managed copies before discarding them |

A switch visits this config's valid, enabled merge/copy skill targets only. Target filters and skill routing still apply. It does not sync agents, extras, plugins, MCP settings, hooks, registered `projects:` targets or symlink-mode targets. A missing symlink target root is not created. Symlink targets need merge/copy mode for profiles to affect their contents.

Edited managed copies and unmanaged entries are kept across copy/merge mode changes. A switch that would replace or prune one refuses and lists its reason. Use `skillshare collect` to retain authored changes in the source. `--discard-edits` only authorizes the listed modified managed copies; it never authorizes unmanaged content. The snapshot must contain the planned bytes, and the writer checks again before deletion. Edits made after planning remain in place.

While a profile or pending switch exists, `diff` shows preserved copies with a kept reason, including copies left in former project folders. These rows are visible in text, JSON and the TUI but count as no executable changes. The API retains force-sensitive `skip` rows for selected entries; departing preservation is `kept` even with Force enabled. Collect or manually remove unmanaged content; verified discard is available only for modified managed copies.

Snapshots use the existing global backup directory, or `.skillshare/backups` in project mode. Text and JSON output include their paths and restore commands. Existing backup retention applies: restore before cleanup removes the snapshot. The same snapshot rule applies to ordinary `sync --discard-edits`, independently of JSON output. In project mode this includes declared former skills folders of selected eligible targets; each reported moved restore command recovers its original folder without replacing the current one. `--no-sync` cannot be combined with discard.

## State and recovery {#state-and-recovery}

Definitions live under `profiles:` in the global or project config. Active selection is machine-local, keyed by the canonical config path under `ProjectStateDir()/profiles`. `SKILLSHARE_CONFIG` selects a separate identity; symlink aliases of the same config share it. Moving a config or using another checkout gives it a new local selection. On Windows the state directory stays outside a shared config git root.

A completed switch exits zero and clears pending. A refused switch exits nonzero without changing state or targets. A partially applied switch exits nonzero, leaves the new active name with `pending: true`, and reports each target. Retry `skillshare sync` or the same `profile use`. A pending clear-to-none retry keeps edit protection until every eligible target completes. An ordinary sync warns about kept entries while continuing its other work.

Mutations and skill sync take a bounded file lock before reloading config/state. The empty lock file is retained after release, including after non-dry refusals, so waiters always lock the same file. Dry runs and reads create no lock or directories. Legacy copy verification may use private temporary replay scratch, removed before returning. A sync that began before the config had a `profiles` key or a state file holds no lock and can finish after the first switch. It runs as a sync does with no profile, so it can still replace or remove edited copies and add skills the profile leaves out; the next sync applies the profile.

Missing, unusable or unreadable active state blocks skill sync, diff, analyze, the sync matrix and target-bound skill serving. Status and profile list show the cause. Repair with `skillshare profile use <name>` or `skillshare profile use --none`; neither silently falls back to all skills. Filesystem permission problems may need repair first.

With no profile key or state, existing commands retain their behavior. Inactive definitions are inert, apart from locking ordinary skill sync. Older binaries ignore local selection and can drop definitions when saving config.

Copy manifests record versioned output fingerprints (`sha256-v2:`), framing each relative path and file's bytes separately. Standard ignored files, modes, ownership and timestamps do not count as preserved edits. Source checksums keep their existing meaning.

During profile operations, an older copy is clean only when available source matches its recorded source checksum and reproduces the deployed bytes, including any target-name rewrite. This establishes present reproducibility, not historical provenance. Missing source or a failed replay keeps the copy protected. Unknown, malformed or explicitly empty output fingerprints are also protected; they never fall back to an older encoding. Older source-free verification remains available outside profile operations for compatibility. Binaries predating output fingerprints cannot provide this protection after a downgrade.

## JSON {#json}

Switch output contains `profile: {previous, active, pending}`, `complete`, per-target `targets` with `added`, `refreshed`, `removed`, `converted`, `kept_modified`, plus `blocked`, `symlink_targets` and `snapshot`. Target state is `applied`, `incomplete` or `failed`. Check the exit code as well as the result. List/show include definition usability, selected names, unmatched patterns and, for show, per-target selected counts.

See [configuration](/docs/reference/targets/configuration#profiles), [filtering](/docs/reference/filtering#profiles), [sync](/docs/reference/commands/sync) and [status](/docs/reference/commands/status).
