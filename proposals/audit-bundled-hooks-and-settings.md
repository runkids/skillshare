# Feature Proposal: Audit execution surfaces in bundled hook and settings files

Issue: https://github.com/runkids/skillshare/issues/714

## Problem

Bundled hooks and settings can declare automatic commands, permissions and
helper commands outside SKILL.md instructions. JSON text is eligible for scan,
but the skill walker skips hidden directories such as `.claude`; a structured
analyzer relying only on its current cache would miss those settings.

## Proposed Solution

Add a skill-scope analyzer for an explicit list of bundled configuration paths:
`hooks.json`, `hooks/hooks.json`, `.claude/settings.json` and
`.claude/settings.local.json`. Inspect skill content, not live user settings.

- Collect these paths explicitly with bounded reads and existing source-root/link
  protections. Hidden-directory support must not turn into an unrestricted crawl
  of `.git`, unrelated hidden trees or files outside the audited skill.
- Parse supported hook/event structures and report lifecycle auto-run commands.
  Propose MEDIUM for an executable hook, escalating to HIGH for network-capable
  commands or non-loopback destinations; distinguish a capability from proven
  exfiltration and show the command evidence.
- Recognize remote POST behavior separately from merely mentioning a URL.
  Inspect broad permission entries such as Bash wildcards and bypass modes.
- Report command-valued helper fields such as apiKeyHelper and statusLine at
  proposed HIGH severity, only where the supported schema gives them that meaning.
- Attach file, event/helper identity and JSON path to findings. Redact credentials
  before snippets, diagnostics, logs or any report formatter receives them.
- Emit an explicit informational parse/unsupported-shape diagnostic for malformed
  or unsupported files. Such a diagnostic must not imply the surface was clean.
- Register rule IDs for listing, disabling and severity overrides, and display
  findings through normal CLI, API and dashboard audit reports.

Integration points are `internal/audit/audit_scan_skill.go` and
`internal/audit/analyzer_registry.go`. Managed-hook save/sync auditing is separate;
this bundled-content analyzer has no dependency on that proposal.

## Alternatives Considered

Text scanning alone does not identify event and permission semantics. Reading
only current cached JSON misses `.claude`. Launching the bundle to discover
commands would execute the content the audit is meant to inspect.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines)
- [ ] Large (10+ files, 500+ lines)

Estimate may expand for collection, redaction and schema fixtures. Cover all
named locations, event variants, remote POST, benign local hooks, permissions,
helper fields, malformed JSON, size limits and credential-safe reports.
No hook is executed and no analyzer is implemented by this proposal.

## Open Questions

- Which hook/settings schema versions and event forms should be supported first?
- Should all lifecycle hooks be MEDIUM, or only events with automatic execution?
- Is HIGH appropriate for every network-capable command, including benign fetches?
- How should informational parse failures affect gating under stricter policy?
