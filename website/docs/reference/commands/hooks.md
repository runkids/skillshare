---
sidebar_position: 4
---

# hooks

Manage named hooks and synchronize each receiving Agent's or Git's native configuration.
Use the dashboard's **Hooks** page to add, edit, import, enable or disable entries,
then preview before syncing. Hooks also appear in destination **Targets** and
**Projects**, **Sync**, and **Settings → Backups**. The **Hooks** page works only
when the browser opens the dashboard by `localhost`, an IP address or a trusted
origin; see [Reverse Proxy](/docs/reference/commands/ui#reverse-proxy).

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
skillshare hooks sync check lint --replace --dry-run
skillshare hooks sync --trust -g
skillshare hooks sync --trust --yes -g
skillshare hooks sync --trust --yes --json -g
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
and events you did not import stay unmanaged. When importing directly from a
target's native folder (without `--file`), flat code files (Pi, Oh My Pi, Amp,
OpenCode) and Copilot hook files write `skillshare-NAME` on sync and retire the
imported original file after taking a backup (an existing `skillshare-NAME`
file under the same name is adopted in place); `hooks restore` can bring a
retired original back, whereas `--file` imports read the supplied file without
retiring it. Import only reads configuration or code and never runs it.

`hooks list` also shows sources an Agent loads that import never reads: Codex
inline `[hooks]` tables in `config.toml`, OMP's `hooks/pre` and `hooks/post`,
Droid's legacy `hooks/hooks.json`, Claude's `settings.local.json` in a project,
and Git config hooks. They show as `left as is` instead of `not managed`, carry
`"importable": false` in `--json`, and appear in the dashboard without a Review
button.

The dashboard's **Hooks in targets** dialog opens with no hook selected; tick the ones to
import. It can also edit or remove a hook in its Agent's
settings file without importing it. This works for every Agent whose hooks are
configuration: events in one settings file (Claude, Codex, Gemini, Cursor, Qwen,
and Droid's `hooks.json`), an Antigravity named block, or a Copilot hooks file,
where removing the hook deletes that file. Hooks that are code (Pi, Oh My Pi,
Amp, OpenCode) must be imported first. The dialog shows the file diff before
anything is written and backs the file up; the hook stays unmanaged.

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
| `--replace` | Explicitly replace an existing source entry or conflicting native output for that entry. `hooks sync` accepts several names to take them over together |
| `--trust` | With `hooks sync`: also record Codex trust for the hooks Skillshare owns. See [Codex trust](#codex-trust) |
| `--yes`, `-y` | With `--trust`: skip the question and trust the listed hooks. See [Codex trust](#codex-trust) |
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
| `bindings.AGENT.codeFile` | Instead of `code`: a path, relative to the config's folder, whose contents are used as `code` |
| `bindings.git.commands` | Named Git config hooks: `events`, `command`, optional `parallel` |
| `bindings.AGENT.files` | Optional UTF-8 script files for command bindings, keyed by relative filename; a value is the contents or `{from: path}`, read like `codeFile` |

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
dedicated `skillshare-NAME.ts` file (OpenCode v1), or an OpenCode v2 directory package, without generating a universal hook runtime.

Longer code and scripts can live in their own files, which a tsconfig, shellcheck or
an editor can check. Use `codeFile` instead of `code`, and `{from: path}` instead of a
`files` value; inline values keep working. Both are read every time the config is
read, so edit the file and run `hooks sync`; a change made after a preview fails the
apply as stale. A path resolves against the folder of the config that holds the
entry (`.skillshare/` in a project, the global config's folder for `hooks.projects`;
with `hooks add|edit --file`, the selected config's folder, not the entry file's),
must stay inside it, and must not pass through a symlink. A missing file stops the
config from loading, like any other invalid entry. Saving an entry keeps the
references. Editing the code or a script in the dashboard saves it inline. JSON
(`hooks list --json`, the Web API) carries the contents in `code` and `files`, with
the references in `codeFile` and `filesFrom`; an entry sent back with both must send
the file's current contents. An entry JSON file may also write a `files` value as
`{"from": path}`.

```yaml
hooks:
  entries:
    lint:
      bindings:
        pi:
          codeFile: hooks/lint.pi.ts
        cursor:
          events:
            afterFileEdit:
              - command: .cursor/hooks/skillshare/lint/lint.sh
          files:
            lint.sh: { from: hooks/lint.sh }
```

### Oh My Pi code bindings {#omp}

Use `bindings.omp.code` with the extension API of your installed OMP version.
Skillshare copies the source verbatim; it does not translate shell events or
execute the extension to validate it. OMP has no project trust prompt, so a
synced project extension can execute at the next OMP startup.

OMP's `hooks/pre` and `hooks/post` factories remain additional unmanaged sources.
They are listed but not imported into `extensions`, which would risk loading the
same factory twice. Importing an existing standalone extension from the target
(without `--file`) adopts it: the next sync writes `skillshare-NAME.ts` and
deletes the original file after taking a backup (an existing `skillshare-NAME.ts`
under the same name is adopted in place), and `hooks restore` can bring the
original file back.

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
| [OpenCode](https://opencode.ai/docs/plugins/) | `~/.config/opencode/plugins/skillshare-NAME[.ts]` | `.opencode/plugins/skillshare-NAME[.ts]` | v1 file; v2 directory package (see below) |

Native config-directory environment overrides apply in global scope, unless
they point at an account's `config_dir`. Project sync writes native hook files inside the checkout. Confirming `--trust` separately records trust in each verified user’s Codex home.
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
| `omp` | `<config_dir>/extensions/skillshare-NAME.ts` |

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
sync never changes trust or executes a hook unless you pass `--trust`.

### Codex trust {#codex-trust}

Codex runs a user-level hook only after it is trusted, and keeps that trust in
the same home's `config.toml` as `hooks.state."<hooks.json>:<event>:<group>:<handler>".trusted_hash`.
The key is the hook's position, so a hook Skillshare adds or moves starts
untrusted, and `codex exec` skips it.

```bash
skillshare hooks sync --trust -g
skillshare hooks sync --trust --yes -g
skillshare hooks sync --trust --yes --json -g
```

`--trust` first writes the hook files as a normal sync does. Then it asks Codex which of the hooks Skillshare owns need trust, in every Codex home it manages, account homes included. It gets each hook's hash from `codex app-server`, the same calls the `/hooks` review uses, so `codex` must be on `PATH`. Skillshare never computes the hash.

Hooks that are already trusted are reported as `unchanged` and need no answer. If a hook is new, or its command changed since it was trusted, the command lists those hooks with their commands and asks once. If nothing needs trust, it does not ask.

```
Codex trust
  stop  needs trust  codex  /home/me/.codex/hooks.json:stop:1:0
  /usr/local/bin/notify-done

? Trust 1 hook in Codex?  Yes  No
```

After you confirm:

```
Codex trust
✓ stop  trusted  codex  /home/me/.codex/hooks.json:stop:1:0
```

`--yes` (`-y`) skips the question. Codex's review exists so that a person sees the command before it runs. Without a terminal and without `--yes`, nothing is trusted: the command lists the hooks and exits with an error that names `--yes`. `--json` never asks either: without `--yes` it reports the hooks as `needed` and exits non-zero. Project JSON includes the reason in `codexTrustError` and the destination `home` on each verified hook. `--yes` is only valid with `--trust`. The `--json` output has a `codexTrust` list. Each item has `target`, `entry`, `key`, `command`, `action` (`needed`, `trusted` or `unchanged`), and `disabled` when true.

Only `trusted_hash` is written, and only ever added. A hook you disabled in Codex stays disabled and is reported. `enabled`, the rest of `config.toml` and hooks Skillshare does not own are left alone. When a hook moves or is removed, its old entry stays in `config.toml`. It matches only a hook with the same event, matcher and handler at that position, so it cannot trust a different hook.

If `codex` is not on `PATH` or Codex fails, the hook files are still written and the command fails with "hook files were written, but Codex trust was not recorded". With several Codex homes, a home that Codex cannot list, or whose write fails, does not stop the others: the error names the homes that were recorded and the ones that were not. Plain `hooks sync` never starts Codex and never changes trust. Use `skillshare hooks sync -p --trust` to review this checkout’s owned hooks. Trust is recorded in the listed user homes, not in the checkout. Each home must already trust the folder in Codex. A home that cannot verify every owned command is skipped; use its native `/hooks` to review. If other homes were updated, the result reports partial completion (non-zero exit; `codexTrustError` in project JSON output). `sync hooks` and `hooks list` reject `--trust`. Hooks that `hooks.projects` writes into a project are not included; trust them in Codex's `/hooks`. The dashboard's Hooks page (global or project mode) has the same action: it lists the hooks that need trust with their commands and records trust only after you confirm in the dialog.

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
mapping using the same Entry format. Manage these in **Projects → Hooks**; **Stop managing hooks here**
there takes the root out of `hooks.projects`, and the next sync removes the hooks
Skillshare wrote under it.
A project with its own `.skillshare/config.yaml` must be managed in project
scope. Project-only sync applies that project's hook changes. See the
[session start recipe](../../how-to/recipes/session-start-command.md) for one
project Entry with a binding for each Agent.

Sync preserves unrelated settings and unowned hooks. Matching content alone
does not establish ownership. Modified owned outputs are conflicts, including
on disable, remove and restore. Use the preview to inspect exact actions; an
explicit replacement applies only to the named entries, and the sync stays
blocked while any other entry conflicts. When several hooks conflict, name them
all: `skillshare hooks sync NAME1 NAME2 --replace`. For a shared file, each
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
commands during management or changes native trust automatically; only
[`--trust`](#codex-trust) records Codex trust, when you ask for it.

### OpenCode v2 directory packages

On Windows, the hook version probe resolves npm's standard native `.cmd` shims for `opencode-ai` (later v1) and `@opencode/cli` (v2) directly to their installed `bin/opencode.exe`, without a shell. Older v1 Node-based launchers and custom batch wrappers are not inferred or executed. For those installations, set the OpenCode account's `cli` to the absolute path of its installed native `opencode.exe` (for example, the matching `opencode-windows-x64-baseline` package's `bin/opencode.exe`), or put that native executable's directory first on `PATH`. Skillshare does not run postinstall scripts or repair missing binaries.

Skillshare probes only `opencode --version` (or the account's `cli`). When the version cannot be detected, because the binary is missing or fails or reports a major version other than 1 or 2, only the OpenCode bindings are skipped: the preview shows a warning, existing OpenCode output is left as it is, and every other Agent syncs normally. v1 keeps the flat `.ts` file. v2 writes `plugins/skillshare-NAME/index.ts` plus `package.json`, declaring `@opencode/plugin: ^2.0.0`, and adds the directory's `file://` URL to `plugins` in `opencode.json(c)`. Project hooks use `.opencode/opencode.json(c)`. Comments and unrelated registrations remain unchanged. Conflicting JSON/JSONC files or `OPENCODE_CONFIG`/`OPENCODE_CONFIG_CONTENT` overrides are refused for hooks. A `code` binding is written as given on either version; Skillshare does not check that it matches the v1 or v2 plugin API.

For a complete v2 package, use `bindings.opencode.package` instead of `code`/`codeFile`: a map of relative filenames to UTF-8 contents, including `package.json`. The manifest and its dependency requirements are preserved verbatim. Nested files are allowed; traversal, symlinks, file/directory collisions, `.git`, and `node_modules` are not package payloads. The native editor displays this file map as JSON. The server entry must be a string root or `./server` export with a package name, or a conventional root `server`/`index` file (`.ts`, `.tsx`, `.js`). Conditional and fallback server exports are refused rather than guessed. A `package` binding needs OpenCode v2; on v1 it is skipped with a warning.

`hooks import --from opencode` also reads immediate package directories under the native `plugins` folder. Unlike flat code imports from the target folder (which automatically retire the imported file after taking a backup), importing a package directory copies its source files to Skillshare's named output and warns you to remove the original directory and registration after syncing to avoid duplicate loading. Installed `node_modules` and `.git` are not imported. A package holding a file that cannot be read, such as a symlink, is listed with that problem; the other packages and flat files stay importable. On OpenCode v1, directories under `plugins` are not listed as hooks. Sync, edits and removal use the same ownership checks and backups as other hooks; an externally edited package file conflicts rather than being overwritten.

Provision declared dependencies yourself before loading; the preview says so whenever it writes package files. Skillshare never runs plugin code, installs dependencies, or converts v1 code to v2. Registration is not proof the plugin loaded. The dashboard intentionally supplies no OpenCode starter code because the two APIs differ.
