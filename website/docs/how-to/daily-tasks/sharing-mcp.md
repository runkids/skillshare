---
sidebar_position: 10
---

# Set up MCP once for your Agents

MCP lets an Agent use tools supplied by another program or service. Skillshare
stores the connection settings once and writes each supported Agent's native
configuration. It does not run a gateway or keep a background server alive.

Supported MCP clients are Claude Code, Codex, Cursor, VS Code, OpenCode and the
official Grok CLI. Pi requires a separate MCP extension and is not supported yet.

## Start with the guided setup

Initialize Skillshare first if this is a new installation, then run:

```bash
skillshare mcp add
```

Paste the URL or JSON supplied by your MCP provider, give it a name, select your
Agents, and review the changes. **Save and sync** applies the settings immediately;
**Save only** keeps the definition for a later `skillshare sync mcp`.

In the dashboard, open **MCP → Add MCP**. Choose a URL, pasted configuration, or
an existing Agent configuration. The dashboard uses the same source, validation,
preview and conflict rules as the CLI. The Sync page also has **Sync all resources**
for skills, agents, extras and MCP.

The Config editor formats YAML when you save, using two-space indentation and
preserving comments. Click a field to see its explanation in the right panel,
including `mcp`, `sources.mcp`, connection fields and environment references.

After syncing, reload your Agent. Complete any login or approval in that Agent.
Skillshare does not test the connection, install the server program, or copy login
sessions. A successful sync means the configuration was written, not that a tool
call has succeeded.

## Understand the two connection types

| Provider gives you | Connection | Example |
|---|---|---|
| A command and arguments | `stdio`: the Agent starts a local process | `command: npx` plus `args` |
| An MCP endpoint URL | Streamable HTTP: the Agent connects to a running service | `url: https://example.com/mcp` |

You usually do not need to set `transport`; Skillshare infers it from `command`
or `url`. A URL can point to a service on your own computer or a remote service.
Use the provider's actual MCP endpoint, not an ordinary website URL. Legacy SSE
configuration is rejected rather than silently converted.

## Keep everything in one file

This is the default. Your existing skills and agents remain directory sources;
MCP connections are structured settings under `mcp.servers`:

```yaml
sources:
  skills: ~/.config/skillshare/skills
  agents: ~/.config/skillshare/agents

mcp:
  targets: [claude, codex, cursor, vscode]
  servers:
    company-docs:
      url: https://docs.example.com/mcp
```

`company-docs` is a name you choose. It does not install or look up a server.
Replace the example URL with your provider's endpoint. `mcp.targets` selects
receiving clients independently of your skill targets. A server's optional
`targets` list overrides that default.

## Split MCP into its own file

Use an external source when you want to share or version it separately:

```yaml title="config.yaml"
sources:
  skills: ~/.config/skillshare/skills
  agents: ~/.config/skillshare/agents
  mcp: ./mcp.yaml

mcp:
  targets: [claude, codex, cursor]
```

```yaml title="mcp.yaml"
servers:
  company-docs:
    url: https://docs.example.com/mcp
```

Relative paths are resolved from the directory containing `config.yaml`.
For `.skillshare/config.yaml`, `./mcp.yaml` means `.skillshare/mcp.yaml`.
Absolute paths and `~/` are also supported.

Use **one source at a time**: `sources.mcp` and `mcp.servers` cannot coexist,
including `mcp.servers: {}`. To switch, move the `servers` mapping into the external
file, add `sources.mcp`, and remove inline `mcp.servers`. Keep `mcp.targets` in
`config.yaml`. Preview before syncing:

```bash
skillshare sync mcp --dry-run
```

Both CLI and dashboard edits follow the active source. A missing or invalid
external file stops synchronization; it never means “delete all servers.” Use an
explicit `servers: {}` to remove definitions intentionally, then preview the
managed removals.

## Local programs and credentials

```yaml
mcp:
  targets: [claude, codex]
  servers:
    internal-tools:
      command: company-mcp
      args: [--workspace, /path/to/workspace]
      env:
        COMPANY_TOKEN:
          fromEnv: COMPANY_TOKEN
    company-docs:
      url: https://docs.example.com/mcp
      bearerToken:
        fromEnv: DOCS_TOKEN
```

Install the required local program yourself. The Agent must be able to find it
and read any referenced environment variables in its own environment. A variable
set only in a terminal may not reach an Agent launched from the desktop.

Skillshare writes variable references and never resolves them. Keep actual tokens
out of source files, URLs and command arguments. Known sensitive environment or
header keys require `fromEnv`. Import converts recognizable literal secrets to
references and reports the variable you need to set; it cannot identify every
possible credential format.

Codex forwards local variables by name, so `env.KEY.fromEnv` must also be `KEY`
when Codex is selected. A target that cannot represent a setting blocks the
preview instead of dropping it. Client-specific placeholders, input prompts and
unknown native fields must be resolved explicitly before import.

## Import existing connections

```bash
skillshare mcp import                         # Choose an Agent and a server
skillshare mcp import docs --from claude --target claude --target codex --sync
```

Import one server at a time. The original native definition is treated as an
explicit adoption only for the Agent you imported from. Entries with the same
name in other Agents still require review. **Save only** records the imported
native baseline without changing that Agent's file, so a later sync can detect
new edits correctly.

If the source already contains the name, use the dashboard's **Edit** action or
CLI `--replace`. This replaces the source definition, not arbitrary conflicting
native entries. In the MCP dashboard, preview a conflict and explicitly choose
the affected entry's replacement, or import the Agent's version instead.

## Remove and restore

```bash
skillshare mcp remove company-docs
skillshare sync mcp --dry-run
skillshare sync mcp
```

Only unchanged entries previously managed by this configuration are removed.
Unmanaged entries and entries edited by another program are protected.

Every native file change creates a private backup of the affected MCP entries.
The output includes its ID:

```bash
skillshare mcp restore BACKUP_ID --dry-run
skillshare mcp restore BACKUP_ID
```

Restore preserves unrelated settings and refuses to overwrite newer changes to
the affected entries. It does not revert your source file; edit the source too
if you want the restoration to survive the next sync. Backups may contain old
native credentials, so keep the local state directory private.

Writes are atomic per file. A failure midway through multiple files leaves
completed files applied and reports their backup IDs. Fix the reported cause and
retry; an interrupted native write is reconciled with the ownership journal.
Do not delete ownership state to “fix” conflicts: existing entries would become
unmanaged and need explicit import again.

See the [MCP command reference](/docs/reference/commands/mcp) for supported paths,
flags and current limitations.
