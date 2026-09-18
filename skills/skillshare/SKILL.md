---
name: skillshare
description: |
  Manage skills, agents, extras, plugins, and MCP connection settings with the Skillshare CLI.
  Use when the user asks to configure or run Skillshare, install or sync resources
  across AI tools, import MCP settings, manage targets, audit skills, recover backups,
  or troubleshoot Skillshare configuration and sync. Covers global and project modes,
  noninteractive automation, and guidance for the terminal UI.
argument-hint: "[command] [target] [--json] [--dry-run] [-p|-g]"
metadata:
  version: v0.21.0
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
- MCP uses its own receiving targets and source definitions. Read [mcp.md](references/mcp.md)
  before editing MCP settings or importing native configurations. Pi requires an explicit
  MCP extension choice; follow its setup instructions in that reference.
- Plugins keep their native components together. Read [plugins.md](references/plugins.md)
  for installation, import, sync selection, updates, and native compatibility limits
  across Claude, Codex, Cursor, Antigravity, Pi, and OpenCode.

## Execution rules

1. **Choose noninteractive inputs.** Supply names and selection flags supported by the
   specific command. Use `--no-tui` for plain output or `--json` where supported.
   Do not add `--force` simply to avoid a prompt: it can overwrite conflicts or override audits.
2. **Keep previews separate from writes.** Use `--dry-run` where supported. After changing
   skills or agents, sync the intended resource and scope. MCP mutations save source only
   unless `--sync` is supplied; `sync --all` also includes extras and MCP, but excludes plugins.
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

### Install, update, and sync skills

```bash
skillshare install user/repo -s pdf,commit
skillshare install user/repo --track -b develop --all
skillshare sync
skillshare check --json
skillshare update my-skill
skillshare sync
```

For project setup, use `init -p`, then `install -p` and `sync -p`. Rehydrate configured
remote dependencies with `install` without a source. For full flag details and agents,
read [install.md](references/install.md) and [sync.md](references/sync.md).

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

```bash
skillshare plugin list --json
skillshare plugin add ./my-plugin --target claude --dry-run --json
skillshare plugin disable demo --target claude --no-tui
skillshare sync plugins demo --dry-run --json
```

Run only the requested operation. `add` installs a whole native package; `import`
adopts an existing installation. Plugin enable/disable saves sync selection only;
the next `sync plugins` installs or removes the managed target. It is excluded
from `sync --all`. Inspect `targetDefinitions` for supported operations and scopes;
format discovery alone does not mean installation is supported. Read [plugins.md](references/plugins.md) before applying changes.

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
## References

Read the matching file for command flags, examples, and limitations. Avoid loading all
references for a single task.

| Topic | File |
|-------|------|
| Native agents, selection, ignore rules, and recovery | [native-agents.md](references/native-agents.md) |
| MCP configuration, imports, TUI, and recovery | [mcp.md](references/mcp.md) |
| Complete plugins, native lifecycle, and sync selection | [plugins.md](references/plugins.md) |
| Init flags | [init.md](references/init.md) |
| Sync/collect/commit/push/pull | [sync.md](references/sync.md) |
| Install/update/uninstall/new | [install.md](references/install.md) |
| Status/diff/list/search/check | [status.md](references/status.md) |
| Security audit | [audit.md](references/audit.md) |
| Trash | [trash.md](references/trash.md) |
| Operation log | [log.md](references/log.md) |
| Targets | [targets.md](references/targets.md) |
| Extras (rules/commands/prompts) | [extras.md](references/extras.md) |
| Backup/restore | [backup.md](references/backup.md) |
| Troubleshooting | [TROUBLESHOOTING.md](references/TROUBLESHOOTING.md) |
