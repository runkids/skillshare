# Feature Proposal: Compare declared allowed tools with bundled capabilities

Issue: https://github.com/runkids/skillshare/issues/732

## Problem

A skill can declare narrow allowed tools while bundled scripts use shell,
network, files or environment access. Existing tier classification describes
command behavior, but does not establish tool-permission equivalence or compare
observed capabilities with the skill declaration.

## Proposed Solution

Add a skill-scope capability comparison, initially active only under the
strict audit profile. Treat results as static evidence rather than enforcement
of what a target agent can actually execute.

- Parse SKILL.md `allowed-tools` through the general frontmatter parser. Define
  accepted scalar/list forms, target-specific names and malformed declarations.
- Publish an explicit mapping from supported tool declarations and scoped tool
  arguments to shell, network, file and environment capabilities. Do not assume
  that a command tier or one target's tool name grants the same permission elsewhere.
- Extract used capabilities from non-test bundled scripts using compatible tier
  and dataflow evidence, extended where they lack the required categories.
  Retain file/line evidence and confidence; unknown dynamic use stays unknown.
- Emit undeclared-capability findings at HIGH for strong evidence and MEDIUM for
  uncertain evidence, wildcard permission at MEDIUM and unused declarations at LOW.
  Include a distinct missing-declaration diagnostic rather than treating absence
  as an explicit empty permission list.
- Define interaction-based suppression: once a missing declaration is reported,
  suppress duplicate per-category undeclared noise; broad wildcard grants cover
  their mapped capabilities but retain the wildcard finding. Do not suppress an
  unrelated strong undeclared capability because another heuristic matched.
- Exclude documented test/fixture paths and do not call absence of observed use
  proof that a permission is unnecessary. Display findings consistently across
  CLI, API and dashboard, with normal rule controls and strict-profile gating.

Useful foundations are `internal/utils/frontmatter.go`, `internal/audit/tiers.go`
and `internal/audit/policy.go`. Current tiers are optional evidence reuse, not a
complete capability mapper. This proposal has no prerequisite feature.

## Alternatives Considered

Tier counts alone conflate command behavior and tool permission. Checking only
wildcards misses underdeclarations, missing declarations and unused permissions.
Runtime sandbox enforcement is a separate feature from this static comparison.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate includes declaration/target semantics, extraction, interaction policy,
strict activation, reports, docs and fixtures. Cover clean, underdeclared,
overdeclared, wildcard, missing and malformed cases, dynamic uncertainty,
test exclusion and duplicate suppression. No capability analyzer is implemented here.

## Open Questions

- Which target tool names and scoped argument forms should the first mapping support?
- What severity should a missing declaration have under strict policy?
- Which evidence justifies HIGH rather than MEDIUM for undeclared capability?
- Should unused-permission findings require complete script coverage before emission?
