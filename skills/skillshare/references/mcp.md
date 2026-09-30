# MCP Connections

Skillshare keeps MCP server definitions in one source and writes each Agent's native
config file from it. It writes settings only: it never starts a server, checks
connectivity, resolves a secret or copies OAuth credentials. The one exception is
`mcp check --live`, which starts or calls servers only when asked.

## Commands

```bash
skillshare mcp --no-tui                             # Plain status (bare mcp opens the TUI)
skillshare mcp add docs --url https://example.com/mcp --target claude --sync --no-tui
skillshare mcp add local --target claude --no-tui -- npx -y @modelcontextprotocol/server-filesystem /path/to/docs
skillshare mcp edit docs --url https://updated.example/mcp --no-tui
skillshare mcp import --from claude --json           # Inspect candidates without saving
skillshare mcp import docs --from claude --target claude --sync --no-tui
skillshare sync mcp --dry-run --json                 # Preview without executing servers
skillshare sync mcp                                 # Apply native settings
skillshare mcp remove docs --sync --no-tui           # Remove unchanged managed entries
skillshare mcp remove docs --keep-files --no-tui     # Stop managing; Agent entries stay as they are
skillshare mcp restore BACKUP_ID --dry-run --json     # Preview entry-level restoration
skillshare mcp restore BACKUP_ID --no-tui            # Apply restoration; source stays unchanged
skillshare mcp check --json                          # Static check: variables, commands, DNS, sync state
skillshare mcp check --live --timeout 30s --json     # Also start/call each server: serverInfo, protocol, tools
```

## Automation rules

- Pass `--no-tui` or `--json` and every required input. `edit`, `remove` and `restore`
  need a name or backup ID when noninteractive.
- Receiving clients use repeated `--target` (singular), not `--targets`. `--force` is
  unsupported and cannot bypass a conflict.
- `--target none` (YAML `targets: []`) keeps a server in Skillshare and writes it to no
  client; the next sync removes entries it had. Omitting `--target` inherits
  `mcp.targets` instead, and fails when that is empty. Not valid with `--disabled`.
- `add`, `edit`, `import` and `remove` save the source only. Add `--sync` to write the
  Agent files too. `sync --all` includes MCP along with skills, agents and extras.
- Scripted `edit` accepts `--url`, repeated `--target`, `--tools-allow`,
  `--tools-deny`, `--pi-options` or `-- command args`. Switching transport clears the
  fields of the other one.
- `--pi-extension`, `--pi-options-prune` and `--direct-tools` were removed in 0.23.0 and
  fail with a message. See [Upgrading Pi from 0.22](#upgrading-pi-from-022).
- Preview with `skillshare sync mcp --dry-run --json`, then apply with
  `skillshare sync mcp --revision <revision>` to reject a stale plan.
- `mcp check [name...] [--json] [--no-dns]` is read-only; without `--live` it starts nothing. It exits 1
  on an error (unset `fromEnv` variable, command not on PATH, client rule, conflict);
  an unresolved host or an unsynced entry is only a warning. In global mode it also checks
  servers under `mcp.projects`; `--json` marks each with its `project` root.
- `mcp check --live [--timeout 10s]` also starts each local server (your environment plus
  its `env`) and POSTs to each remote one, skipping servers with a static error. A 401 is a
  warning with the resource metadata URL; it never signs in. Use it only for trusted
  servers. `--json` adds `live: {protocolVersion, serverInfo, tools, toolNames}`.
- Noninteractive `import` without a name only lists candidates. Use `--replace` only
  when replacing is intended.
- An `update` with the message `same settings, laid out one field per line` is a
  formatting-only rewrite of a managed JSON entry that sat on one line. Values do not
  change, and entries formatted by hand are left alone.

## Source

Definitions live in `mcp.servers` of the Skillshare config, or in the YAML file named by
`sources.mcp` (top-level `servers`, path relative to the config directory). Never define
both. `mcp.targets` is the default client list; a server's own `targets` overrides it.
MCP targets are independent of skill targets.

```yaml
mcp:
  targets: [claude, codex]
  servers:
    docs:
      url: https://example.com/mcp
      headers:
        X-Api-Key: {fromEnv: DOCS_KEY}     # a reference, never the secret itself
    files:
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/path/to/docs"]
      targets: [claude, opencode]
```

| Field | Meaning |
|---|---|
| `command`, `args`, `env` | Local stdio server. `env` values are strings or `{fromEnv: VARIABLE}` |
| `url`, `headers`, `bearerToken` | Streamable HTTP server. `bearerToken` is `{fromEnv: VARIABLE}` and cannot coexist with an Authorization header |
| `transport` | Optional `stdio` or `streamable-http`; inferred when omitted. Legacy SSE is not supported |
| `targets` | Receiving clients for this server. `[]` keeps it in Skillshare only |
| `tools` | `{allow, deny}`: which tools reach the model, translated per Agent. See [Tool policy](#tool-policy) |
| `piOptions` | Other Pi built-in per-server fields. See [Pi](#pi) |
| `disabled` | `true` only, no connection fields, and a project must be in scope: project mode, or a root under `mcp.projects`. See [Turn off a global server in one project](#turn-off-a-global-server-in-one-project) |

Client IDs: `claude`, `codex`, `cursor`, `vscode`, `opencode`, `kilocode`, `grok`,
`antigravity`, `amp`, `claude-desktop`, `cline`, `copilot`, `factory`, `gemini`, `goose`,
`junie`, `kiro`, `lmstudio`, `warp`, `windsurf`, `pi`. Scope and transport support vary by
client; the website's MCP command reference lists every native destination and limit.
Names such as `company-docs` (letters, digits, hyphens) work across all clients.

## Ownership and conflicts

- Skillshare only changes entries it wrote. A native entry that already matches the
  source but is not managed yet is planned as `adopt`: sync records it without
  writing, and later removal or unticking removes it.
- A differing unmanaged entry is a conflict. Resolve it with `mcp import --replace` or
  an explicit per-entry replacement. Inspect conflicts with `skillshare mcp --json`.
- Only the fields Skillshare writes are compared. Agent-only fields in an entry, such
  as timeouts, are kept across syncs. Tool filters come from `tools`.
- Turning a managed server off by hand in the Agent's file (`enabled: false`,
  `disabled: true`) is reported as a conflict.
- Every write is backed up per Agent file. `mcp restore` restores entries, not the
  source.

## Agent notes

Read the matching note before writing for one of these Agents. They are where MCP
setups usually go wrong.

### Claude Code

| Scope | File |
|---|---|
| Global | `~/.claude.json` (honors `CLAUDE_CONFIG_DIR`) |
| Project | `.mcp.json` |

- Another account: a target with `agent:` and `config_dir: <dir>` is an MCP target by its own
  name, and `mcp import --from <name>` reads that file. `claude` (`CLAUDE_CONFIG_DIR`) writes
  `<dir>/.claude.json`, `codex` (`CODEX_HOME`) writes `<dir>/config.toml`, `pi`
  (`PI_CODING_AGENT_DIR`) writes `<dir>/mcp.json`. Global scope only; a project
  uses the Agent's own name, and a project's off switch goes to every account that has the
  server.
- Reserved names: a server called `workspace`, `claude-in-chrome` or `computer-use` is
  skipped by Claude Code. Skillshare refuses them for `claude`.
- Claude Code never sends its own credentials to a remote server.
  `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN`, `AWS_BEARER_TOKEN_BEDROCK`, `HTTPS_PROXY`
  and `NPM_TOKEN` read as empty in `url` and `headers`, so Skillshare refuses them there.
  Copy the credential into a variable with another name and reference that.
- Local scope hides entries. A server added with `claude mcp add` and no `--scope` is
  stored per project in `~/.claude.json` and wins, whole, over the same name in
  `.mcp.json` or the user scope. Project mode reports it beside the entry it hides
  without blocking. Remove it with `claude mcp remove NAME -s local` in that folder.
- Claude Code rewrites `~/.claude.json` constantly. A preview stays valid unless the
  file's MCP entries themselves changed.
- In project mode, `claude` and `copilot` cannot be selected together, and an existing
  `.mcp.json` blocks Copilot CLI sync, because Copilot reads `.mcp.json` first. Use
  global mode for one of them.
- Claude Desktop (`claude-desktop`) is a separate client: global only, stdio only.

### Codex

| Scope | File |
|---|---|
| Global | `~/.codex/config.toml` (honors `CODEX_HOME`) |
| Project | `.codex/config.toml` |

- One `config.toml` is shared by the Codex CLI, the Codex IDE extension and the ChatGPT
  desktop app. A server synced to `codex` appears in all three.
- Codex loads `.codex/config.toml` only in a project it trusts. In an untrusted project
  the synced servers do not load, and Codex shows no error. Check trust first when a
  user reports "synced but missing".
- Only `[mcp_servers.NAME]` tables and their subtables can be edited. Inline or dotted
  definitions are rejected without touching the file; convert them to tables first.
- `cwd`, `http_headers_helper`, approval modes, timeouts (`startup_timeout_sec`) and
  the `oauth` table have no portable form. Import leaves them out with a warning, and
  sync keeps them in the existing entry. `enabled_tools`/`disabled_tools` come from
  `tools` (exact names only) and import back into it.
- Servers bundled by a Codex plugin live under `plugins.<plugin>.mcp_servers` and are
  not managed here.
- `--disabled` is not supported: a lone `enabled = false` breaks Codex's whole config
  where the global server is missing.

### OpenCode

| Scope | File |
|---|---|
| Global | `~/.config/opencode/opencode.json` (honors `XDG_CONFIG_HOME`) |
| Project | `opencode.json` in the project root |

- Entries are written in OpenCode's own shape: `local` / `remote` types under `mcp`,
  and `{env:VARIABLE}` for `fromEnv`. Write the portable form in the source; Skillshare
  converts it.
- An existing `opencode.jsonc` is used instead of creating `opencode.json`. A project
  file kept in `.opencode/` is reused too; a new one goes in the project root. If more
  than one exists, sync stops; the user consolidates them first. Comments and unrelated
  settings are preserved.
- `OPENCODE_CONFIG`, `OPENCODE_CONFIG_DIR`, inline config and ancestor-directory files
  are not managed and may override what Skillshare wrote.
- `--disabled` writes `{"enabled": false}` for the named server in the project file.
- Kilo Code (`kilocode`) uses the same format in `kilo.jsonc`. It treats project config
  as untrusted and drops the whole file if it contains an `{env:VARIABLE}` reference,
  so a server using `fromEnv` or `bearerToken` is refused for Kilo in project mode.
  Define it in global mode.

### Pi

Pi >= 0.99.0 has built-in MCP, and it is the only Pi format Skillshare writes. The
third-party `pi-mcp-adapter` and `pi-mcp-extension` are not sync destinations.
Upgrading from 0.22 moves their servers to `mcp.json`: older Pi then stops loading them,
and an extension still installed can replace Pi's built-in MCP, so update Pi and remove
the extension from Pi. Sync warns once when it moves them.

| Scope | File |
|---|---|
| Global | `~/.pi/agent/mcp.json` (honors `PI_CODING_AGENT_DIR`) |
| Project | `.pi/mcp.json` |

Personal servers and servers with credentials belong in the global file. Project
files are for project-required servers in trusted projects; a project entry replaces
the whole global entry. Skillshare edits files with preview/backup; it does not grant
trust, launch servers, install extensions, or authorize OAuth.

Use `pi mcp add` for simple Pi-only setup (`-l` for project). After sync use `/reload`
or a new session, and `/mcp` to inspect connections. `pi mcp list` launches every
enabled server to check connections. `pi mcp login NAME` needs user approval.

`piOptions` holds the other per-server fields of Pi's built-in MCP: `exposure`
(`codemode`, `codemode-deferred`, `deferred`, `direct`, `hidden`), `toolExposure`
(tool names or wildcards; an exact name wins, then the first matching pattern, and
order is preserved), positive seconds `timeout`, `cwd`, `enabled` and `oauth`. Known
values are checked; unknown fields pass through.

```bash
skillshare mcp add docs --url https://example.com/mcp --target pi --pi-options '{"exposure":"deferred","timeout":120}' --no-tui
skillshare mcp edit docs --pi-options '{}' --no-tui
```

- `exposure` sits next to `tools` and also sets how allowed tools are offered. Prefer
  `tools` over `toolExposure`; a server cannot set both.
- JSON replaces the source options. A cleared field is removed from Pi's file when
  Skillshare wrote it and it is unchanged; a field the user added stays; a written
  field changed in Pi blocks sync until imported.
- Refused in `piOptions`: main connection fields, `type`, top-level `settings`,
  `autoEnableCodemode`, `directTools`, `includeTools`/`excludeTools` (use `tools`), and
  the other `pi-mcp-adapter` fields (`lifecycle`, `idleTimeout`, `toolPrefix`,
  `bearerTokenEnv` and so on), which built-in MCP does not read.
- Keep credentials in environment references. Portable literal `!command` env/header
  values are refused; command values in `piOptions` are rejected even under
  non-secret keys such as `oauth.clientId`.
- Pi server names allow only letters, digits, `_` and `-`.
- Pi cannot take a switch-only `disabled` entry. Use `piOptions: {enabled: false}` on
  a complete server instead.

## Tool policy

`tools` is written once and translated per Agent on sync:

```yaml
mcp:
  servers:
    github:
      command: github-mcp
      targets: [pi, codex, copilot]
      tools:
        allow: [get_*, search_code] # only these stay; * matches any characters
        deny: [get_secret]          # removed even when allowed
```

```bash
skillshare mcp edit github --tools-allow 'get_*,search_code' --tools-deny get_secret --no-tui
skillshare mcp edit github --tools-allow '' --no-tui      # empty value clears that part
```

The flags work on `add`, `edit` and `import`; lists are comma-separated. Only `*` is a
wildcard; `? [ ] { }`, spaces, commas, duplicates, a deny list that removes every
allowed tool, and `tools` on a `disabled` entry are errors.

| Agent | Applies |
|---|---|
| `pi` | Everything: `toolExposure` (denied `hidden`, then allowed at `piOptions.exposure` or `codemode`, then `"*": "hidden"` when allow is set) |
| `codex` | `enabled_tools`/`disabled_tools`, exact names only; not allow patterns |
| `copilot` | `tools`: exact allowed names minus denied, else `["*"]`; no allow patterns, or deny without exact allow names |
| `opencode`, `kilocode`, all others | Nothing; their tool filters live in a top-level permission map |

Unapplied parts are never dropped silently: the plan prints
`tool policy not applied for <target>: <parts> (<servers>)` (JSON `notices`), and
`mcp check` reports a `tools` warning per Agent. Import maps Codex and Copilot tool
lists back to `tools`, and Pi `toolExposure` only when it round-trips exactly
(otherwise it stays in `piOptions` with a warning); `exposure` stays in `piOptions`.

## Upgrading Pi from 0.22

Older configs still load; `sync mcp --dry-run` warns per kind and names the servers.
The first real sync (`sync mcp`, `sync --all`, or the dashboard's sync) converts them,
writes the Agent files, then saves `config.yaml` (or the `sources.mcp` file) without the
retired keys, keeping the old file in `backup files` history with reason `migrate` and
printing `Updated config.yaml for 0.23.0 (backup: <path>)`. `--dry-run` writes nothing.

- `piExtension` is removed. Adapter servers move to `mcp.json`, and the entries
  Skillshare wrote in `mcp-adapter.json` are removed; hand-written ones stay.
  Extension entries are rewritten in place.
- `piOptionsPrune` is removed; pruning unchanged owned fields is always on.
- `directTools`: `true` → `piOptions.exposure: direct`, `"search"` → `deferred`, a list
  → `toolExposure` entries `direct`. `mcp.directTools` and a project's `directTools`
  apply to servers that reach Pi without their own value.
- `piOptions.includeTools`/`excludeTools` → `tools.allow`/`tools.deny` (a
  `directTools` next to them still becomes `piOptions.exposure`).
- Other adapter-only `piOptions` are dropped; `pi` is removed from `disabled` entries.
- `--pi-extension` and `--pi-options-prune`: drop them. `--direct-tools`: use
  `--pi-options '{"exposure":"direct"}'`, or `--pi-options '{"toolExposure":{"TOOL":"direct"}}'`.
- `mcp import --from pi` still reads `mcp-adapter.json` (read-only; `mcp.json` wins on
  the same name) and converts its tool settings.

## Turn off a global server in one project

Needs a project in scope: `-p`, a folder with `.skillshare/config.yaml`, or a root under
`mcp.projects` in the global config. It writes just the switch, so the Agent keeps its
global command or URL. NAME must be the name in the Agent's own global config;
Skillshare does not check that it exists there.

| Target | Supported | Written |
|---|---|---|
| `claude` | Yes | name added to this project's `disabledMcpServers` in `~/.claude.json` (per machine) |
| `opencode`, `kilocode` | Yes | `{"enabled": false}` |
| `codex` | No | see [Codex](#codex) |
| `pi`, all other targets | No | Error, nothing written |

```bash
skillshare mcp add NAME --disabled --target opencode -p --no-tui
skillshare sync mcp -p
```

- `--disabled` cannot be combined with `--url` or `-- command`. Without `--target` the
  entry follows the project's targets: each sync sends it to the clients that have a
  switch (under `mcp.projects`, also only those the global server of that name goes to).
  With `--target`, an unsupported client is an error.
- Claude's off list is per machine and keyed by the project's absolute path. Each
  teammate syncs once in their own checkout, and a moved project needs a new sync. A
  name the user turned off in `/mcp` themselves is never claimed or removed.
- To turn the server back on, `skillshare mcp remove NAME -p --sync`.
- For a server Skillshare itself defines, unselect the Agent on that server instead.

## Several projects from the global config

`mcp.projects` in the global config writes project files for many folders in one sync,
without a `.skillshare/config.yaml` in each. Keys are absolute paths or start with `~`.
Each project takes `targets` and `servers`; missing `targets` inherit the global ones.

```yaml
# ~/.config/skillshare/config.yaml
mcp:
  targets: [claude, opencode]
  servers:
    context7:
      command: npx
      args: ["-y", "@upstash/context7-mcp"]
  projects:
    ~/work/project01:
      servers:
        context7:                  # off in this project only
          disabled: true
          targets: [opencode]
    ~/work/project02:
      servers:
        internal-docs:             # exists in this project only
          url: https://example.com/mcp
          targets: [opencode]
```

- A project lists only what differs. Global servers already load in every project,
  because the Agent reads its global and project files together; do not repeat them. A
  `disabled` entry only switches one off in that folder.
- No command edits `mcp.projects`; `mcp add` leaves it as written. Edit `config.yaml`, or
  use the dashboard: the MCP page's **Projects** tab adds and removes projects, turns
  global servers off per project and edits project servers.
- `disabled` under `mcp.projects` cannot target `claude`. Use project mode in that
  folder instead.
- It is refused inside a project config. A folder whose own `.skillshare/config.yaml`
  manages the same entry produces a conflict, not an overwrite.
- Removing a project from the list removes the entries Skillshare wrote there.
- To share one definition between projects, use a YAML anchor kept inside
  `mcp.projects`.
- The preview names the file when one server appears in several places.

## Interactive use

For a person at a terminal: `skillshare mcp` opens the manager (`/` search, Enter
details, `a` add, `i` import, `e` edit, `x` remove, `s` sync, `b` backups). `mcp add`
guides URL or JSON setup, `mcp edit` opens a server picker and editor, `mcp import
--from claude` offers batch selection with one preview, and `mcp restore` browses
backups. Review screens scroll before confirmation, and Esc cancels without saving. The
TUI offers "Save only" or "Save and sync" instead of `--sync`. An agent running
commands for the user should stay on `--no-tui` or `--json`.
