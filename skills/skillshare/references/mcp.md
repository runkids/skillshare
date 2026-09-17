# MCP Connections

```bash
skillshare mcp --no-tui                             # Plain status (bare mcp opens the TUI)
skillshare mcp edit docs --url https://updated.example/mcp --no-tui
skillshare mcp add docs --url https://example.com/mcp --target claude --sync --no-tui
skillshare mcp add local --target claude --no-tui -- npx -y @modelcontextprotocol/server-filesystem /path/to/docs
skillshare mcp import --from claude --json           # Inspect candidates without saving
skillshare mcp import docs --from claude --target claude --sync --no-tui
skillshare sync mcp --dry-run --json                 # Preview without executing servers
skillshare sync mcp                                 # Apply native settings
skillshare mcp remove docs --sync --no-tui           # Remove unchanged managed entries
skillshare mcp restore BACKUP_ID --dry-run --json     # Preview entry-level restoration
skillshare mcp restore BACKUP_ID --no-tui            # Apply restoration; source stays unchanged
```

MCP supports 19 clients including Claude Code, Codex, Cursor, VS Code, OpenCode,
Grok, Antigravity, Amp, Cline, Copilot CLI, Factory, Gemini CLI, Goose, Junie,
Kiro, LM Studio, Warp, Windsurf and Claude Desktop. Client scope and transport
support vary; see the MCP command reference for native destinations and limits.
Interactive `mcp` provides search, details, add/edit/remove, sync and backup
browsing. Import supports multi-selection with one preview before saving the
batch. Use `--json` or `--no-tui` for automation; `edit`, `remove` and `restore`
require a name or backup ID when noninteractive.
For human-operated terminals: `skillshare mcp` opens the manager (`/` search,
Enter details, `a` add, `i` import, `e` edit, `x` remove, `s` sync, `b` backups).
`mcp add` guides URL/JSON setup; `mcp edit` opens a server picker and editor;
`mcp import --from claude` offers batch selection; `mcp restore` browses backups.
Review screens scroll before confirmation; Esc cancels without saving the draft.
The editor covers connection, arguments, env, headers, bearer-token env name,
and targets. Scripted `edit` accepts `--url`, repeated `--target`, or `-- command args`;
switching transport clears incompatible fields. Add/edit/import/remove save only
unless `--sync` is supplied; the TUI instead offers Save only or Save and sync.
Definitions live in `mcp.servers`, or in `sources.mcp` (a YAML file with `servers`).
Never define both; external paths are relative to the config directory.
Use `mcp.targets` or per-server `targets`; MCP targets are independent of skill targets.
Credentials use `{fromEnv: VARIABLE}`. Never resolve secrets, launch servers or
copy OAuth credentials while managing settings. A matching native entry that
Skillshare does not manage stays unmanaged until imported. A differing one needs
`mcp import --replace` or an explicit entry replacement; `--force` cannot bypass it.
`sync --all` includes MCP as well as skills, agents and extras.

## Automation rules

Use `--no-tui` or `--json` and provide all required inputs. Receiving clients use
repeated `--target` (singular), not `--targets`; `--force` is unsupported.
Preview with `skillshare sync mcp --dry-run --json`, then apply with
`skillshare sync mcp --revision <revision>` to reject stale plans.
Noninteractive import without a name only lists candidates. Use `--replace`
only when replacement is intended. Sync writes settings; it does not start
servers or verify connectivity.
