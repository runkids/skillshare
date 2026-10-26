---
name: skillshare
description: |
  Manage skills, agents, extras, hooks, plugins, and MCP connection settings with the Skillshare CLI.
  Use when the user asks to configure or run Skillshare, install or sync resources
  across AI tools, manage shared memory notes, review a session for lessons or dream over saved memories,
  import MCP settings, manage targets and project folders, audit skills, recover backups,
  or troubleshoot Skillshare configuration and sync. Covers global and project modes,
  noninteractive automation, and guidance for the terminal UI.
argument-hint: "[command] [target] [--json] [--dry-run] [-p|-g]"
metadata:
  version: v0.27.0
---

# Skillshare CLI

Use Skillshare to maintain a source and distribute resources to configured targets.
Read only the reference relevant to the requested operation; the routing table is below.
Check `skillshare <command> --help` if the installed version differs from these examples.

## Scope and resource selection

- Global config: `~/.config/skillshare/config.yaml`. Project config: `.skillshare/config.yaml`
  or `skillshare/config.yaml`; hidden wins when both exist. Use `-g` or `-p` explicitly
  when location matters instead of relying on auto-detection.
- Skills are directories containing `SKILL.md`; agents are single Markdown files.
  Source paths can be customized. Use the configured sources instead of assuming defaults.
- Use native agents targets for agents. Use extras for arbitrary file resources or when
  flattening/content transformation is required; `extension:` works only on extras targets.
- Shared memory notes live in the `memory` extra. Use `extras memory` to manage Markdown
  notes and reading guidance; nested paths such as `wiki/architecture.md` are supported.
  Read [extras.md](references/extras.md) for scope, version checks, and agent setup.
  For a session retrospective or Dreaming across saved notes, read
  [memory-review.md](references/memory-review.md). The host agent performs the review;
  Skillshare never launches or schedules an agent.
- MCP uses its own receiving targets and source definitions. Read [mcp.md](references/mcp.md)
  before editing MCP settings or importing native configurations. Its Agent notes cover
  what is specific to Claude Code, Codex, OpenCode and Pi: scopes and files, names and
  credentials they refuse, Pi's built-in MCP and `piOptions`, OMP's native MCP and preserved enable/disable lists, the portable `tools` policy,
  upgrading Pi settings from 0.22, turning a global server off in one project, `mcp.projects` for many folders from
  one config, and `mcp serve` for Agents that cannot reach the synced skill folders.
- Plugins keep their native components together. Read [plugins.md](references/plugins.md)
  for installation, import, sync selection, updates, and native compatibility limits
  across Claude, Codex, Cursor, Antigravity, Pi, OMP, and OpenCode, including Pi packages from pi.dev
  and OMP marketplace plugins and standalone extension selection.
- Hooks preserve each Agent's native event map or extension/plugin code. Read
  [hooks.md](references/hooks.md) before import, synchronization or recovery.

## Execution rules

1. **Choose noninteractive inputs.** Supply names and selection flags supported by the
   specific command. Use `--no-tui` for plain output or `--json` where supported.
   Do not add `--force` simply to avoid a prompt: it can overwrite conflicts or override audits.
2. **Keep previews separate from writes.** Use `--dry-run` where supported. After changing
   skills or agents, sync the intended resource and scope. MCP and hooks mutations save source only
   unless `--sync` is supplied; `sync --all` also includes extras, MCP and hooks, but excludes plugins.
3. **Inspect audit blocks.** Review findings before choosing an override. `install --json`
   permits overwrite and selects all when no skill/agent filter is given, but retains
   the audit gate; it is not merely an output format. Read [install.md](references/install.md)
   and [audit.md](references/audit.md) before using it for automation.
4. **Use the matching recovery mechanism.** Uninstalled skills go to trash; native MCP
   entries use MCP backups. Do not substitute filesystem deletion for CLI uninstall.
5. **Keep credentials out of MCP definitions.** Use `{fromEnv: VARIABLE}` references.
   Managing settings does not require resolving secrets, copying OAuth credentials,
   starting servers, or probing their tools.
6. **Respect operation scope.** Local configuration work does not imply permission to
   commit, push, or publish. Run those commands when included in the user's request.

## Common workflows

### Inspect and diagnose

```bash
skillshare status --json
skillshare list --json
skillshare diff --json
skillshare doctor --json
skillshare mcp --json
```

`doctor` exits 1 when it finds errors. Audit uses `--format json`; JSON flags and
TUI availability vary by command. See the relevant reference rather than assuming parity.

### Track an upstream repository or install selected skills

Choose the resource and scope before installing. Use `-g` for user-wide resources
or `-p` for this project; keep that scope on preview, apply, update, and sync.

| Intent | Command |
|--------|---------|
| Keep an upstream skill collection as one Git checkout | `skillshare install owner/repo --track --name team -g` |
| Install only named skills with their source metadata | `skillshare install owner/repo -s pdf,commit -g` |
| Install all discovered skills individually | `skillshare install owner/repo --all -g` |
| Manage a complete native plugin, including hooks/MCP | Use `plugin discover` and `plugin add`; see below |

For the scoped preview → apply → update → sync workflow, read
[install.md](references/install.md#track-a-collection-without-a-picker). It covers
following the latest upstream branch, rejected flag combinations, and target filters.

For project setup, use `init -p`, then `install -p` and `sync -p`. Rehydrate configured
remote dependencies with `install` without a source. For full flag details and agents,
read [install.md](references/install.md) and [sync.md](references/sync.md).

To use a local skills checkout in place, run `skillshare link <path> --enable`, then
`sync`; `skillshare unlink <name>` removes only the link. Read
[TROUBLESHOOTING.md](references/TROUBLESHOOTING.md) for how such links are followed.

### Disable or remove

```bash
skillshare disable 'draft-*' --dry-run
skillshare disable 'draft-*'
skillshare enable 'draft-*'
skillshare uninstall my-skill
skillshare sync
skillshare trash restore my-skill
skillshare sync
```

Run the requested operation, not this entire example sequence. Quote glob patterns so
Skillshare receives them unchanged. Disabling uses ignore rules; uninstall uses trash.

### Move skills into folders

```bash
skillshare move my-skill frontend --dry-run
skillshare move my-skill other-skill frontend
skillshare sync
```

Use `move` instead of `mv` for installed skills: it keeps their install records, so
nothing is downloaded again. It does not sync; run `sync` afterwards. Read
[install.md](references/install.md) for refusals and project mode.

### Project folders from the global config

```bash
skillshare project list --json
skillshare project add ~/work/app --target claude,codex --skills-mode copy --dry-run
skillshare project add ~/work/app --target claude,codex --skills-mode copy --sync
skillshare project edit app --skills-include 'frontend-*'
skillshare project sync app --json
skillshare project remove app --dry-run --json
```

`project` always uses the global config, including inside a repo; it does not accept
`-p`. Edit filters replace lists. Removal detaches skills and keeps agent files;
JSON removal requires `--yes` unless previewing. Read [targets.md](references/targets.md)
for saved-then-sync failures, retries, and partial removal results.

### MCP settings

```bash
skillshare mcp import --from claude --json
skillshare mcp add docs --url https://example.com/mcp --target claude --no-tui
skillshare sync mcp --dry-run --json
skillshare sync mcp
```

`--target` is singular and repeatable. A preview does not apply changes. For revisions,
editing, import conflicts, credentials, backups, and human-operated TUI commands, read
[mcp.md](references/mcp.md).

### Complete plugins

Use `plugin discover` and `plugin add` for a complete native package; `plugin import`
adopts an existing native installation. Run only the requested operation. Read
[plugins.md](references/plugins.md) before applying changes; it covers the scoped
upstream workflow, supported targets, and source-ref rules.
`plugin check/update` handles upstream changes; `sync plugins` reconciles target
selection. Plugins are excluded from `sync --all`.

### Skill hubs

```bash
skillshare hub add https://example.com/hub.json          # Save a hub source
skillshare hub add https://example.com/hub.json --label my-hub  # With custom label
skillshare hub add git@ghe.corp.com:team/skills.git --label ghe  # SSH/private/GHE hub source
skillshare hub list                                      # List saved hubs
skillshare hub default my-hub                            # Set default hub
skillshare hub remove my-hub                             # Remove a hub
skillshare hub index --source ~/.config/skillshare/skills/ --full --audit  # Build hub index
```
The active `follow_source_links` setting also applies to `hub index` (including
`--source`), built-in dashboard search, and Hub draft candidates.

## References

Read the matching file for command flags, examples, and limitations. Avoid loading all
references for a single task.

| Topic | File |
|-------|------|
| Native agents, selection, ignore rules, and recovery | [native-agents.md](references/native-agents.md) |
| MCP configuration, imports, TUI, and recovery | [mcp.md](references/mcp.md) |
| Native hooks, code bindings, scopes and recovery | [hooks.md](references/hooks.md) |
| Complete plugins, native lifecycle, and sync selection | [plugins.md](references/plugins.md) |
| Init flags | [init.md](references/init.md) |
| Sync/collect/commit/push/pull | [sync.md](references/sync.md) |
| Install/update/uninstall/move/new | [install.md](references/install.md) |
| Status/diff/list/search/check | [status.md](references/status.md) |
| Security audit | [audit.md](references/audit.md) |
| Trash | [trash.md](references/trash.md) |
| Operation log | [log.md](references/log.md) |
| Targets | [targets.md](references/targets.md) |
| Extras (rules/commands/prompts) | [extras.md](references/extras.md) |
| Session retrospectives and Dreaming over saved notes | [memory-review.md](references/memory-review.md) |
| Backup/restore | [backup.md](references/backup.md) |
| Troubleshooting | [TROUBLESHOOTING.md](references/TROUBLESHOOTING.md) |
