# Feature Proposal: Test audit and install resource ceilings with hostile fixtures

Issue: https://github.com/runkids/skillshare/issues/719

## Problem

Existing audit tests cover many detection behaviors, and
`internal/audit/scan_benchmark_test.go` measures rule scaling. Those checks do
not establish the proposed aggregate audit and remote-install resource limits.
Resource guarantees need fixtures that exercise exhaustion and cleanup directly.

## Proposed Solution

Add generated, reproducible hostile-input tests tied to a documented table of
ceilings, keeping large performance experiments outside the routine fast suite.

- Generate file floods, 100-level nesting, oversized files and a 50 MiB line in
  temporary directories. Use bounded generators so test setup has its own
  resource ceiling; do not commit huge fixtures.
- Exercise each regex-heavy rule with large near-match input within admitted
  file limits, checking termination and useful diagnostics. Go regex behavior
  does not justify claims that every analyzer has a hard wall-clock guarantee.
- Use fixture HTTP servers and synthetic bodies for never-ending/slow remote
  downloads. Verify bounded reads, cancellation and typed limit errors; close
  every server/body and preserve an existing installation on failure.
- Check exact limits and one-over cases for aggregate bytes, files, findings,
  line length and elapsed deadlines. Assert retained findings, partial Coverage
  reasons and that cancellation stops the work rather than only the waiter.
- Keep baseline file/depth tests runnable before new budget features land.
  Record current behavior honestly; do not assert future limits against code
  that has no implementation of them.
- Add larger flood and per-rule timing/allocation benchmarks in an explicit
  performance lane. Use generous termination guards for CI variability and
  deterministic counters for the actual policy boundary.

Use `internal/audit/audit.go`, `internal/audit/parallel.go`,
`internal/install/github_download.go` and `internal/install/gitea_download.go`
as entry points. Documentation should list each ceiling, its enforcement stage,
fixture, units and expected exhaustion result.

Tests of new install ceilings genuinely require the remote-install budget
implementation. Tests of new audit ceilings require configurable scan budgets;
Coverage assertions also require the completeness contract. Existing-limit
fixtures can start now, but do not complete the full requested guarantee suite.
Archive bomb/forged-count tests belong with archive inspection if that feature
lands; archive parsing is not a prerequisite for the baseline audit/install suite.

## Alternatives Considered

Benchmarks alone can expose a slowdown but do not verify cleanup or partial
results. A single outer test timeout can catch a hang while missing work that
continues in the background. Real networks add variability without needed coverage.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines), plus generated data and documentation
- [ ] Large (10+ files, 500+ lines)

Estimate 250–450 test lines initially, with additional cases as limit contracts
settle. This proposal supplies no tests or measurements; it defines the full
suite and its prerequisites, not a claim that fixture-only checks finish it.

## Open Questions

- Which benchmarks belong in scheduled CI versus an explicit local lane?
- What termination margin is acceptable without making timing the policy oracle?
- Can the audit/install limit contracts expose deterministic accounting for tests?
- Should archive-bound fixtures become a separate optional extension of this suite?
