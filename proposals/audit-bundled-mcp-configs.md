# Feature Proposal: Audit bundled MCP server configuration

## Problem

A skill can include MCP server definitions as well as instructions and scripts.
The audit engine currently scans supported text but does not interpret those
definitions as server launch configuration. Existing install-command rules do
not establish whether a configured launcher is pinned or which environment
values and remote destinations it exposes.

## Proposed Solution

Add a structured MCP configuration analyzer for explicitly supported bundled
files, such as `.mcp.json`, `mcp.json` and `claude_desktop_config.json`.
Reuse compatible parsing from `internal/mcp` where the package boundaries
allow it, and register the findings through the existing audit engine.

- Inspect launcher command/argument arrays for unpinned `npx`/`bunx`/`pnpm dlx`,
  `uvx`/`pipx run` and Docker images. Parse scoped package names and versions
  rather than relying on unsupported regex lookahead or a blanket `@` test.
- Report remote server URLs according to an explicit transport/host policy.
  Do not classify every remote MCP connection as malicious or make network
  requests while auditing configuration.
- Inspect environment forwarding and permissions. Distinguish intentionally
  declared credential references from broad or unexplained passthrough.
- Include server name and JSON location in findings. Redact credentials and
  URL userinfo from snippets, arguments and diagnostic messages before they
  reach any report format or audit log.
- Support rule listing, disabling and severity overrides consistently with
  existing analyzers. Report malformed or unsupported configuration explicitly
  instead of presenting it as an inspected, clean server definition.

The analyzer would use bounded reads and explicit file discovery, including
the named hidden files. It would not read the user's real MCP configuration,
launch servers, install packages, resolve credentials or contact remote hosts.

## Alternatives Considered

Additional install-command regexes are useful for prose but cannot reliably
interpret structured launcher arguments or environment maps. Reusing the MCP
management service wholesale could introduce unnecessary mutation or package
dependencies; shared pure parsing is preferable where needed.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines), including complete regression coverage

The complete change includes parser integration, bounded collection, findings,
redaction, policy and documentation consumers. Fixture-only tests should pair
pinned/unpinned launchers, legitimate remote servers, declared credentials and
unexplained passthrough, malformed files and redaction cases. No analyzer is
implemented by this proposal.

## Open Questions

- Which bundled file names and configuration versions should be supported?
- Which launcher/version forms count as sufficiently pinned, and at what severity?
- Should remote-host allowlists be optional policy, and how should trusted private
  HTTP connections relate to Skillshare's existing MCP transport options?
- Which environment-forwarding cases warrant findings without flagging normal
  intentional credential references as inherently unsafe?
- How should parse failures be represented until a general scan-completeness
  contract is available?
