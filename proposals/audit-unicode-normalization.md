# Feature Proposal: Detect audit patterns in a normalized Unicode view

Issue: https://github.com/runkids/skillshare/issues/699

## Problem

Static matching uses source text, while hidden-Unicode rules flag selected
characters separately. Confusable letters or ignorable characters can split a
phrase without making the dangerous intent obvious to raw-text matching.
Normalization also changes positions, so findings must still identify real source.

## Proposed Solution

Scan raw text and a separate security view for injection, exfiltration,
destructive-command and shell patterns. Never rewrite installed content.

- Apply NFKC, a versioned UTS #39 confusable skeleton and an explicit
  default-ignorable policy. NFKC alone is not confusable detection.
- Preserve an offset map through expansion, contraction and removal so snippets
  and locations refer to the original bytes and lines, including combining marks.
- Deduplicate raw/normalized matches for the same rule and source span, retaining
  evidence that the normalized view matched. Preserve the original-source
  snippet and other existing key inputs on the surviving finding so `AcceptKey`
  and `Fingerprint` remain stable when the same finding is also matched through
  the second view. Test raw-only versus dual-view acceptance and fingerprint
  stability, including existing acknowledgements. Carry view provenance in a
  separate finding metadata field, subject to schema review, rather than changing
  accepted-key inputs. Define JSON, SARIF and dashboard representation and keep
  original snippets and locations in each consumer.
- Keep Markdown context and rule overrides consistent across both views. Evaluate
  a rule's exclude expression against the same view as its positive match; test
  raw and normalized exclusions independently and after deduplication.
- Skip view construction for pure ASCII where equivalent. Document the Unicode
  version, table provenance, generation process and update/review procedure. Pin
  compatible normalization and skeleton table versions together, or document and
  justify any version skew and review their updates as a coupled change.
- Add a separately disableable mixed-script identifier heuristic. Use explicit
  script exceptions and conservative severity rather than flagging multilingual
  prose as an attack merely because it contains several scripts.
- Validate Cyrillic-confusable instruction text, soft-hyphen splits, compatibility
  forms, legitimate multilingual text and stable locations. Maintain a reviewed
  known-good multilingual fixture corpus with explicit expected findings and no
  unexpected high-severity findings; test the separately controlled mixed-script
  heuristic on identifier-like tokens and normal multilingual prose. Target less
  than 15% scan slowdown on a documented representative benchmark that includes
  non-ASCII and multilingual normalization/offset work as well as the ASCII fast
  path; extend `scan_benchmark_test.go` or add a named companion benchmark. This is
  an acceptance target to measure, not a demonstrated performance claim.

Specify the normalization unit and order before implementation. One candidate is
to remove approved ignorables before NFKC on bounded text segments, then apply the
skeleton; test composition across a removed ignorable with exact original offsets.
The pinned NVIDIA code normalizes per code point, so a segment-based approach
would be a deliberate difference, not an upstream guarantee. Script-sensitive
exceptions and segment boundaries remain maintainer decisions.

Integration points are `internal/audit/audit_scan_content.go`,
`internal/audit/audit_scan_skill.go` and `internal/audit/dedupe.go`.
Decide explicitly whether `internal/audit/tiers.go` command classification and
`internal/audit/dataflow.go` shell analysis also consume the security view; if
deferred, report that coverage limit and test the raw-only behavior rather than
implying those analyzers are protected. Evidence schema and consumers include the
finding model, JSON, SARIF and dashboard.
`go.mod` already declares `golang.org/x/text`; generated confusable data is new.

## NVIDIA source and adaptation

The pinned SkillSpector [`normalized_security_view`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/artifacts.py#L1882-L1910) builds an NFKC/UTS #39 ASCII-skeleton view, skips selected ignorable spans and records source offsets. Its [ASCII fast-path tests](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/analyzers/test_security_character_folding.py#L27-L48) check equivalent character handling without normalization.

This directly inspires a derived security view. Skillshare's byte/line mapping through expansion, contraction and removal, matching-rule selection, evidence deduplication, Unicode data license/version and mixed-script policy remain separate design decisions. The less-than-15% slowdown is a proposed Skillshare acceptance target, not an NVIDIA measurement.

## Alternatives Considered

More hidden-character rules identify obfuscation but do not recover the phrase.
NFKC without skeleton mapping misses cross-script confusables. Replacing original
text in reports would conceal the actual source and invalidate locations.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate includes matching consumers, mapped locations, generated data,
mixed-script policy, documentation and benchmarks. Handwritten logic may be
smaller, but maintained Unicode data and coverage are part of the feature.
No normalization implementation or benchmark result is included here.

## Open Questions

- Which Unicode version and source/license should the generated table pin?
- Which default-ignorable characters should be removed in which script contexts?
- Should normalized-only matches inherit severity or start at reduced confidence?
- Which mixed-script identifiers warrant findings without overwhelming normal skills?
- Which normalization boundaries and ignorable-removal order are appropriate for
  each supported script, and should tier/dataflow normalization ship in this scope?
