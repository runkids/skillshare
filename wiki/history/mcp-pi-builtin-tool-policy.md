# MCP: Pi built-in only and the portable tool policy (0.23.0)

## Outcome

Pi receives MCP servers only through its built-in MCP (Pi >= 0.99.0):
`~/.pi/agent/mcp.json` (honoring `PI_CODING_AGENT_DIR`) and `.pi/mcp.json`. The
`piExtension` modes (`builtin`, `pi-mcp-adapter`, `pi-mcp-extension`), the
`piOptionsPrune` switch and `directTools` are retired (#143). A server-level
`tools: {expose, allow, deny}` policy is written once and translated per Agent (#140).

CLI: `--tools-expose`, `--tools-allow` and `--tools-deny` on `mcp add`, `edit` and
`import`. `--pi-extension`, `--pi-options-prune` and `--direct-tools` fail with a
"removed in 0.23.0" message that names the replacement. The dashboard server dialog
has a Tools section, including a "Load tools from server" action that runs the live
check for one saved server. It shows per-Agent "not applied" notes. Pi help sits in
info tooltips, and there is no page-level legacy notice.

## Design decisions

- Migration needs no special code path. Loading converts the retired keys in memory
  and records notices. The ownership ledger still owns the old `mcp-adapter.json`
  entries, so the plan removes them. Hand-written adapter entries are not owned and
  stay untouched.
- The first applied sync saves the converted config (`config.yaml` or `sources.mcp`)
  after the Agent files are written. It keeps the old file in the file history with
  the new backup reason `migrate`. Dry runs write nothing. `sync --all` runs MCP when
  only a migration is pending.
- `directTools` true/`"search"`/list map to Pi `exposure` direct/deferred or
  `toolExposure`. Adapter `includeTools`/`excludeTools` map to `tools.allow`/`deny`.
  The other adapter-only `piOptions` are dropped with a notice and refused on save.
- Tool policy per Agent follows only documented fields. Pi holds all of it: the
  `toolExposure` order is deny, then allow, then `"*": "hidden"`. Codex gets exact
  `enabled_tools`/`disabled_tools`, which are managed fields. Copilot gets exact
  `tools`. OpenCode and Kilo Code filter tools only in a top-level permission map, so
  nothing is written for them. Every unapplied part is named in the plan notices, in
  `mcp check` (`tools` warning) and in the dashboard.
- `mcp import --from pi` still reads `mcp-adapter.json`, read-only, for this release.

## Evidence and limits

Code commits: `09209c77..0c95f9ea` on `runkids/0.23`. Each phase ran `make check`,
vitest, the UI build and browser checks in the devcontainer. The
[MCP command reference](../../website/docs/reference/commands/mcp.md#tool-policy)
documents the behavior. The [MCP runbook](../../ai_docs/tests/mcp_runbook.md) steps
9–12 cover Pi built-in, the 0.22 migration, the removed flags and the tool policy per
Agent in an isolated HOME.

The runbook cannot create a 0.22 ownership ledger, so removal of an owned
`mcp-adapter.json` entry is covered by Go tests only. Native Agents did not
load or run the written tool policies.
