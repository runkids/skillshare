# Native Agents

Use native agents for standalone Markdown agent definitions. They are separate from
skills (directories containing `SKILL.md`) and from MCP connection settings.

## Select the resource explicitly

```bash
skillshare install org/agents --kind agent --all
skillshare install org/agents --agent reviewer,tutor
skillshare list agents --json
skillshare check agents --json
skillshare update agents reviewer
skillshare collect agents claude --dry-run
skillshare sync agents --dry-run
skillshare sync agents
```

Add `-p` for project resources or `-g` for global resources. Install uses `--kind agent`
or `--agent`; resource-aware commands such as list, check, update, collect, and sync
accept positional `agents`. Do not assume every command has identical selectors.

## Sources, targets, and filters

Read `sources.agents` and each target's `agents` configuration before choosing paths.
Only targets with an agents path receive native agents. Configure filters with
`target <name> --add-agent-include` / `--add-agent-exclude`, and agent sync mode with
`--agent-mode`; see [targets.md](targets.md).

Agent discovery uses `.agentignore`. Toggle an agent without deleting its source:

```bash
skillshare disable reviewer --kind agent --dry-run
skillshare disable reviewer --kind agent
skillshare enable reviewer --kind agent
skillshare sync agents
```

Choose disable or enable as requested, then sync. Quote patterns such as `'draft-*'`.
Native agent targets do not run extras extensions. Use extras only if the requested
output needs flattening or transformation; see [extras.md](extras.md).

## Removal and recovery

```bash
skillshare uninstall agents reviewer --dry-run
skillshare uninstall agents reviewer
skillshare trash agents list --no-tui
skillshare trash agents restore reviewer
skillshare sync agents
```

Uninstall and restore are alternative operations, not a single sequence. Review the
selection before confirming removal. Source recovery uses agent trash; target recovery
uses `backup agents` / `restore agents <target>`. Project agent backup/restore also
supports `-p`. See [trash.md](trash.md) and [backup.md](backup.md).
