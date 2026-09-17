---
sidebar_position: 3
---

# mcp

Manage portable MCP connection definitions and synchronize native Agent settings.
Start with [Set up MCP once](/docs/how-to/daily-tasks/sharing-mcp).

## Commands

```bash
skillshare mcp add
skillshare mcp add docs --url https://example.com/mcp --target claude --sync
skillshare mcp add local --target codex -- company-mcp --workspace /path/to/workspace
skillshare mcp import docs --from claude --target claude --target cursor --sync
skillshare mcp import docs --file ./provider.json --target claude
skillshare mcp list --json
skillshare mcp remove docs --sync
skillshare mcp restore BACKUP_ID --dry-run
skillshare sync mcp --dry-run --json
skillshare sync mcp
skillshare sync --all
```

| Option | Meaning |
|---|---|
| `--target CLIENT` | Receiving client; repeat to select multiple clients |
| `--url URL` | Streamable HTTP endpoint for `add` |
| `-- command args...` | Local executable and literal arguments for `add` |
| `--from CLIENT` | Existing client to import, or the format of `--file` |
| `--file PATH` | Native JSON/JSONC or TOML to import; `.toml` defaults to Codex, JSON is detected from its `mcpServers`, `servers` or `mcp` key |
| `--sync` | Save and synchronize; noninteractive add/import/remove otherwise save only |
| `--replace` | Explicitly replace an existing source definition during add/import; on import, also rewrite the imported client's entry when it differs |
| `--dry-run`, `-n` | Preview without saving or writing native configuration |
| `--json` | Structured output; sync/preview reports contain names, paths and actions, not server values |
| `--revision ID` | Require a matching preview for add/import/remove or `sync mcp` |
| `--global`, `-g` | Use global Skillshare configuration |
| `--project`, `-p` | Use project Skillshare configuration |

With no subcommand, `mcp` lists status. Noninteractive import without a name lists
parsed candidates for selection and does not save. Candidates contain portable
definitions, with recognizable secrets converted to references. Agent-specific
fields are listed as warnings and left out; disabled servers and unsupported
transports block the candidate. `restore` always previews again before applying;
use `--dry-run` to inspect it without applying.

`sync mcp` accepts scope flags, `--dry-run`, `--json`, and `--revision`.
`sync --all` includes skills, agents, extras and MCP; plain `sync` keeps its
existing resource behavior. MCP conflicts are checked before `--all` changes
other resources. Resource types and native files are separate operations, not a
single transaction.

## Source fields

Choose inline `mcp.servers` or an external file named by `sources.mcp`.
External files have a top-level `servers` mapping. `mcp.targets` stays in the
Skillshare config. The schema is `schemas/mcp.schema.json` in the repository.

| Server field | Meaning |
|---|---|
| `command` | Local executable; mutually exclusive with `url` |
| `args` | List of literal arguments for a local executable |
| `env` | Local environment values: strings or `{fromEnv: VARIABLE}` |
| `url` | HTTP(S) MCP endpoint; no embedded credentials or fragment |
| `headers` | HTTP headers: strings or `{fromEnv: VARIABLE}` |
| `bearerToken` | `{fromEnv: VARIABLE}`; cannot coexist with an Authorization header |
| `transport` | Optional `stdio` or `streamable-http`; inferred when omitted |
| `targets` | Optional receiving clients; overrides `mcp.targets` |

Client IDs are `claude`, `codex`, `cursor`, `vscode`, `opencode`, and `grok`.
`grok` means the official xAI Grok CLI. Server names use letters,
digits, dots, underscores and hyphens. A server must select at least one client
either directly or through `mcp.targets` before synchronization.

For Grok, names must start with a letter or underscore, contain only letters,
digits, hyphens and single underscores, and cannot end with an underscore.
Names such as `company-docs` work across all supported clients.

## Native destinations

| Client | Global | Project | Section |
|---|---|---|---|
| Claude Code | `~/.claude.json` | `.mcp.json` | `mcpServers` |
| Codex | `~/.codex/config.toml` | `.codex/config.toml` | `mcp_servers` |
| Cursor | `~/.cursor/mcp.json` | `.cursor/mcp.json` | `mcpServers` |
| VS Code | User `mcp.json` (below) | `.vscode/mcp.json` | `servers` |
| OpenCode | `~/.config/opencode/opencode.json` | `opencode.json` | `mcp` |
| Grok CLI | `~/.grok/config.toml` | `.grok/config.toml` | `mcp_servers` |

OpenCode respects `XDG_CONFIG_HOME` for its global directory. An existing
`opencode.jsonc` is used instead of creating `opencode.json`; if both exist in
the selected directory, consolidate them before syncing. Custom OpenCode config
paths, directory overrides, inline config and inherited ancestor files are not
managed. They may override the selected destination in OpenCode.

OpenCode uses `local`/`remote` types and `{env:VARIABLE}` references; Grok uses
`${VARIABLE}` references. Skillshare converts these automatically. Claude's
`"type": "streamable-http"` imports as HTTP. Disabled connections block import.
Other native options without a portable equivalent, such as Codex
`startup_timeout_sec` or `envFile`, are left out of the import with a warning;
sync keeps them in the Agent's existing entry. Pi has no built-in MCP support
and is not a supported MCP target.

VS Code Stable's default user file is:

- macOS: `~/Library/Application Support/Code/User/mcp.json`
- Linux: `${XDG_CONFIG_HOME:-~/.config}/Code/User/mcp.json`
- Windows: `%APPDATA%/Code/User/mcp.json`

Global Claude, Codex and Grok paths respect `CLAUDE_CONFIG_DIR`, `CODEX_HOME`
and `GROK_HOME`.
Project destinations are relative to the selected project root. Project trust,
server approval and authentication remain the receiving Agent's responsibility.

## Safety and limitations

- JSONC comments and unrelated settings are preserved. Changed owned entries
  are replaced as a unit, so comments inside those entries may change. Only the
  fields Skillshare writes are compared and replaced; Agent-specific fields such
  as timeouts are kept.
  Defaults an Agent fills in, such as `"type": "stdio"`, an empty `env` or
  header name case, are not changes. Turning a managed server off with
  `enabled: false` or `disabled: true` is reported as a conflict.
- A preview stays valid while an Agent rewrites unrelated settings in the same
  file, as Claude Code does with `~/.claude.json`. Only a change to that file's
  MCP entries requires a new preview.
- Codex and Grok edits support ordinary `[mcp_servers.NAME]` tables and their subtables.
  Updated entries stay in place, and CRLF line endings are kept.
  Inline/dotted MCP definitions must be converted to tables before writing;
  they are rejected without modifying the file.
- Native file symlinks, malformed files and duplicate JSON properties block
  writes. A symlinked Skillshare `config.yaml` is written through to its target. File permissions are preserved; new native
  files, ownership records and backups use private permissions.
- An entry that already matches the source is reported as unchanged without a
  write, for example after pulling a teammate's change. If this configuration
  did not manage it before, such as after moving a project, it stays unmanaged:
  removing the server leaves it in place until you import it. A different
  unmanaged entry requires import or an explicit per-entry replacement; another
  Skillshare configuration's ownership cannot be overridden.
- The dashboard's MCP settings work only when the browser opens the dashboard by
  `localhost` or an IP address. Through a domain name, including a reverse
  proxy, MCP requests return 403, because DNS rebinding attacks always use a
  domain name.
- Credentials use environment references; no secret store, OAuth session sync,
  runtime health check, package installation, gateway, registry or plugin sync.
- VS Code Insiders, custom profiles, remote workspaces and legacy SSE are not
  supported in this version.
- VS Code does not currently substitute `${env:VARIABLE}` inside `headers`
  ([microsoft/vscode#336232](https://github.com/microsoft/vscode/issues/336232)),
  so header and `bearerToken` references synced to VS Code reach the server
  unresolved until that is fixed.
- Local operation records live under the Skillshare state directory's `mcp/`:
  `state.json`, `pending.json` during a write, and `backups/` (the newest 20 per
  Agent file). Do not share this
  directory as a portable manifest.
