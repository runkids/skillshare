# Feature Proposal: Report audit scan completeness

Issue: [#689](https://github.com/runkids/skillshare/issues/689)

## Problem

A clean audit report does not currently establish that every relevant file was
inspected. The walker skips oversized, deep, binary or unreadable content, and
some analyzer errors are discarded. Aggregate analyzability is useful but does
not identify every skipped file or explain which analyzers actually ran.

## Proposed Solution

Add a shared coverage contract to audit results and use it consistently in
CLI reports, installation policy and the dashboard.

- Record relative file paths with scanned, partial or skipped dispositions and
  stable reasons such as size, depth, binary, encoding, unreadable content,
  resource limits and analyzer failure. Record the analyzers that ran.
- Distinguish an intentionally excluded policy scope from a scan that could
  not complete. Show explicit warnings when analyzer selection narrows scope.
- Emit coverage and incomplete state in JSON, text and Markdown, and surface
  failures through SARIF execution notifications. Keep findings already found.
- Add `audit --fail-on-incomplete` and matching policy configuration. Agree how
  strict install policy handles incomplete inspection, keeping actual audit
  execution failures fail-closed rather than treating them as accepted findings.
- Scan oversized text in bounded overlapping windows where useful. Preserve
  source offsets and deduplicate overlap findings. Do not claim full analysis
  when window boundaries, encoding or parser context leave a coverage gap.

The main integration points are `internal/audit/audit_scan_skill.go`, the
parallel runner, `audit.Result`, the report exporters and existing CLI/install
policy consumers. Keep one definition of completeness across these consumers.

## Alternatives Considered

Counting only readable bytes cannot explain omitted artifacts. Turning every
skip into a threat finding confuses coverage with malicious behavior. A ledger
without windowed scanning improves visibility but leaves the original large-text
inspection requirement unfinished.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

This is a result/policy contract spanning collection, analyzers and consumers.
Fixture-only tests should cover each disposition, selected-analyzer runs,
unreadable content, analyzer errors, oversized-window boundaries, stable source
locations, partial findings and the incomplete-policy gate in both modes.
No implementation or change to current scan/exit policy is included here.

## Open Questions

- Which intentional exclusions affect completeness, and which define the
  declared inspection scope? Binary artifacts need an explicit policy.
- Should strict profiles enable the incomplete gate by default, or should it
  remain separately configured?
- What error/exit representation should incomplete-policy failures use, keeping
  them distinct from threshold findings where the CLI supports that distinction?
- Which analyzers can preserve meaningful context across text windows? Fixed
  overlap alone cannot guarantee arbitrary regex or parser-context coverage.
- How should the contract accommodate later aggregate scan budgets without
  requiring another incompatible result format?
