# Feature Proposal: Accept known audit findings with reasoned, content-bound baselines

Issue: https://github.com/runkids/skillshare/issues/695

## Problem

Users need to accept a known finding without disabling its rule everywhere.
Existing finding acceptance already persists keys and removes acknowledged
findings from blocking. Those keys do not bind acceptance to file content or
engine version, and risk scoring still counts acknowledged findings.

## Proposed Solution

Add a versioned YAML baseline for global and project audit contexts. Propose
`audit-baseline.yaml` beside the active audit configuration, with explicit
file selection available as a proposed CLI option.

- Store generated exact fingerprints and hand-authored path/message glob rules;
  require a nonempty reason on every entry. Keep broad glob acceptance explicit.
- Bind exact entries to engine/rule version, relative file identity, file-content
  digest and finding identity, including severity and source location. Content
  or engine changes must make exact acceptance stale rather than silently carry it.
- Add proposed baseline generate/refresh operations, output-file and reason
  inputs, and audit options for an explicit baseline and suppressed visibility.
  Refresh must require reviewing new findings and supplying reasons.
- Apply acceptance before effective risk scoring, exit decisions and install,
  update and dashboard gates. Preserve raw findings separately for inspection.
- Show suppressed findings and reasons in CLI and dashboard reports; expose
  them in JSON and SARIF suppression records. Reject malformed baselines and
  unknown versions with a nonzero result rather than silently ignoring them.
- Keep existing acknowledgements readable during transition, distinguish their
  weaker identity in reports and offer explicit migration rather than deletion.

Relevant source is `internal/audit/accept.go`, `internal/audit/audit_result.go`,
`internal/install/audit_accept.go` and `internal/server/handler_audit.go`.
This is a new persistence and CLI contract with no prerequisite feature.

## NVIDIA source and adaptation

SkillSpector's [`finding_fingerprint`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/suppression.py#L218-L260) hashes canonical structured data including scanner version, file-content digest, path, severity, location and additional finding evidence; its [fingerprint tests](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/unit/test_suppression.py#L98-L118) cover field/content/version changes. [`partition_findings`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/suppression.py#L504-L545) separates effective and suppressed findings and prevents stale-version exact matches. The actual [SARIF renderer](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/report.py#L743-L784) includes suppressed results with an external suppression and reason for an audit trail; this is different from excluding them entirely from SARIF. An upstream [blank-reason test](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/unit/test_suppression.py#L376-L386) supplies a default reason, whereas this proposal requires a nonempty user reason.

These directly inspire evidence-bound acceptance and reportable suppression. Skillshare's exact hash schema, required reasons, global/project composition, migration, raw/effective scores and gate behavior remain separate approval decisions; do not assume the two fingerprint formats are interchangeable.

## Alternatives Considered

Rule-wide disabling is too broad. Existing acceptance is useful but lacks
reasoned review and content/engine invalidation. Automatically regenerating a
baseline during update could accept the very changes users need to review.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate includes format validation, generation, refresh/migration, all gating
and reporting consumers, documentation and fixture tests. Cover edited content,
engine changes, stale entries, glob reasons, bad versions and consistent scores
and exits. The proposal does not implement a baseline or change current acceptance.

## Open Questions

- Should an engine change invalidate all exact entries or only changed rule versions?
- Should glob entries require explicit renewal on engine changes as well?
- How should global and project baselines combine, and which file wins conflicts?
- Should effective and raw risk scores both be displayed during migration?
