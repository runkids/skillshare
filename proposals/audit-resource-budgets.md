# Feature Proposal: Bound audit resource use and report partial results

Issue: https://github.com/runkids/skillshare/issues/713

## Problem

The scanner has fixed per-file and depth limits in `internal/audit/audit.go`.
It has no configurable aggregate byte/file ceiling, finding cap or overall
deadline. `internal/audit/parallel.go` bounds workers but accepts no context.
Large skill trees can consume substantial work without a clear partial verdict.

## Proposed Solution

Add one explicit resource policy and cooperative cancellation through collection,
content analysis, skill analyzers and parallel scheduling.

- Propose `audit.max_total_bytes`, `max_files`, `timeout`,
  `max_findings_per_skill` and `max_line_bytes` configuration, plus an audit
  `--timeout` override. Resolve policy consistently in global/project modes,
  CLI, dashboard and install/update audit gates.
- Start discussion with 64 MiB aggregate bytes, 10,000 files, 30 seconds,
  1,000 findings per skill and 64 KiB per regex line. Retain existing file/depth
  ceilings initially; validate finite positive overrides and document units.
- Share aggregate bytes, files and elapsed budget across one audit invocation,
  including concurrent workers. Keep finding limits per skill and line limits
  per inspected line. Define counts for skipped files and integrity reads.
- Check cancellation before expensive stages and during reads/loops. Stop
  scheduling new skills and propagate context to supported operations; timing
  out a caller while its scan continues is not a resource guarantee.
- Preserve findings already produced. Add explicit Coverage status and reasons
  such as `runtime_limit`, with the exhausted budget and inspected/skipped
  counts. Keep unfinished and never-started skills visible in batch output.
- Bound line processing and report omitted content rather than silently treating
  a truncated prefix as a fully scanned line. Emit one informational
  `findings-truncated` notice when the finding allowance is exhausted.
- Propose that incomplete scans block automatic install/update approval unless
  an explicit separate policy permits them. A clean partial scan is not clean.

Relevant paths include `internal/config/config.go`,
`internal/audit/audit_scan_skill.go`, `internal/audit/audit_scan_content.go` and
`internal/audit/audit_result.go`. Coverage needs a shared completeness contract,
provided by coordinated work or delivered jointly here. Reuse of an existing
contract is preferred; a separate merged feature is not inherently required.

## Alternatives Considered

Per-request/per-file limits do not bound aggregate work. A deadline around a
waiting goroutine leaves the scanner running. Hard process isolation would
provide a different termination boundary and is outside this initial design.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimated implementation: 500–850 production lines plus boundary/cancellation
fixtures and generated large-tree benchmarks. Include 100,000-file benchmark
coverage outside the default fast suite. No limits or tests are implemented here.

## Open Questions

- Approve the proposed defaults and invocation-wide aggregation policy?
- Which incomplete-scan overrides should be allowed for install/update gates?
- How should integrity verification share a budget without losing its coverage?
- What deadline overshoot is acceptable for non-interruptible library calls?
