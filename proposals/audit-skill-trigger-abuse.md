# Feature Proposal: Audit broad, stuffed and shadowing skill triggers

Issue: https://github.com/runkids/skillshare/issues/709

## Problem

A skill description influences when an agent selects it. Broad instructions,
keyword stuffing and descriptions that imitate commands or another installed
skill can compete for unrelated requests. Metadata analysis currently checks
publisher mismatch and authority language rather than trigger selection.

## Proposed Solution

Extend metadata analysis with broad-trigger and keyword-stuffing checks, plus
command and installed-skill shadow comparisons. Propose MEDIUM findings with
stable rule IDs, individual disable support and CLI/dashboard visibility.

- Parse complete name/description frontmatter, including multiline descriptions,
  before applying trigger heuristics. Do not infer intent from length alone.
- Report explicit all-request triggers and repeated unrelated trigger terms;
  pair each heuristic with ordinary broad utility and detailed-description negatives.
- Define a versioned built-in command inventory tied to supported target tools.
  Match invocation-like names in their target context rather than every common word.
- Compare descriptions/names against other installed skills using a deterministic
  local index and documented similarity threshold. Exclude self-comparisons and
  identify the competing skill and evidence without choosing an installation winner.
- For install/update, compare candidate content to the existing installed set,
  replacing its own previous entry. Offline path-only audits should report when
  this comparison context is absent rather than imply a complete shadow check.
- Build the installed-description index once per audit invocation from the
  selected installed-skill context and share an immutable snapshot with parallel
  workers. Define source/context plumbing for install and update, bounded entries,
  bytes and construction time, and deterministic behavior if any limit or read
  fails. Do not rebuild the installed set for every candidate. Test concurrent
  scans, self-replacement, unreadable/missing context and a many-skill fixture
  that checks bounded construction and reuse; choose concrete limits and latency
  targets with maintainers before implementation.
- Keep cross-skill findings consistent across CLI, API and dashboard audit paths;
  allow rule disabling and severity overrides through audit controls. Existing
  analyzer findings do not automatically consume YAML severity overlays: add and
  test explicit analyzer-override plumbing or choose an equivalent rule-registry
  integration. Do not claim that an accepted override already changes analyzer
  severity. Cover listing, disable and severity changes in global/project policy
  and every supported entry point.
- Reconcile these heuristics with shipped lint guidance encouraging trigger
  phrasing and longer descriptions. Include benign examples that satisfy that
  guidance as audit negatives, and document any intentional lint change.
- Initial scope is `SKILL.md` skill metadata. Agent Markdown files use a distinct
  file-scope path and are not covered by these checks or by cross-agent shadowing;
  disclose that limit in output. Extending agent coverage is a separate decision.

`internal/audit/analyzer_metadata.go` handles current metadata checks.
`internal/audit/analyzer_crossskill.go` provides bundle analysis scope, but its
current result-based analysis is not an installed-description index.
Reuse that scope where useful; no prerequisite analyzer is required.

## NVIDIA source and adaptation

The pinned SkillSpector [`_analyze_triggers`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/static_patterns_supply_chain.py#L3368-L3609) examines activation prose and legacy triggers for broad, command-shadowing and universal-scope patterns. The [description-calibrated TR1–TR3 implementation](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/static_patterns_supply_chain.py#L3490-L3608) and [trigger tests](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/unit/test_patterns_new.py#L3284-L3365) illustrate specific cases and negative controls.

This is direct inspiration for trigger-risk categories, not an installed-skill similarity service. Skillshare's target-command inventory, installed-description index, self-update replacement, missing-context reporting and candidate MEDIUM severities require separate policy decisions. NVIDIA's TR1 example uses LOW severity; do not attribute the proposed Skillshare severity or similarity thresholds to it.

## Alternatives Considered

Broad-trigger checks alone miss command and installed-skill shadowing. Exact
name collisions alone miss near-identical descriptions. A remote similarity
service adds disclosure and nondeterminism to a local metadata check.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate includes frontmatter handling, target command inventory, installed-set
context, candidate comparison, reports, configuration and fixtures. Test benign
utility descriptions, stuffing, command matches, near matches, self-updates and
missing comparison context. The design extends existing metadata analysis; implementation follows separately.

## Open Questions

- Which target command inventories should ship first, and who maintains them?
- What similarity and stuffing thresholds keep false positives acceptable?
- Should a missing installed-set context produce an informational diagnostic?
- Should explicitly related skill variants be exemptible through normal rule overrides?
- What index admission and latency limits should apply, and should agent metadata
  and cross-agent shadowing be included in a later scope?
