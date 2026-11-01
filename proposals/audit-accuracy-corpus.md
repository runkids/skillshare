# Feature Proposal: Gate audit accuracy against a labeled skill corpus

Issue: https://github.com/runkids/skillshare/issues/725

## Problem

Positive and negative unit tests already exist in
`internal/audit/audit_scan_content_test.go`. They do not provide a labeled
skill corpus with per-rule precision/recall accounting or a deterministic
regression gate. A change can improve one rule while making routine skills noisy.

## Proposed Solution

Build the complete initial corpus gate, with labels describing actual expected
behavior rather than snapshots that automatically bless current scanner output.

- Add benign and malicious skill trees under the proposed corpus location in
  `internal/audit/testdata`, with expected and forbidden rule IDs per case.
  Include mixed-content skills, ordinary documentation, scripts and manifests,
  not only isolated strings that exactly reproduce existing unit tests.
- Define the unit of measurement as a case/rule label. Count true positives,
  false positives and false negatives per rule, aggregate them consistently,
  and report precision/recall with explicit handling of zero denominators.
- Require reviewed labels and explain ambiguous cases. Define whether unrelated
  unexpected findings fail a case; do not silently count unlabeled results as
  correct or treat every benign-looking example as universal ground truth.
- Add a deterministic test to the normal `make test` path in `Makefile`.
  Fail on forbidden findings, missing required findings and approved metric
  regressions, with named cases and per-rule counts in the failure output.
- Run with isolated configuration and stable built-in rules. Version the corpus,
  metric policy and baseline together; adding cases must not invisibly reset
  thresholds or conceal a newly introduced false positive.
- Represent the supported detection families with a documented corpus inventory.
  A small seed is useful during development but does not complete the proposed
  systematic gate. Published metrics describe this corpus, not real-world rates.

Keep explicitly later scope visible: compare two builds/revisions with per-rule
JSON/Markdown deltas, an approved regression-policy/override file, optional
external private corpus paths, and PR comment automation in
`.github/workflows/test.yaml`. Those are follow-on deliverables, not implied
by the initial test gate. External corpus contents must never enter public reports.

`internal/audit/rules.yaml`, `internal/audit/credentials.go` and
`internal/audit/patterns.go` define relevant built-in rule sources. No separate
feature is required. A benign-twin fixture project could share labels/harnesses;
that is optional reuse and a merge-coordination concern, not a prerequisite.

## Alternatives Considered

More unit cases remain useful but do not supply corpus-level accounting.
Aggregate pass percentages hide which rules regress. Starting with two-build
PR automation adds complexity before the labels and metric contract are stable.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines), counting the complete corpus and policy

Estimate 250–500 harness/test lines initially plus representative labeled data;
later comparison/automation work needs its own estimate. This proposal includes
no corpus, implementation, accuracy measurement or claim of complete coverage.

## Open Questions

- Which detection families and minimum case diversity make the initial gate useful?
- Should the first policy allow any regression, or require reviewed baseline edits?
- How should multi-rule cases and unknown labels affect precision/recall?
- Which later comparison and private-corpus features are worth maintaining?
