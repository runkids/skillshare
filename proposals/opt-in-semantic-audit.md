# Feature Proposal: Add an opt-in, isolated semantic audit stage

Issue: https://github.com/runkids/skillshare/issues/698

## Problem

Static analysis cannot resolve every natural-language instruction or intent
mismatch. The audit package has no model-provider stage. Introducing one adds
content disclosure, nondeterminism and subprocess capabilities that need an
explicit contract before any provider is enabled.

## Proposed Solution

Keep the complete stage and its safety/accounting requirements together.

- Propose `audit --llm` and explicit provider selection, plus an opt-in install
  audit path. Remain off by default and never run automatically during sync.
- Add a provider interface for availability and structured completion through
  supported authenticated local CLIs. Availability is not proof of isolation.
- Pass bounded, line-numbered chunks with overlap as untrusted content; validate
  JSON severities, locations, confidence and rule IDs before creating findings.
  Attribute findings to an `llm` analyzer and retain normal JSON/SARIF reporting.
- Enable a CLI only after verifying no tools, MCP, hooks, settings discovery,
  slash commands or session persistence; use a scrubbed environment, replaced
  system prompt, disposable working directory and capped stdout/stderr.
  Preserve only explicitly required authentication access, with a documented
  boundary. A provider that cannot meet it stays unavailable; no silent fallback.
- Record requested/resolved provider and model, honored controls and actual
  provider-reported usage; missing counters stay unknown. Never log prompts,
  credentials or inferred prices as measured usage.
- Bound calls, concurrency, file/chunk size and time; warn about billing on first
  use and offer a dry-run plan. Cache by content, prompt version and model.
- Add required-pass mode that reports unavailable/failed/skipped batches as
  incomplete and fails its gate. Optional mode must still disclose every gap.
- Keep static-finding confirm/dismiss/downgrade triage advisory by default;
  preserve originals and never silently remove HIGH/CRITICAL findings.
- Supply mock-provider and record/replay tests keyed to prompt identity, offline
  malformed/empty/truncated-output cases and an explicit labeled eval lane.

Relevant source: `internal/audit/analyzer_registry.go`, `internal/audit/audit.go`, `internal/audit/sarif.go`.

## Alternatives Considered

More static rules remain useful but do not provide semantic review. Reusing
an unrestricted interactive CLI would expose the audit to tool execution.
A cloud SDK is a different authentication and disclosure design.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate 1,800–3,500 production lines plus 1,200–2,500 test lines; provider
isolation feasibility must be settled before provider enablement.
Validate isolation, deadlines, failure accounting, provenance, cache invalidation
and unchanged static findings through offline mocks; no live model experiment here.
No implementation, test result or maintainer approval is claimed.

## Open Questions

- Which providers can satisfy a verifiable capability and authentication boundary?
- What content may leave the machine, and what user approval/disclosure is required?
- Which required-pass and advisory-triage semantics should be supported first?
