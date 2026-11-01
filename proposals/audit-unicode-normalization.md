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
  evidence that the normalized view matched. Keep existing Markdown context and
  rule overrides consistent across both views.
- Skip view construction for pure ASCII where equivalent. Document the Unicode
  version, table provenance, generation process and update/review procedure.
- Add a separately disableable mixed-script identifier heuristic. Use explicit
  script exceptions and conservative severity rather than flagging multilingual
  prose as an attack merely because it contains several scripts.
- Validate Cyrillic-confusable instruction text, soft-hyphen splits, compatibility
  forms, legitimate multilingual text and stable locations. Target less than 15%
  scan slowdown on a documented representative benchmark; this is an acceptance
  target to measure, not a demonstrated performance claim.

Integration points are `internal/audit/audit_scan_content.go`,
`internal/audit/audit_scan_skill.go` and `internal/audit/dedupe.go`.
`go.mod` already declares `golang.org/x/text`; generated confusable data is new.

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
