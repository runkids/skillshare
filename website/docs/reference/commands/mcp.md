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
| `--file PATH` | Native JSON/JSONC or TOML to import; `.toml` defaults to Codex, otherwise Claude |
| `--sync` | Save and synchronize; noninteractive add/import/remove otherwise save only |
| `--replace` | Explicitly replace an existing source definition during add/import |
| `--dry-run`, `-n` | Preview without saving or writing native configuration |
| `--json` | Structured output; sync/preview reports contain names, paths and actions, not server values |
| `--revision ID` | Require a matching preview for add/import/remove or `sync mcp` |
| `--global`, `-g` | Use global Skillshare configuration |
| `--project`, `-p` | Use project Skillshare configuration |

With no subcommand, `mcp` lists status. Noninteractive import without a name lists
parsed candidates for selection and does not save. Candidates contain portable
definitions, with recognizable secrets converted to references. Unsupported
fields block the candidate. `restore` always previews again before applying;
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
`${VARIABLE}` references. Skillshare converts these automatically. Native
options without a portable equivalent, including disabled connections, block
import rather than silently changing behavior. Pi has no built-in MCP support
and is not a supported MCP target.

VS Code Stable's default user file is:

- macOS: `~/Library/Application Support/Code/User/mcp.json`
- Linux: `${XDG_CONFIG_HOME:-~/.config}/Code/User/mcp.json`
- Windows: `%APPDATA%/Code/User/mcp.json`

Global Claude and Codex paths respect `CLAUDE_CONFIG_DIR` and `CODEX_HOME`.
Project destinations are relative to the selected project root. Project trust,
server approval and authentication remain the receiving Agent's responsibility.

## Safety and limitations

- JSONC comments and unrelated settings are preserved. Changed owned entries
  are replaced as a unit, so comments inside those entries may change.
- Codex and Grok edits support ordinary `[mcp_servers.NAME]` tables and their subtables.
  Inline/dotted MCP definitions must be converted to tables before writing;
  they are rejected without modifying the file.
- Native file symlinks, malformed files, duplicate JSON properties and
  unsupported fields block writes. File permissions are preserved; new native
  files, ownership records and backups use private permissions.
- Matching unmanaged entries are not automatically acquired. Import or an
  explicit per-entry replacement is required; another Skillshare configuration's
  ownership cannot be overridden.
- Credentials use environment references; no secret store, OAuth session sync,
  runtime health check, package installation, gateway, registry or plugin sync.
- VS Code Insiders, custom profiles, remote workspaces, legacy SSE and native
  fields without a portable mapping are not supported in this version.
- Local operation records live under the Skillshare state directory's `mcp/`:
  `state.json`, `pending.json` during a write, and `backups/`. Do not share this
  directory as a portable manifest.
