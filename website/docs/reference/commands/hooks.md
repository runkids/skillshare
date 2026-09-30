---
sidebar_position: 4
---

# hooks

Manage named hooks and synchronize each receiving Agent's native configuration.
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
| `--from AGENT` | Native Agent format or existing Agent configuration to import |
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
| `bindings.AGENT.code` | Supplied native extension/plugin source for Pi, Amp or OpenCode |
| `bindings.AGENT.files` | Optional UTF-8 script files for command bindings, keyed by relative filename |

Agent IDs are `claude`, `codex`, `gemini`, `copilot`, `cursor`, `droid`, `qwen`,
`antigravity`, `pi`, `amp` and `opencode`. `factory` is accepted as an alias for
`droid`, and `antigravity-cli` and `agy` for `antigravity`.
Keep event names, matchers, handler types, commands, timeout units and payloads
in each Agent's native format. Skillshare does not translate one Agent's
runtime behavior into another's. Event names are checked against each command
Agent's documented events: an unknown name, such as a misspelled `Stopp`, is a
warning in previews and in the plan's `warnings`, never a sync blocker, because
Agents add events over time. Pi, Amp and OpenCode code is not checked.

Pi, Amp and OpenCode use their own extension/plugin APIs. Supply code matching
the installed Agent version, including its imports. Skillshare writes it to a
dedicated `skillshare-NAME.ts` file without generating a universal hook runtime.
Script files are stored under the Agent config directory's
`hooks/skillshare/NAME/`; commands retain the native macros or explicit paths
you provide. Inspect the exact paths in the preview.

## Native destinations

| Agent | Global default | Project default | Native shape |
|---|---|---|---|
| [Claude Code](https://code.claude.com/docs/en/hooks) | `~/.claude/settings.json` | `.claude/settings.json` | `hooks` event map with matcher groups |
| [Codex](https://learn.chatgpt.com/docs/hooks) | `~/.codex/hooks.json` | `.codex/hooks.json` | Wrapped `hooks` event map |
| [Gemini CLI](https://geminicli.com/docs/hooks/reference/) | `~/.gemini/settings.json` | `.gemini/settings.json` | `hooks` event map |
| [Copilot CLI](https://docs.github.com/en/copilot/reference/hooks-reference) | `~/.copilot/hooks/skillshare-NAME.json` | `.github/hooks/skillshare-NAME.json` | Version 1, `hooks` event map |
| [Cursor](https://cursor.com/docs/hooks) | `~/.cursor/hooks.json` | `.cursor/hooks.json` | Version 1, native lowerCamelCase events |
| [Factory Droid](https://docs.factory.com/harness/hooks) | `~/.factory/hooks.json` | `.factory/hooks.json` | Unwrapped event map |
| [Qwen Code](https://qwenlm.github.io/qwen-code-docs/en/users/features/hooks/) | `~/.qwen/settings.json` | `.qwen/settings.json` | `hooks` event map |
| [Antigravity](https://antigravity.google/docs/hooks) | `~/.gemini/config/hooks.json` | `.agents/hooks.json` | Named hook blocks, one per hook |
| [Pi](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/extensions.md) | `~/.pi/agent/extensions/skillshare-NAME.ts` | `.pi/extensions/skillshare-NAME.ts` | Native extension code |
| [Amp](https://ampcode.com/docs/plugin-api) | `~/.config/amp/plugins/skillshare-NAME.ts` | `.amp/plugins/skillshare-NAME.ts` | Native plugin code |
| [OpenCode](https://opencode.ai/docs/plugins/) | `~/.config/opencode/plugins/skillshare-NAME.ts` | `.opencode/plugins/skillshare-NAME.ts` | Supplied v1/v2 plugin code |

Native config-directory environment overrides apply in global scope. Project
operations write inside that project and never fall back to a global path.
Other native sources, such as Codex inline TOML declarations, remain separate.
Antigravity and its CLI (`agy`) read the same `hooks.json`; each hook is one
block named after it, so importing a block keeps its name. Hooks in the CLI's
`~/.gemini/antigravity-cli/settings.json` stay separate.
Copilot loads project hooks from `.github/hooks` only in a trusted folder.
Sync refuses to create a Droid standalone file while active inline hooks exist.
Import them first, review and remove the original inline hooks, then sync; this
avoids silently changing which native source Droid loads.

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
