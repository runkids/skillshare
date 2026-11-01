# Feature Proposal: Identify evaluation datasets in audit output

Issue: https://github.com/runkids/skillshare/issues/696

## Problem

Evaluation datasets can contain deliberately hostile prompts. Scanning those
prompts as skill instructions creates findings that obscure the behavior users
actually intend to install. Exempting a path also creates a place for an author
to hide content, so this needs an explicit coverage policy.

`internal/audit/audit_scan_skill.go` has no evaluation-specific disposition.
It recognizes JSON and YAML as scannable text; JSONL is outside that extension
list today. The proposal must report recognized datasets in either case.

## Proposed Solution

Recognize a narrow set of skill-relative dataset paths and propose excluding
only their static pattern analysis by default, with a visible disclosure.

- Start with `evals/evals.{json,jsonl,yaml,yml}` and
  `eval/dataset.{json,jsonl,yaml,yml}` as proposed exact path families.
  Do not exempt an entire directory or arbitrary `dataset.*` extensions.
- Keep manifests and scripts subject to their normal analysis even when they
  sit under an evaluation directory. A dataset label does not confer trust
  or permission to execute its contents.
- Add proposed `audit.scan_eval_datasets: true` policy to restore static
  inspection of these datasets, including the named JSONL format.
- Record each recognized dataset as data excluded from static analysis.
  Show counts and reasons in JSON and a concise human summary; do not count
  excluded bytes as inspected bytes or present exclusions as a clean verdict.
- Preserve integrity accounting and hash verification independently of the
  static exemption, using `internal/audit/analyzer_integrity.go`.
- Explain that dataset paths are author-controlled and that strict users can
  scan them. Retain other applicable analyzers rather than a blanket bypass.

Policy belongs with `internal/config/config.go`; collection and result changes
belong with `internal/audit/audit_scan_skill.go` and
`internal/audit/audit_result.go`. A general completeness contract could supply
shared dispositions if available, but is not a prerequisite for explicit output.

## Alternatives Considered

Rule-by-rule exclusions require repeated configuration and obscure why a file
was skipped. Scanning every dataset preserves coverage but retains the noise.
An opt-in exemption is a reasonable safer default if maintainers reject a
built-in exclusion; the default must be decided before implementation.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines), including policy and output consumers
- [ ] Large (10+ files, 500+ lines)

Cover matching/nonmatching paths, case handling, JSONL policy reversal,
identical hostile text in a dataset and a script, truthful byte accounting and
integrity changes. No implementation or change to default coverage is included.

## Open Questions

- Approve default static exclusion, or require users to opt in?
- Are these exact path families sufficient, and should matching be case-sensitive?
- Should strict profiles always scan recognized datasets despite the default?
- Which non-static analyzers should continue to inspect dataset content?
