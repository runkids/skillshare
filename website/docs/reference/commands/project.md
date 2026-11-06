---
sidebar_position: 21
---

# project

Manage the project folders the global config syncs skills and agents into, without creating a `.skillshare/` configuration in each folder. A project is saved in `config.yaml` unless you keep it on this machine only.

`project` always uses the global config, even when run inside a project. This differs from `-p` / `--project`, which selects a repo's own configuration and is rejected by this command. Use [project mode](/docs/understand/project-skills) when teammates should receive the setup with the repo.

## Usage {#usage}

```bash
skillshare project                          # Same as project list
skillshare project list --json
skillshare project show <project> --json
skillshare project add <root> --target T [options]
skillshare project edit <project> [options]
skillshare project sync <project> [--dry-run] [--json]
skillshare project localize <project> [--dry-run] [--json]
skillshare project share <project> [--yes] [--dry-run] [--json]
skillshare project remove <project> [--shared] [--yes] [--dry-run] [--json]
```

`add` requires an existing folder and at least one target. Other commands accept the effective project name (custom name or folder basename) or its root path. Prefix relative paths with `./` or `../` to select a root explicitly. If a bare token matches one project's name and another project's root, the command reports both candidates instead of choosing one. Symlink aliases stay separate registrations.

## Flags {#flags}

| Flag | Applies to | Description |
|------|------------|-------------|
| `--target T` | add, edit | Target tools; repeat or use a comma list. Required for add; replaces the entire list on edit |
| `--name N` | add, edit | Custom name; `--name=` restores the folder basename |
| `--skills`, `--no-skills` | add, edit | Enable or disable skills; enabled by default on add |
| `--agents`, `--no-agents` | add, edit | Enable or disable agents; disabled by default on add |
| `--skills-mode MODE`, `--agents-mode MODE` | add, edit | `merge`, `symlink`, or `copy`; empty value clears the override |
| `--skills-naming NAMING` | add, edit | `flat`, `standard`, or `prefixed`; empty value inherits |
| `--skills-include P`, `--skills-exclude P` | add, edit | Repeat for multiple glob patterns; replaces the corresponding list; empty value clears it |
| `--agents-include P`, `--agents-exclude P` | add, edit | Same replacement semantics for agents; filters require merge or copy mode |
| `--machine-local` | add | Save on this machine only, not in `config.yaml` |
| `--shared` | add, remove | add: save in `config.yaml` even when [`projects_storage`](#local-registrations) is `machine-local`. remove: also remove `config.yaml`'s copy of a project this machine overrides |
| `--sync` | add, edit | Save, then sync this project's skills and agents; force is off |
| `--dry-run`, `-n` | add, edit, sync, remove, localize, share | Preview without writing or prompting |
| `--yes` | remove, share | Skip confirmation; required for removal with JSON unless previewing. share asks only when `config.yaml` has different settings for the project |
| `--json` | all | Structured output; check the process exit code as well |
| `--global`, `-g` | all | Explicit global scope (also the default for this command) |
| `--help`, `-h` | all | Show help |

Resource settings turn on that resource, including clearing an existing `enabled: false`. Omitted settings preserve its disabled state. Editing changes only supplied settings; enabling an existing resource keeps its settings. Opposing toggles and disable-plus-setting combinations such as `--no-skills --skills-mode copy` are rejected. There is no agents naming flag. Unlike [`target`](/docs/reference/commands/target), these include/exclude flags replace lists rather than add or remove individual patterns.

## Examples {#examples}

### Register, preview, and sync {#register-and-sync}

```bash
skillshare project add ~/work/shop-web --target claude,codex \
  --skills-mode copy --skills-include 'frontend-*' --agents --dry-run
skillshare project add ~/work/shop-web --target claude,codex \
  --skills-mode copy --skills-include 'frontend-*' --agents --sync
skillshare project show shop-web --json
```

The project is saved under [`projects`](/docs/reference/targets/configuration#projects) in `config.yaml`, so every machine that uses that file gets it. Add `--machine-local` for a folder that exists on this machine only; see [where projects are saved](#local-registrations). Shared tool folders are grouped into one sync target. A normal `skillshare sync` also syncs every project.

Projects with skills disabled never run the skills pass, including in global symlink mode. An empty requested/enabled resource selection succeeds without discovery, backups or target writes.

### Replace filters or reset overrides {#edit-settings}

```bash
skillshare project edit shop-web --skills-include 'frontend-*' --skills-include 'core-*'
skillshare project edit shop-web --skills-exclude= --skills-mode= --name=
skillshare project sync shop-web --dry-run --json
skillshare project sync shop-web --json
```

Add and edit validate the full effective configuration before writing. An edit is saved where the project is: in `config.yaml`, or on this machine for a machine-local project. In `config.yaml`, editing a project declared through a YAML anchor also changes the aliases of that anchor. Mutation results list every affected root. A no-op add or edit returns `applied: false` and `unchanged: true`, and still honors `--sync`. Repeating add with different settings fails and points at `project edit`. Custom names must not look like roots (for example `~name`, `./name`, or `C:name`).

### Remove a project {#remove}

```bash
skillshare project remove shop-web --dry-run --json
skillshare project remove shop-web --yes --json
```

Removal deletes the project where it is saved. Removing a project from `config.yaml` removes it for every machine that uses that file, and the confirmation says so. When this machine overrides a `config.yaml` project, removal deletes only this machine's copy: the `config.yaml` version applies again and the JSON result reports it under `remaining`. Add `--shared` to remove both.

Removal detaches skills before removing the project. A whole-folder link is removed; merge-mode links become local copies while their source content exists; copy-mode files remain and their manifest is removed. A path still owned by a surviving enabled target is retained.

CLI edits detach dropped skill paths by the same rules. While a project's skills remain off, any CLI edit, including a rename or an agents-only edit, also detaches its remaining skill paths, even if skills were turned off earlier in the dashboard. Copy-back and surviving enabled-target ownership rules still apply. Dashboard saves never detach skills. Agent files remain in place. The preview and result list removed, copied-back, and retained paths.

## JSON and failures {#json-and-failures}

| Command | Main fields |
|---------|-------------|
| list | `projects` array (empty when none are registered) |
| show | `project` object |
| add, edit, remove, localize, share | `action`, `project`, `applied`, `affected`, `warnings`; optional `sync`, `detach`, or `remaining` (remove) |
| sync | `action`, `project`, `sync`; previews also have `applied: false` |

Project objects include `root` (stored spelling), `path` (expanded path), `name`, `targets`, `skills`, `agents`, `groups`, `missing`, and `hasOwnConfig`, `origin` (`shared` or `local`), `originPath` (the owning file), and `sharedCopy` (a local project that `config.yaml` also declares). An omitted or disabled resource is `null`. Enabled resources expose their settings. Text `project list` and `project show` report the on/off state and origin. The sync result includes `results`, `failed`, `warnings`, and discovery/filter diagnostics.

A failed target makes sync exit 1 while reporting completed targets. Sync previews report conflicts in `failed` and exit 0; invalid input still fails. If add/edit saves successfully but its sync fails, the registration stays saved; fix the target and retry with `project sync`. A missing project folder can still be edited, but cannot be synced.

Edits and removals are not transactional: a later detach or save failure can leave earlier filesystem changes in place. The JSON result reports completed `detach`, `registration` (`kept`, `updated`, or `removed`), and `error`. A failed edit save has `applied: false` and describes the persisted project; a successful save followed by failed sync keeps `applied: true`; inspect those fields before retrying. Failures before any changes use an error payload. Every project command skips startup migrations, cleanup, and the version check, including text mutations.

This command manages skills and agents only. MCP project settings and hooks use their own commands; it does not convert targets or edit a project's own config.

## Where projects are saved {#local-registrations}

A project is saved in one of two places:

- **`config.yaml`** (`origin: shared`, the default). Every machine that uses this config file gets the project, for example through dotfiles or `skillshare push`.
- **This machine only** (`origin: local`). A registry outside the config directory, for a folder that exists on one machine.

`project add` uses `config.yaml` unless you pass `--machine-local`. To make machine-local the default, set [`projects_storage: machine-local`](/docs/reference/targets/configuration#projects-storage) in `config.yaml`; `--shared` then saves one project in `config.yaml`. The dashboard's **Add project** dialog offers the same choice and starts from this setting. Edits and removals stay in the place the project is saved.

Move a saved project between the two places:

```bash
skillshare project localize shop-web --dry-run --json   # keep a copy on this machine only
skillshare project share shop-web --dry-run --json      # move it into config.yaml
```

`localize` copies a `config.yaml` project to this machine without changing `config.yaml`, detaching files, or syncing. Other machines keep the `config.yaml` version; this machine uses its copy. `share` writes a machine-local project into `config.yaml` and deletes the local copy. When `config.yaml` already has the project with different settings, `share` asks before replacing them; `--yes` replaces them without asking. Both are no-ops when the project is already in that place, and neither syncs.

A local project replaces the whole `config.yaml` entry at the same normalized root (`~` expanded, path cleaned; project symlink roots stay separate). Removals saved by v0.29.0 hid a `config.yaml` project on that machine; they stay hidden until the project is added again. Duplicate names at different roots and duplicate normalized roots within one layer are rejected. A project's MCP servers and hooks are always under `mcp.projects` and `hooks.projects` in `config.yaml`, wherever the project itself is saved.

The versioned registry is `projects/<config-identity>.yaml` under `$XDG_STATE_HOME/skillshare` when set, otherwise `~/.local/state/skillshare` on macOS/Linux or `%LOCALAPPDATA%\skillshare\state` on Windows. Keep explicit state overrides local. The identity follows the resolved absolute config destination: config symlink aliases share state, different config files do not, and moving the destination starts a new registry. Reads create no state files. Malformed or unsupported registry versions fail closed.

The dashboard's **Convert** action replaces ordinary targets with a `config.yaml` `projects` declaration in one write. Roots already declared, locally overridden, or hidden on this machine cannot be converted. The raw config editor validates submitted YAML together with this machine's projects before writing it.

## Concurrent project writers {#concurrency}

CLI and dashboard project mutations share a `gofrs/flock` lock beside the local registry in its state directory, keyed by the resolved global config destination, wait up to 10 seconds, and reload after acquiring it. Config symlinks share the same lock. Reads and dry runs do not create or acquire it. Candidates reject local or shared snapshots changed since loading before detach or write. A change to both places, such as `share`, writes `config.yaml` first and the local registry second, so an interrupted write never loses the project. Other shared-config writers, including `target`, MCP, hooks, and ordinary `Config.Save()` callers, remain outside this protection; concurrent writes through those paths can still overwrite changes.

## Related {#related}

- [Many Projects, One Config](/docs/how-to/recipes/many-projects-one-config)
- [Project configuration fields](/docs/reference/targets/configuration#projects)
- [MCP project settings](/docs/reference/commands/mcp#manage-several-projects-from-the-global-config)
