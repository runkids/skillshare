# Feature Proposal: Add audit and update-check tools to the MCP server

Issue: https://github.com/runkids/skillshare/issues/708

## Problem

Agents can already list and read skills through the existing MCP server.
They cannot request a structured audit or update check through those tools,
so these operations still require a separate CLI integration.

`internal/skillserve/tools.go` registers `list_skills` and `read_skill`.
`internal/skillserve/server.go` already uses the Go MCP SDK, supports optional
editing and provides an authenticated HTTP handler. A second server is unnecessary.

## Proposed Solution

Extend the existing server with read-only audit and update-check tools while
keeping its current transport, authentication and optional editing behavior.

- Add proposed `audit_skill(target, threshold, profile)` and `check_updates`
  tools. Resolve targets from the server-selected catalog or installed skills,
  respecting the selected global/project context and source-follow policy.
- Reuse `internal/audit` and `internal/check/resolve.go` for domain behavior.
  Return findings, resolved threshold/profile, blocking state and update statuses
  as structured results. Distinguish errors, unknown state and partial inspection.
- If exposing `safe_to_install`, define it only as a policy verdict under the
  reported scan scope. Prefer `passes_audit_policy`; no finding is not a
  guarantee that arbitrary skill behavior is safe.
- Reject arbitrary local paths and caller-supplied remote URLs. Do not turn an
  authenticated HTTP tool call into access to the server's entire filesystem
  or a general network-fetch interface.
- Keep audit local to selected content. Update checking may contact only the
  selected installed sources, subject to explicit server policy; propose it
  disabled for HTTP unless the operator enables outbound checks.
- Preserve authenticated non-loopback serving and existing HTTP protections.
  Apply new restrictions to these tools without making all serving loopback-only.
- Make the new tools available to clients using the native Skills extension;
  its current hiding of duplicate list/read tools should not hide audit/check.

Document tool schemas, policy resolution, transport-specific target restrictions
and existing server registration. No new `skillshare mcp` command, SDK migration
or replacement list/read interface is proposed.

## Alternatives Considered

Shelling out to JSON CLI output remains possible but duplicates context and
error handling. A separate MCP server adds configuration and transport policy
without improving the existing server's supported read/check operations.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines), potentially larger with documentation
- [ ] Large (10+ files, 500+ lines)

Estimated implementation: 230–400 production lines plus fixture-backed tool,
policy, transport and client-capability tests. No prerequisite feature or new
server is required. This proposal includes no implementation or network checks.

## Open Questions

- Approve the catalog-only target boundary and outbound-check defaults?
- Should callers be allowed to weaken server audit policy, or only strengthen it?
- Which structured field best expresses a limited audit verdict?
- Should audit/check tools be opt-in independently of existing edit tools?
