# Feature Proposal: Discover and audit skills in an arbitrary tree

Issue: https://github.com/runkids/skillshare/issues/704

## Problem

A directory can already be audited as one target and managed skills scan in
parallel. Users cannot discover every SKILL.md as an independent result under
an arbitrary tree with per-skill failures, cancellation and ranked output.

## Proposed Solution

Extend the existing audit command and parallel runner rather than a new CLI.

- Propose `--discover <dir>`, `--jobs N`, `--skill-timeout 30s` and `--top N`.
  Validate positive limits and bound discovery depth and entry count.
- Discover SKILL.md parent directories deterministically, skipping `.git` and
  `node_modules`; define nested-skill, hidden-path and symlink policies.
- Reuse parallel scheduling with actual cooperative scan cancellation.
  A timeout wrapper that abandons a running scan is insufficient.
- Record malformed, unreadable, timed-out and never-started skills explicitly;
  continue independent targets and retain findings already produced.
- Sort by maximum severity, then finding count with stable path tie-breaking.
  Top-N limits human display, not underlying findings, failures or exit gating.
- Emit one structured report with full per-skill statuses and aggregate counts,
  while keeping single-path and managed-source modes compatible.
- Use a shared completeness contract for partial work; do not represent a
  timeout or failed scan as a zero-finding clean skill.

Add cooperative cancellation to scanning and ParallelScan as part of this
feature. Reuse an agreed resource/completeness contract if one is available;
discovery itself has no prerequisite feature.

Relevant source: `cmd/skillshare/audit.go`, `internal/audit/parallel.go`.

## Alternatives Considered

Running separate CLI processes can isolate failures but duplicates configuration
and report merging. Reusing ParallelScan without cancellation does not satisfy
the per-skill deadline requirement.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate 300–550 production and 250–450 test lines across discovery, CLI,
scheduling, reporting and documentation.
Cover nested trees, ignored paths, malformed skills, cancellation, failures,
ordering, top-N visibility and unchanged exit decisions with synthetic fixtures.
No implementation, test result or maintainer approval is claimed.

## Open Questions

- Approve discovery/symlink defaults and the aggregation/exit contract?
- Should a nested skill be audited independently from its parent?
- What deadline overshoot is acceptable for non-interruptible operations?
