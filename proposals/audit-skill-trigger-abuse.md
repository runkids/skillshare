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
- Keep cross-skill findings consistent across CLI, API and dashboard audit paths;
  allow rule disabling and severity overrides through existing audit controls.

`internal/audit/analyzer_metadata.go` handles current metadata checks.
`internal/audit/analyzer_crossskill.go` provides bundle analysis scope, but its
current result-based analysis is not an installed-description index.
Reuse that scope where useful; no prerequisite analyzer is required.

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
missing comparison context. This proposal contains no trigger analyzer.

## Open Questions

- Which target command inventories should ship first, and who maintains them?
- What similarity and stuffing thresholds keep false positives acceptable?
- Should a missing installed-set context produce an informational diagnostic?
- Should explicitly related skill variants be exemptible through normal rule overrides?
