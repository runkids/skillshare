# Feature Proposal: Require positive and benign-twin fixtures for every built-in rule

Issue: https://github.com/runkids/skillshare/issues/730

## Problem

The audit suite already includes inline positive and negative cases in
`internal/audit/audit_scan_content_test.go`. It does not enforce paired,
full-skill fixtures for every built-in rule. A rule change can preserve its
positive match while adding noise that existing negative cases do not cover.

## Proposed Solution

Add skill-tree fixtures with expected and absent rule IDs, then enforce paired
coverage against the complete built-in rule inventory.

- Store small skill trees under a proposed fixtures subdirectory of
  `internal/audit/testdata`. Each case gets `expected.json` declaring required
  rule IDs and rule IDs that must be absent, with a short rationale for labels.
- Scan each tree through the real skill scanner with isolated configuration.
  Compare findings by stable rule ID rather than fragile message text, while
  keeping existing targeted unit tests for their useful local detail.
- Require every built-in rule to have a reachable positive fixture and a paired
  benign twin asserting that same rule is absent. List uncovered or stale IDs
  in failures and reject labels for rules that no longer exist.
- Enumerate built-ins through the rule source used by the scanner, including
  generated credential rules from `internal/audit/credentials.go`.
  `internal/audit/rules.yaml` alone is not the complete inventory;
  `internal/audit/patterns.go` appends generated credential rules.
- Include benign variants of credential access, legitimate Git/CLI use,
  documentation quoting risky commands and evaluation dataset content. Labels
  must follow the intended policy; quoted commands are not automatically safe.
- Cover each rule's applicable file context. Avoid a passing negative caused
  only by an unrelated file filter, disabled rule or unvisited fixture path.
  Keep pair relationships explicit so one generic clean file cannot stand in
  for meaningful benign coverage of the entire rule set.
- Keep the all-rule check in the normal test gate. Deliver the complete fixture
  inventory with it; a handful of seeded pairs is an intermediate milestone.

The harness belongs with existing scanner tests and `internal/audit/testdata`.
No new runtime analyzer or default audit policy is proposed. Dataset-exclusion
or MCP analyzer work may later add fixtures for its own behavior; neither is
needed to establish coverage for today's built-in rules.

## Alternatives Considered

Checking only YAML entries omits generated rules. More inline tests without an
inventory gate cannot show what remains uncovered. A labeled accuracy corpus
could share fixtures, but its metrics do not replace this per-rule obligation.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines), including the complete paired fixture data

Estimate 150–250 harness lines plus 700–1,500 lines of fixture data, potentially
more after inventorying contexts. No prerequisite feature exists; coordination
with corpus work is optional reuse. This proposal includes no tests or coverage
claim and does not treat a small initial seed as completion.

## Open Questions

- Is one meaningful pair per rule sufficient, or do some contexts need more?
- Should unexpected unrelated findings fail fixtures, or only declared assertions?
- How should policy changes update labels without rubber-stamping regressions?
- Should non-regex analyzer findings get an analogous coverage inventory later?
