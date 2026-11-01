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
- Record recognized datasets and their actual static-analysis disposition.
  If static analysis ran with an exemption, mark the file excluded; if static
  was disabled by analyzer selection, record that distinct reason rather than
  claiming an exemption was applied. If opt-in scanning runs, record the actual
  outcome. Expose counts and reasons in terminal, Markdown, JSON and SARIF;
  define SARIF invocation notifications/properties so an empty findings list
  cannot conceal exclusions. Do not count excluded bytes as inspected bytes or
  present exclusions as a clean verdict.
- Adapt bounded discovery so an oversized recognized dataset is disclosed before
  the current content-size skip, without reading its full contents. For paths
  beyond the normal walk depth, use a separately bounded metadata-only discovery
  policy or explicitly report the unvisited frontier as incomplete; do not claim
  every deep dataset was found when a limit prevented traversal. Test oversize,
  depth-boundary and truncated-frontier cases.
- Preserve integrity accounting and hash verification independently of the
  static exemption, using `internal/audit/analyzer_integrity.go`.
- Explain that dataset paths are author-controlled and that strict users can
  scan them. Retain other applicable analyzers rather than a blanket bypass.

Policy belongs with global and project audit configuration and resolved policy:
`internal/config/config.go`, `internal/config/project.go`,
`internal/audit/policy.go`, and both global/project configuration schemas must
accept the selected option. Test global/project precedence, profiles and
`enabled_analyzers`/`--analyzer` combinations. Collection and result changes
belong with `internal/audit/audit_scan_skill.go` and
`internal/audit/audit_result.go`; all report consumers need disclosure support.
A general completeness contract could supply shared dispositions if available,
but is not a prerequisite for explicit output.

## Acceptance criteria

- With static enabled and an exemption selected, dataset bytes remain in total
  scope but not inspected bytes. Verify the resulting analyzability ratio and
  any low-analyzability finding and risk-score effect across all consumers.
  With scanning selected, inspect admitted bytes and report actual partial/limit
  outcomes; never force the ratio or score to appear complete.
- Exercise static-only, non-static-only and mixed analyzer selections; distinguish
  an exemption from an analyzer disabled for the entire invocation. Preserve
  applicable integrity checks independently, including changed-file failures.
- Verify matching/nonmatching paths, JSONL policy reversal, oversized and deep
  inputs, exact discovery limits and disclosed incomplete frontiers. Confirm
  terminal, Markdown, JSON and SARIF retain counts/reasons in zero-finding cases.
- Document option defaults, project/global resolution, coverage and strict-mode
  implications in the shipped skill reference and website audit documentation.

## NVIDIA source and adaptation

There is a source conflict at the pinned revision. SkillSpector's [dataset documentation](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/docs/EVAL_DATASETS.md#L3-L21) describes static exclusions for the listed paths, but its [static-pattern regression](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/analyzers/test_static_patterns.py#L881-L906) asserts findings for two evaluation paths, and its [ledger regression](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/test_security_remediation.py#L1773-L1782) asserts completed static inspection of an evaluation file. The [static runner](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/static_runner.py#L3212-L3228) iterates components without an evaluation-path exemption in that loop. The ledger defines an [`EVAL_DATASET` reason](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/inspection_ledger.py#L174-L176), and a [synthetic ledger test](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/test_inspection_ledger.py#L47-L58) uses it; that fixture does not prove the static runner emits it. These are source observations, not a claim that tests were run here.

The documented path family is only historical or indirect inspiration; do not claim current NVIDIA runtime skips datasets. Skillshare's default versus opt-in exemption, strict profile, wildcard scope, JSONL handling and truthful coverage disclosures require independent policy approval. A path chosen by an author is not evidence that its contents are safe.

## Alternatives Considered

Rule-by-rule exclusions require repeated configuration and obscure why a file
was skipped. Scanning every dataset preserves coverage but retains the noise.
An opt-in exemption is a reasonable safer alternative: use the same policy
knob with scanning enabled by default and require an explicit exclusion. Its
default must be decided before implementation.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines), including policy and output consumers
- [ ] Large (10+ files, 500+ lines)

Cover matching/nonmatching paths, case handling, JSONL policy reversal,
identical hostile text in a dataset and a script, truthful byte accounting and
integrity changes. No implementation or change to default coverage is included.

## Open Questions

- Approve default static exclusion with opt-in scanning, or default scanning
  with an explicitly opted-in exemption?
- Are these exact path families sufficient, and should matching be case-sensitive?
- Should the original broader `eval/dataset.*` request remain narrowed to the
  named JSON/JSONL/YAML/YML formats, or should any other extensions be included?
- Should strict profiles always scan recognized datasets despite the default?
- Which non-static analyzers should continue to inspect dataset content?
