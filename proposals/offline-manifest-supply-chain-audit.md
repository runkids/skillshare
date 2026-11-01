# Feature Proposal: Audit manifest pins, suspicious package names and disabled container trust

Issue: https://github.com/runkids/skillshare/issues/737

## Problem

Existing install-command rules do not parse dependency manifests. Useful
supply-chain checks need ecosystem/version semantics and maintained package
name data, rather than treating every dependency URL or similar name as malicious.

## Proposed Solution

Add an offline manifest analyzer with explicit supported file families.

- Parse requirements*.txt, supported pyproject sections, package.json/lockfiles
  and Dockerfile/compose forms; document which ecosystems and version syntax
  are understood and report unsupported content without a clean-coverage claim.
- Report unpinned dependencies at proposed LOW severity using ecosystem-specific
  exact-pin semantics, including relevant install command forms.
- Compare package names against a pinned, provenance-documented popular-package
  dataset with proposed Damerau-Levenshtein distance<=1. Respect namespaces,
  normalization and legitimate near-name packages; propose MEDIUM, not proof.
- Detect explicit disabled container trust/content verification options at
  proposed HIGH severity, distinguishing them from ordinary unsigned references
  or prose mentioning a flag.
- Collect named manifests/config basenames deliberately under existing bounds;
  do not rely only on currently scannable text extensions.
- Register rule IDs and policy overrides, attach package/file/line evidence and
  surface findings consistently through CLI, API and dashboard reports.
- Keep all checks offline; dataset generation/update review is a maintenance
  process, not a runtime fetch. Parser sharing with OSV lookup is optional reuse.

Relevant source: `internal/audit/rules.yaml`, `internal/audit/audit_scan_skill.go`.

## Alternatives Considered

Command regexes miss manifest declarations. Live package ranking or reputation
lookups add a separate network/privacy contract. A name-distance heuristic
alone cannot establish typosquatting intent.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate 400–750 production and 300–550 test lines plus versioned package
data and its update process; no prerequisite feature.
Test exact/range pins, multiple ecosystem forms, namespaces, legitimate similar
names, trust flags, prose negatives, unsupported formats and stable locations.
No implementation, test result or maintainer approval is claimed.

## Open Questions

- Which ecosystems and manifest versions should ship first?
- Who maintains/licenses the popular-package data and approves updates?
- Which explicit container trust bypasses warrant HIGH severity?
