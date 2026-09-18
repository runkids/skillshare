---
sidebar_position: 3
---

# mcp

Manage portable MCP connection definitions and synchronize native Agent settings.
Start with [Set up MCP once](/docs/how-to/daily-tasks/sharing-mcp).

## Commands

```bash
skillshare mcp
skillshare mcp add
skillshare mcp edit
skillshare mcp edit docs --url https://updated.example/mcp --no-tui
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
| `--file PATH` | Native JSON/JSONC, TOML or Goose YAML; `.toml` defaults to Codex, other formats are detected from their MCP section; use `--from` for an explicit dialect |
| `--sync` | Save and synchronize; noninteractive add/import/remove otherwise save only |
| `--replace` | Explicitly replace an existing source definition during add/import; on import, also rewrite the imported client's entry when it differs |
| `--dry-run`, `-n` | Preview without saving or writing native configuration |
| `--json` | Structured output; sync/preview reports contain names, paths and actions, not server values |
| `--no-tui` | Disable interactive menus; also disabled by `tui: false`, `--json`, or non-terminal input/output |
| `--revision ID` | Require a matching preview for add/import/remove or `sync mcp` |
| `--global`, `-g` | Use global Skillshare configuration |
| `--project`, `-p` | Use project Skillshare configuration |

With no subcommand, `mcp` opens the searchable manager in an interactive terminal,
or prints status in noninteractive mode. Noninteractive import without a name lists
parsed candidates for selection and does not save. Candidates contain portable
definitions, with recognizable secrets converted to references. Agent-specific
fields are listed as warnings and left out; disabled servers and unsupported
transports block the candidate. `restore` always previews again before applying;
use `--dry-run` to inspect it without applying.

`sync mcp` accepts scope flags, `--dry-run`, `--json`, `--no-tui`, and `--revision`.
`sync --all` includes skills, agents, extras and MCP; plain `sync` keeps its
existing resource behavior. MCP conflicts are checked before `--all` changes
other resources. Resource types and native files are separate operations, not a
single transaction.

## Interactive management

Run `skillshare mcp` or `skillshare mcp list`. Like the skills list, the manager
supports `/` to search and `Enter` for details. Connection lists hide argument,
header and environment values, and omit URL queries.

| Key | Action |
|---|---|
| `a` | Add a connection |
| `i` | Import one or more connections |
| `e` | Edit the selected connection |
| `x` | Remove the selected connection |
| `s` | Preview and confirm synchronization |
| `b` | Browse backups by client, then newest first |
| `r` | Refresh status |
| `q` | Quit |

`mcp edit`, `mcp remove`, and `mcp restore` offer selection menus when their name
or backup ID is omitted. The editor covers command/URL, arguments, environment
variables, HTTP headers, bearer-token environment references and receiving
targets. Arguments accept one literal argument per line or a JSON array. Switching
transport clears fields that do not apply to the new connection type.

Add, edit, remove and import show a preview before **Save and sync** or **Save
only**. Escape cancels the pending draft. Restore previews and confirms changes
to Agent entries; it does not rewrite the source definition.

Import without a server name supports multiple selections (`Space` toggles,
`a` selects all). Invalid candidates are skipped; existing source names are skipped
unless `--replace` is specified. Select one set of compatible receiving clients
for the batch. The entire batch is validated before the source is saved once;
later native-file I/O failures retain the existing recovery behavior.

For scripts, provide a name and flags. `mcp edit NAME --url URL`,
`mcp edit NAME --target CLIENT`, and `mcp edit NAME -- command args...` update the
specified fields while preserving other applicable settings. They save only
unless `--sync` is added. With `--no-tui`, remove requires a name and restore
requires a backup ID. `--dry-run` never saves or synchronizes changes.

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

Client IDs are `claude`, `codex`, `cursor`, `vscode`, `opencode`, `grok`,
`antigravity`, `amp`, `claude-desktop`, `cline`, `copilot`, `factory`, `gemini`,
`goose`, `junie`, `kiro`, `lmstudio`, `warp`, `windsurf`, and `pi`.
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
| Antigravity (AGY) | `~/.gemini/config/mcp_config.json` | `.agents/mcp_config.json` | `mcpServers` |
| [Amp](https://ampcode.com/docs/customize/mcp) | `~/.config/amp/settings.json` | `.amp/settings.json` | `amp.mcpServers` (literal key) |
| [Claude Desktop](https://modelcontextprotocol.io/docs/develop/connect-local-servers) | Claude application data directory, `claude_desktop_config.json` | Global only | `mcpServers` |
| [Cline (VS Code)](https://github.com/cline/cline/tree/main/apps/vscode/src/services/mcp) | VS Code User directory, `globalStorage/saoudrizwan.claude-dev/settings/cline_mcp_settings.json` | Global only | `mcpServers` |
| [Copilot CLI](https://docs.github.com/en/copilot/how-tos/copilot-cli/customize-copilot/add-mcp-servers) | `~/.copilot/mcp-config.json` | `.github/mcp.json` | `mcpServers` |
| [Factory Droid](https://docs.factory.ai/harness/mcp) | `~/.factory/mcp.json` | `.factory/mcp.json` | `mcpServers` |
| [Gemini CLI](https://geminicli.com/docs/tools/mcp-server/) | `~/.gemini/settings.json` | `.gemini/settings.json` | `mcpServers` |
| [Goose](https://block.github.io/goose/docs/guides/config-files/) | `~/.config/goose/config.yaml` | Global only | `extensions` (YAML) |
| [Junie](https://junie.jetbrains.com/docs/junie-cli-mcp-configuration.html) | `~/.junie/mcp/mcp.json` | `.junie/mcp/mcp.json` | `mcpServers` |
| [Kiro](https://kiro.dev/docs/mcp/configuration/) | `~/.kiro/settings/mcp.json` | `.kiro/settings/mcp.json` | `mcpServers` |
| [LM Studio](https://lmstudio.ai/docs/app/mcp) | `~/.lmstudio/mcp.json` | Global only | `mcpServers` |
| [Warp](https://docs.warp.dev/agents/capabilities/mcp/) | `~/.warp/.mcp.json` | `.warp/.mcp.json` | `mcpServers` |
| [Windsurf (Cascade)](https://docs.devin.ai/desktop/cascade/mcp) | `~/.codeium/windsurf/mcp_config.json` | Global only | `mcpServers` |

The dashboard's server form edits HTTP headers the same way as environment variables,
including `fromEnv` references. **View what each Agent gets**, in a server's menu and
beside the file count in its form, shows read only the native text Sync would write for
the selected client; in the form it reflects edits that are not saved yet. Secrets stay
as references.

JSON entries are written one field per line at the file's own indentation. An entry
Skillshare owns that still sits on one line is reported as an `update` and written again
laid out. Entries it does not own, and entries someone formatted by hand, keep their layout.

The dashboard only offers destinations available in the current scope and host
platform. Each server is one row; the count button on the right opens the full
client list for that server. Global-only clients cannot be selected in project mode.
The **Sync** box on the right lists the changes not yet written: ticking a client
only edits the source, and the files are written after you confirm on the Sync page.
Below it, **Agents** lists the clients whose config file was detected.

Additional client details:

- Claude Desktop file sync supports **stdio only**, on macOS and Windows.
  Its directory is `~/Library/Application Support/Claude` on macOS and
  `%APPDATA%/Claude` on Windows. Configure remote connectors in the application.
- Cline targets the default VS Code Stable profile, not Cline CLI or other IDEs.
- Copilot CLI exports `tools: ["*"]` for new entries and preserves existing tool
  filters. If a project `.mcp.json` exists, sync stops because Copilot reads that
  file ahead of `.github/mcp.json`; consolidate the files first.
  Selecting Claude Code and Copilot CLI together in project mode is also blocked
  before writing either file. Use global mode for one of these clients.
- Gemini uses `httpUrl` for Streamable HTTP. Its `url` field means legacy SSE
  and is rejected on import. Cline uses `type: streamableHttp`; Goose uses
  `type: streamable_http` and `uri`. Skillshare converts these automatically.
- Goose on Windows uses `%APPDATA%/Block/goose/config/config.yaml`. YAML edits
  preserve unrelated settings, comments and built-in extensions, but may change
  formatting. Aliases, merges, duplicate keys and multiple documents block edits.
  Built-in extensions and keychain `env_keys` cannot be imported as portable MCP
  connections.
- Windsurf support is for the documented Cascade configuration. Warp project
  connections still require approval inside Warp each session.

Environment references are exported as `${VARIABLE}` for Amp, Copilot CLI,
Factory, Gemini CLI and Kiro, and `${env:VARIABLE}` for Cline and Windsurf.
Claude Desktop, Goose, Junie, LM Studio and Warp currently reject `fromEnv` and
`bearerToken` exports because their native interpolation has not been verified.
Use connections without custom credentials or authenticate in the receiving
client where supported. Skillshare never resolves references into plaintext.

Antigravity uses the current [official MCP configuration](https://antigravity.google/docs/mcp),
including `serverUrl` for remote connections. Skillshare converts portable `url`
automatically. Older `.gemini/antigravity/` and `.gemini/antigravity-cli/` config
locations are not managed. Antigravity `fromEnv` and `bearerToken` exports are
blocked because its documented configuration does not specify environment
interpolation. Use connections that need no custom secret headers, and complete
supported OAuth login inside Antigravity. Skillshare never expands references
into plaintext credentials.

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
sync keeps them in the Agent's existing entry. Pi is supported through an explicitly
selected third-party extension; see below.

VS Code Stable's default user file is:

- macOS: `~/Library/Application Support/Code/User/mcp.json`
- Linux: `${XDG_CONFIG_HOME:-~/.config}/Code/User/mcp.json`
- Windows: `%APPDATA%/Code/User/mcp.json`

Global Claude, Codex, Grok and Copilot paths respect `CLAUDE_CONFIG_DIR`, `CODEX_HOME`,
`GROK_HOME` and `COPILOT_HOME`. Amp and Goose honor `XDG_CONFIG_HOME` on the
platforms using their `.config` paths.
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


## Pi: choose your MCP extension

Pi can use MCP through either [pi-mcp-adapter](https://pi.dev/packages/pi-mcp-adapter)
or [pi-mcp-extension](https://pi.dev/packages/pi-mcp-extension). These are third-party
packages listed on Pi's website, not built-in Pi features. Install **one** in Pi:

```bash
pi install npm:pi-mcp-adapter
```

Restart Pi after installation. In Skillshare's MCP form, select **Pi**, then choose
the package you installed. The import dialog offers the same choice. In the terminal,
`mcp add` / `mcp edit` guide the selection; scripts must supply `--pi-extension`:

```bash
skillshare mcp add docs --url https://example.com/mcp --target pi --pi-extension pi-mcp-adapter --no-tui
skillshare sync mcp --dry-run
skillshare sync mcp
```

The saved server definition is:

```yaml
mcp:
  servers:
    docs:
      url: https://example.com/mcp
      targets: [pi]
      piExtension: pi-mcp-adapter
```

For the other package, use `pi-mcp-extension` in both the install command and the
selection. Every server targeting Pi within a Skillshare source must choose the
same package, because both packages read the same destination file.

| Package | Native output | What to do after sync |
|---|---|---|
| `pi-mcp-adapter` | `command`/`args` or `url`; `${VARIABLE}` references | Restart/reload Pi; use `/mcp` to inspect connections. Tools connect on demand. |
| `pi-mcp-extension` | Explicit `transport: stdio` or `streamable-http` | Restart Pi; new servers default to manual start with `/mcp:start <server>`. Existing `lifecycle` settings are preserved. |

Both use `~/.pi/agent/mcp.json` globally and `.pi/mcp.json` in project mode.
Skillshare uses these Pi-specific files, not the adapter's shared `.mcp.json` or
`~/.config/mcp/mcp.json` inputs. Project entries override global entries with the
same name. For the adapter, a global `PI_CODING_AGENT_DIR` override is honored.
The extension does not honor that override; global sync refuses it rather than
writing a file the extension would ignore.

The adapter supports `fromEnv` in environment variables and HTTP headers.
The extension does **not** interpolate environment references: matching stdio
variables (for example `TOKEN: {fromEnv: TOKEN}`) are inherited from Pi's process
instead; renaming variables and environment-backed HTTP credentials are rejected.
Use the adapter for those cases. Skillshare never reads or copies secret values.

Import with `--from pi` reads the Pi-specific file. Choose `--pi-extension` when
saving an imported connection; a file alone cannot identify which package is
installed. Unsupported legacy SSE remains blocked. OAuth and package-only options
stay managed in Pi. Sync success means the configuration was written, not that
an extension is installed or a server has connected.
