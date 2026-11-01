# Feature Proposal: Compare declared allowed tools with bundled capabilities

Issue: https://github.com/runkids/skillshare/issues/732

## Problem

A skill can declare narrow allowed tools while bundled scripts use shell,
network, files or environment access. Existing tier classification describes
command behavior, but does not establish tool-permission equivalence or compare
observed capabilities with the skill declaration.

## Proposed Solution

Add a skill-scope capability comparison, initially active only under the
strict audit profile. Findings are static evidence, not enforcement of what a
target agent can actually execute; if included in audit policy they may still
affect audit blocking or install/update gates. Maintainers must explicitly choose
those policy consequences before implementation.

- Extend frontmatter parsing to distinguish absent, explicitly empty, valid
  scalar/list and malformed declarations. The existing list helper does not
  provide those distinctions or parse the scalar form in the shipped skill
  template; define bounded tokenization of scoped scalar arguments rather than
  assuming the helper is sufficient. Define target-specific names and errors.
- Publish an explicit mapping from supported tool declarations and scoped tool
  arguments to shell, network, file and environment capabilities. Do not assume
  that a command tier or one target's tool name grants the same permission elsewhere.
- Extract used capabilities from non-test bundled scripts using compatible tier
  and dataflow evidence, extended where they lack the required categories.
  Include executable-looking SKILL.md code-fence commands as observed static
  evidence, with examples/intent uncertainty and negative fixtures rather than
  asserting they actually run. Retain file/line evidence and confidence;
  unknown dynamic use stays unknown.
- Emit undeclared-capability findings at HIGH for strong evidence and MEDIUM for
  uncertain evidence, wildcard permission at MEDIUM and unused declarations at LOW.
  As a proposed starting distinction, strong evidence is a supported literal
  command or call with a directly mapped capability; uncertain evidence is a
  supported but conditional or dynamically resolved use whose capability is not
  proven. Unknown mappings must remain unknown rather than manufacture a HIGH
  finding. Maintainers must approve the taxonomy and severity policy.
  Include a distinct missing-declaration diagnostic rather than treating absence
  as an explicit empty permission list.
- Define interaction-based suppression: once a missing declaration is reported,
  suppress duplicate per-category undeclared noise; broad wildcard grants cover
  their mapped capabilities but retain the wildcard finding. Do not suppress an
  unrelated strong undeclared capability because another heuristic matched.
- Propose excluding `testdata/`, `tests/` and `fixtures/` path components from
  observed-use inference, document the exact case/path matching and allow
  maintainers to choose the final list. Such author-controlled paths can hide
  behavior, so expose excluded scope and never call absence of observed use proof
  that a permission is unnecessary. Display findings consistently across CLI,
  API and dashboard, with normal rule controls and strict-profile gating.
- Add explicit profile-to-analyzer activation plumbing; existing profile presets
  and install/update gates do not automatically enable this analyzer. Specify
  global/project resolution, install/update profile propagation and what an
  explicit analyzer enable under a non-strict profile does. Decide whether HIGH
  findings participate in CLI audit blocking and install/update gates; disclose
  disabled or unavailable coverage rather than claiming an enforced sandbox.

## Acceptance criteria

- Fixtures distinguish absent, empty, malformed, supported scalar and list
  declarations, including the shipped scalar template and scoped arguments;
  each has a documented status and no false missing-declaration finding.
- Approved literal and uncertain capability fixtures produce the agreed stable
  rule IDs and severities; wildcard, unused and missing cases follow the chosen
  interaction rules. A dynamic unknown does not become a strong finding.
- Bundled-script and SKILL.md fence examples, benign prose, excluded path
  components and their disclosed coverage are tested independently. Test each
  rule's disable and severity controls and preservation of unrelated findings.
- Strict and non-strict CLI/API/dashboard and install/update cases verify the
  approved activation, explicit override, blocking and disabled-coverage policy;
  JSON and SARIF retain rule, severity, original location and confidence.

Useful foundations are `internal/utils/frontmatter.go`, `internal/audit/tiers.go`
and `internal/audit/policy.go`. Current tiers are optional evidence reuse, not a
complete capability mapper. This proposal has no prerequisite feature.

## NVIDIA source and adaptation

SkillSpector's [`_normalize_allowed_tools` and `_detect_capabilities`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/mcp_least_privilege.py#L246-L308) normalize declarations and identify capability patterns. Its [`_map_allowed_tools_to_categories`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/mcp_least_privilege.py#L353-L396) maps supported tool names to categories, while [least-privilege tests](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/test_mcp_least_privilege.py#L315-L348) exercise an undeclared shell capability and explicitly distinguish static evidence from runtime permissions.

This is direct inspiration for declaration-versus-observation analysis. Skillshare's target-specific mappings, strict-only activation, interaction rules, unknown evidence and selected severities are independent policy. Comparing one skill's declared and observed capabilities is not the old-versus-new update-consent decision and does not depend on it.

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
- Should explicit analyzer enable override non-strict profile defaults, and
  should these static findings participate in install/update gates?
- Which fixture path components and code-fence contexts are excluded from
  observed-use inference, and how is that coverage surfaced?
