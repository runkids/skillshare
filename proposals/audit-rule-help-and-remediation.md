# Feature Proposal: Explain audit findings and remediation across reports

Issue: https://github.com/runkids/skillshare/issues/710

## Problem

Rule messages and snippets identify a match but do not provide maintained
why-dangerous and what-to-do guidance. Help must follow rule overrides and
reach every report consumer rather than live in an unrelated lookup table.

## Proposed Solution

Extend existing rule metadata and populate the full built-in inventory.

- Add optional explanation/remediation fields to YAML rules, validate them and
  preserve them through global/project merging and custom rule loading.
- Populate useful guidance for every built-in rule, including generated
  credential rules; a schema-only seed is not completion.
- Expose help in `audit rules`, Markdown reports and the dashboard’s per-finding
  expander rows, with stable rule identity and escaping appropriate to each format.
- Emit SARIF rule help with the available explanation and remediation, retaining
  current result locations, severity and finding messages.
- Let custom rules provide their own text without executing links or templates.
  Define absent/empty override semantics so guidance is not silently lost.
- Keep guidance evidence-based: explain what was detected, common legitimate
  cases and a concrete review/remediation action, without promising safety.
- Add an inventory gate requiring nonempty explanation for every built-in rule,
  plus formatter/schema/override regression tests.

Relevant source: `internal/audit/patterns.go`, `internal/audit/rules_list.go`, `internal/audit/sarif.go`, `ui/src/pages/AuditPage.tsx`.

## Alternatives Considered

Longer messages alone overload compact reports. A separate hand-maintained
map can drift from generated rules and custom overrides. Linking to generic
security advice does not explain a specific finding.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate 250–450 plumbing lines plus 400–800 guidance-content lines and
150–250 test lines; all consumers and populated data are part of one feature.
Test generated-rule coverage, custom help, override behavior, escaping and
consistent SARIF/Markdown/listing/dashboard presentation.
No implementation, test result or maintainer approval is claimed.

## Open Questions

- Which guidance structure should custom rules and overrides share?
- Should remediation be mandatory for built-ins as well as explanation?
- How should localized UI display English rule help initially?
