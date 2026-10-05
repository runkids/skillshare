---
sidebar_position: 4
---

# hooks

Manage named hooks and synchronize each receiving Agent's or Git's native configuration.
Use the dashboard's **Hooks** page to add, edit, import, enable or disable entries,
then preview before syncing. Hooks also appear in destination **Targets** and
**Projects**, **Sync**, and **Settings → Backups**.

Use a hook row's menu to view the native configuration or code for each target,
including script files and destination paths. This read-only preview shows that
hook's contribution; shared files keep their other settings. Disabled hooks can
be inspected but publish nothing.

## Commands

```bash
skillshare hooks
skillshare hooks list --json
skillshare hooks add check --file ./check.yaml
skillshare hooks edit check --file ./updated-check.yaml --sync
skillshare hooks import --from claude --json
skillshare hooks import imported --from claude --file ./settings.json --dry-run
skillshare hooks disable check --sync
skillshare hooks enable check --sync
skillshare hooks sync --dry-run --json
skillshare hooks sync
skillshare hooks sync check --replace --dry-run
skillshare sync hooks --dry-run --json
skillshare sync hooks
skillshare hooks remove check --sync
skillshare hooks remove check --keep-files
skillshare hooks restore BACKUP_ID --dry-run
skillshare hooks restore BACKUP_ID
```

Without a subcommand, `hooks` lists entries. `add` and `edit` read an Entry JSON
or YAML document from `--file`. Import without a name lists candidates, one per
Agent event or file; choose one name to save. Saving an import takes over the
registrations it read in place: the next sync adopts them without `--replace`,
and events you did not import stay unmanaged. Import only reads configuration or
code and never runs it.
Add, edit, import, enable, disable and remove save source only unless `--sync`
is supplied. Sync and restore always preview again before applying.
`sync hooks` is the resource-only alias; `sync --all` includes hooks and checks
native conflicts before other resources change. Its JSON output has a `hooks`
result alongside the other resource results. These are separate operations,
not one transaction.

| Option | Meaning |
|---|---|
| `--file PATH` | Entry JSON/YAML for add/edit; native configuration or code for import |
| `--from AGENT` | Native Agent format or existing Agent/account configuration to import |
| `--sync` | Save and synchronize the mutation |
| `--keep-files` | With `remove`: stop managing the hook and leave its native entries as they are. Not with `--sync`. See [below](#stop-managing-a-hook) |
| `--replace` | Explicitly replace an existing source entry or conflicting native output for that entry |
| `--dry-run`, `-n` | Preview without saving or writing native configuration |
| `--json` | Structured output |
| `--revision ID` | Require the matching preview revision |
| `--global`, `-g` | Global Skillshare configuration |
| `--project`, `-p` | Project-local Skillshare configuration |

Use `hooks list --json` to see backup IDs and exact destination paths. A changed
configuration invalidates the preview; refresh it before saving or syncing.

## Stop managing a hook {#stop-managing-a-hook}

```bash
skillshare hooks remove check --keep-files
```

This removes `check` from the source and forgets which native registrations and
files Skillshare wrote for it. No Agent file changes. From then on those entries
are yours: sync neither removes nor updates them, and `hooks import` offers them
again. `--keep-files` cannot be combined with `--sync`. The dashboard's remove
dialog offers it as **Stop managing**, on the Hooks page and in a project's
**Hooks** tab. **Remove from source only** is different: the next sync deletes
the hook's entries from the Agent files.

`--keep-files` is unavailable for Git bindings; see [Git hooks](#git-hooks).

Only the scope you remove it from changes. Stopping a global hook leaves a
project's hook of the same name managed, and the other way round. To manage the
entries again, import them.

## Source fields

Declarations live in `hooks.entries` in the selected Skillshare config. A name
identifies one entry; its `bindings` select receiving Agents and contain their
native definitions. An empty bindings map keeps the entry in Skillshare only.

```yaml
hooks:
  entries:
    check:
      description: Run the project's check after Claude finishes
      enabled: true
      bindings:
        claude:
          events:
            Stop:
              - hooks:
                  - type: command
                    command: "make check"
                    timeout: 120
```

The file passed to `hooks add check --file check.yaml` contains only the Entry:
`description`, `enabled` and `bindings`, without the `hooks.entries` wrapper.

Skillshare writes the `hooks` section in indented block style, and each save
unfolds entries an earlier edit left on one line. The **config.yaml** button on
the Hooks page opens **Settings → Files** at the `hooks:` section. There,
clicking a `hooks` key explains it in the right panel, and **Beautify** unfolds
nested one-line sections while short lists such as `targets: [claude, codex]`
stay on one line.

| Field | Meaning |
|---|---|
| `description` | Optional description |
| `enabled` | Defaults to true; false retains the source and removes unchanged owned outputs on the next sync |
| `bindings` | Map of native Agent IDs to bindings |
| `bindings.AGENT.events` | Native event map for command/configuration Agents |
| `bindings.AGENT.code` | Supplied native extension/plugin source for Pi, OMP, Amp or OpenCode |
| `bindings.git.commands` | Named Git config hooks: `events`, `command`, optional `parallel` |
| `bindings.AGENT.files` | Optional UTF-8 script files for command bindings, keyed by relative filename |

Agent IDs are `claude`, `codex`, `gemini`, `copilot`, `cursor`, `droid`, `qwen`,
`antigravity`, `pi`, `omp`, `amp`, `opencode` and `git`. `factory` is accepted as an alias for
`droid`, and `antigravity-cli` and `agy` for `antigravity`. A binding may also
name an account target declared under `targets` in global mode.
Keep event names, matchers, handler types, commands, timeout units and payloads
in each Agent's native format. Skillshare does not translate one Agent's
runtime behavior into another's. Event names are checked against each command
Agent's documented events: an unknown name, such as a misspelled `Stopp`, is a
warning in previews and in the plan's `warnings`, never a sync blocker, because
Agents add events over time. Pi, OMP, Amp and OpenCode code is not checked.

Pi, OMP, Amp and OpenCode use their own extension/plugin APIs. Supply code matching
the installed Agent version, including its imports. Skillshare writes it to a
dedicated `skillshare-NAME.ts` file without generating a universal hook runtime.

### Oh My Pi code bindings {#omp}

Use `bindings.omp.code` with the extension API of your installed OMP version.
Skillshare copies the source verbatim; it does not translate shell events or
execute the extension to validate it. OMP has no project trust prompt, so a
synced project extension can execute at the next OMP startup.

OMP's `hooks/pre` and `hooks/post` factories remain additional unmanaged sources.
They are listed but not imported into `extensions`, which would risk loading the
same factory twice. Importing an existing standalone extension also leaves its
original file in place; review the import warning before syncing another copy.

Pi and OMP both honor `PI_CODING_AGENT_DIR`. If bindings resolve to the same
output file, Skillshare blocks the plan before writing. Use separate explicit
account directories when managing both.
Script files are stored under the Agent config directory's
`hooks/skillshare/NAME/`; commands retain the native macros or explicit paths
you provide. Inspect the exact paths in the preview.

## Native destinations

| Agent | Global default | Project default | Native shape |
|---|---|---|---|
| Git | `$XDG_CONFIG_HOME/git/skillshare/hooks.gitconfig` (default `~/.config/git/…`) | `<git-common-dir>/skillshare/hooks.gitconfig` | Git 2.54+ `hook.<name>` sections |
| [Claude Code](https://code.claude.com/docs/en/hooks) | `~/.claude/settings.json` | `.claude/settings.json` | `hooks` event map with matcher groups |
| [Codex](https://learn.chatgpt.com/docs/hooks) | `~/.codex/hooks.json` | `.codex/hooks.json` | Wrapped `hooks` event map |
| [Gemini CLI](https://geminicli.com/docs/hooks/reference/) | `~/.gemini/settings.json` | `.gemini/settings.json` | `hooks` event map |
| [Copilot CLI](https://docs.github.com/en/copilot/reference/hooks-reference) | `~/.copilot/hooks/skillshare-NAME.json` | `.github/hooks/skillshare-NAME.json` | Version 1, `hooks` event map |
| [Cursor](https://cursor.com/docs/hooks) | `~/.cursor/hooks.json` | `.cursor/hooks.json` | Version 1, native lowerCamelCase events |
| [Factory Droid](https://docs.factory.com/harness/hooks) | `~/.factory/hooks.json` | `.factory/hooks.json` | Unwrapped event map |
| [Qwen Code](https://qwenlm.github.io/qwen-code-docs/en/users/features/hooks/) | `~/.qwen/settings.json` | `.qwen/settings.json` | `hooks` event map |
| [Antigravity](https://antigravity.google/docs/hooks) | `~/.gemini/config/hooks.json` | `.agents/hooks.json` | Named hook blocks, one per hook |
| [Pi](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/extensions.md) | `~/.pi/agent/extensions/skillshare-NAME.ts` | `.pi/extensions/skillshare-NAME.ts` | Native extension code |
| [Oh My Pi](https://github.com/can1357/oh-my-pi/blob/v18.6.1/docs/extensions.md) | `~/.omp/agent/extensions/skillshare-NAME.ts` | `.omp/extensions/skillshare-NAME.ts` | Native OMP extension code |
| [Amp](https://ampcode.com/docs/plugin-api) | `~/.config/amp/plugins/skillshare-NAME.ts` | `.amp/plugins/skillshare-NAME.ts` | Native plugin code |
| [OpenCode](https://opencode.ai/docs/plugins/) | `~/.config/opencode/plugins/skillshare-NAME.ts` | `.opencode/plugins/skillshare-NAME.ts` | Supplied v1/v2 plugin code |

Native config-directory environment overrides apply in global scope, unless
they point at an account's `config_dir`. Project
operations write inside that project and never fall back to a global path.
Other native sources, such as Codex inline TOML declarations, remain separate.
Antigravity and its CLI (`agy`) read the same `hooks.json`; each hook is one
block named after it, so importing a block keeps its name. Hooks in the CLI's
`~/.gemini/antigravity-cli/settings.json` stay separate.
Copilot loads project hooks from `.github/hooks` only in a trusted folder.
Sync refuses to create a Droid standalone file while active inline hooks exist.
Import them first, review and remove the original inline hooks, then sync; this
avoids silently changing which native source Droid loads.

## Another account of an Agent {#accounts}

A target declared with `agent` and `config_dir` can receive hooks under its
own name. Claude, Codex, Pi and OMP accounts use their Agent's native binding format.
The `git` key always means [Git config hooks](#git-hooks), so an account named `git`
receives no hooks, and the plan warns once an entry uses the `git` binding; give it
another name:

```yaml
targets:
  codex-2:
    agent: codex
    config_dir: ~/.codex-2
    skills: {enabled: false}
hooks:
  entries:
    check:
      bindings:
        codex-2:
          events:
            Stop:
              - hooks:
                  - type: command
                    command: "echo checked"
```

| Account's Agent | Native destination |
|---|---|
| `claude` | `<config_dir>/settings.json` |
| `codex` | `<config_dir>/hooks.json` |
| `pi` | `<config_dir>/extensions/skillshare-NAME.ts` |

Command scripts go to `<config_dir>/hooks/skillshare/NAME/`. Accounts are
global only: project configs and `hooks.projects` bind the Agent itself, because
every account reads the same project files. A missing `config_dir` is skipped
with a warning; sync never creates an account home.

If `CODEX_HOME`, `CLAUDE_CONFIG_DIR` or `PI_CODING_AGENT_DIR` points at a declared
account's directory, the plain Agent binding uses its built-in default home.
The plan warns and each account still uses its own `config_dir`. Other native
directory overrides keep their existing behavior. Directories that resolve to
the same location, including symlinks, cannot receive two binding keys in one sync.

Owned outputs whose target was removed, whose home moved, or whose account home
is missing are left in place with their ownership records and a warning. Restore
the target or directory to resume managing them. To release one entry's parked
records without changing those files, preview and run
`skillshare hooks sync NAME --replace -g`. Removing a target warns if hook
bindings still name it; remove those bindings too before the next hooks sync.

Import from an account with `skillshare hooks import --from codex-2 -g --json`.
The dashboard labels it **codex-2 (Codex)** and uses Codex's events and editor.
Each Codex home needs its own review and trust of changed hooks in `/hooks`;
sync never changes trust or executes a hook.

## Git hooks

Git bindings use `commands`, with friendly names distinct from Git event names:

```yaml
bindings:
  git:
    commands:
      project.check:
        events: [pre-commit]
        command: "{files}/check.sh"
        parallel: false
    files:
      check.sh: |
        #!/bin/sh
        exec make check
```

The dashboard edits the complete Git binding as YAML. Each command needs a
nonempty list of unique, lowercase event names and a single-line command.
Unknown events warn. Friendly names use letters, digits, `_`, `-` and `.`, begin
with a letter or digit, have at most 128 characters, and cannot contain `..`, end
in `.`, or equal a Git event name. Names must be unique across enabled entries
sharing a destination. Use distinct friendly names for global and project
commands too: Git merges both scopes. `parallel` needs Git 2.55+; older Git
produces a warning.

Git appends the event's arguments to each command and gives each hook the full
stdin. Put compound shell logic in `files`: commands ending in `fi`, `done`,
`esac`, `}`, or `)` are rejected because argument appending breaks them.
`{files}` expands to a quoted absolute helper directory, requires `files`, and
is regenerated for each machine. Helpers are UTF-8, normalized to LF and written
executable. Commands retain native Git semantics and do not have Agent timeouts.
Ordering is deterministic: entry names first, then friendly names; Git controls
execution and parallelism. Skillshare leaves `hook.jobs` settings alone.

### Includes and activation

Global outputs use `$XDG_CONFIG_HOME/git/skillshare/` (default
`~/.config/git/skillshare/`). The include is added to `GIT_CONFIG_GLOBAL` when
set, otherwise an existing `~/.gitconfig`, otherwise an existing
`$XDG_CONFIG_HOME/git/config`, otherwise a new `~/.gitconfig`.
`GIT_CONFIG_GLOBAL` must be absolute. Includes inside HOME use `~/` paths.

Project outputs and helpers live under Git's common directory. The include is
`skillshare/hooks.gitconfig` in its `config` file. Linked worktrees share this
destination; declare one root per common directory. Roots must be non-bare
repository top levels. Unavailable roots or a missing Git executable skip Git
outputs and retain their ownership records for a later retry.

Skillshare edits regular, writable include targets using Git's native lock.
It never writes through a symlink. An absent include in a symlinked or
unwritable target yields **inactive** with the exact lines to add manually:

```gitconfig
[include]
    path = ~/.config/git/skillshare/hooks.gitconfig
```

Generated files can still be written while inactive. Git below 2.54 also reports
inactive: there is no fallback dispatcher. Plans keep the actual `add`, `update`,
`remove` or `unchanged` action and report the activation warning separately in
`inactiveReason`. After sync, inactive outputs do not remain pending writes.
Manual includes stay user-owned,
including conditional `includeIf` lines; Skillshare never adds an unconditional
include to widen their scope, including nested includes beneath inactive
conditions. Unreadable or excessively nested include declarations fail closed.
Global conditional activation is unknown until Git evaluates it in a repository. Project activation is checked from effective
config origins. A `present` include does not alone prove activation.
`hooks list -g --json` exposes the `git` capability/include object and
`projectGit` for declared roots, including manual lines and `core.hooksPath`.

### Conflicts, removal and migration

The generated `hooks.gitconfig` is owned as a whole file. Unmanaged or externally
edited content conflicts even if identical. `--replace` explicitly adopts
identical content or regenerates the whole file, discarding outside edits after
backup. Active ownership by another config still blocks replacement. Friendly
name collisions check command/event definitions across effective scopes and
includes. Replacement can remove only that name's section in the writable
regular target config; system or included-file collisions remain conflicts.
`enabled=false` overrides warn rather than being overwritten. Previews and
section backups expose only the affected hooks/include values, preserving
unrelated private config.

Disable or remove with `--sync` removes owned commands, helpers and includes,
then prunes only directories Skillshare created. Manual includes remain.
Backups can restore the whole generated file, helpers or a narrow config
operation while preserving unrelated later edits. Source definitions do not
change during restore. `remove --keep-files` is refused for any Git binding:
commands share one generated file, so detaching an entry is ambiguous.

Git config hooks coexist with hooks-directory scripts. Inspect existing
`core.hooksPath` and hooks-directory registrations before migrating a tool;
config hooks run before the hooks-directory hook and duplicates can run twice.
Removing old copies is a separate user action. Git import, hooks-directory
recognizers, script-mode bindings, the structured Git editor and doctor checks
are reserved for later phases. This implementation supports config commands
with inline helpers only. POSIX shell quoting and Windows-style forward-slash
paths are rendered; native Windows hook execution is unverified.

See the [Roborev recipe](../../how-to/recipes/git-hooks.md) for portable wrappers.

## Projects, conflicts and recovery

In a global config, `hooks.projects` maps absolute project roots to an `entries`
mapping using the same Entry format. Manage these in **Projects → Hooks**.
A project with its own `.skillshare/config.yaml` must be managed in project
scope. Project-only sync applies that project's hook changes.

Sync preserves unrelated settings and unowned hooks. Matching content alone
does not establish ownership. Modified owned outputs are conflicts, including
on disable, remove and restore. Use the preview to inspect exact actions; an
explicit replacement applies only to the selected entry. For a shared file, each
plan line lists the events the entry adds (`+`), updates (`~`) and removes (`−`),
also available as `events` in JSON plans. `update` means the entry keeps
registrations in that file; `remove` means it leaves the file entirely. Edits keep
the file's style, compact or indented, and a `hooks` key Skillshare added is
removed again when its last hook leaves. A file Skillshare created is deleted
once nothing else is left in it, together with the folders it created that are
now empty; the preview shows this as a file deletion. Files and folders that
existed before, and anything you added, stay. An externally edited
output that can no longer be identified safely is released from ownership and
left untouched; inspect the preview before publishing another registration.

Backups restore native outputs while preserving unrelated later edits; they do
not replace your source definition. Use **Settings → Backups → Hooks** or
`hooks restore` to preview and restore one backup.

**Synced** describes configuration written by Skillshare. Restart or reload the
Agent as its native workflow requires; native trust, enabling hooks and code
compatibility remain controlled by that Agent. Skillshare never executes hook
commands during management or changes native trust automatically.
