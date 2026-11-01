# Feature Proposal: Audit poisoned skill and tool metadata by field

Issue: https://github.com/runkids/skillshare/issues/728

## Problem

Current metadata checks cover publisher mismatch and authority language.
Identifiers, free-text descriptions and schema defaults have different meanings;
scanning arbitrary adjacent JSON fields would create noise and missed coverage.

## Proposed Solution

Extend metadata analysis using the general frontmatter parser and an explicit
set of supported adjacent tool-schema locations.

- Check skill/tool identifiers for invalid forms, invisibles, mixed scripts and
  confusables using documented conservative script exceptions.
- Inspect name, description, trigger text and parameter descriptions in their
  field context; detect override language and hidden-comment instructions.
  Propose HIGH for override phrases in a skill description, subject to approval.
- Decode candidate base64 text of at least 50 characters within strict
  input/output/depth limits; inspect valid UTF-8 results, preserve evidence and
  never execute decoded content. Long benign encodings need negative fixtures.
- Flag parameter descriptions longer than 500 characters and defaults
  containing nonlocal destinations or executable curl/wget/bash-c commands,
  only where the supported schema actually gives them default-value meaning.
- Register stable IDs with normal listing, disable and severity controls;
  attach field/JSON path and original source locations through all reports.
- Distinguish unsupported/malformed schema coverage from an inspected clean
  field; redact secrets before snippets or parser diagnostics escape.
- Reuse a maintained confusable helper if available, but this feature is not
  dependent on a separate normalized full-content scan.

Relevant source: `internal/audit/analyzer_metadata.go`, `internal/utils/frontmatter.go`.

## NVIDIA source and adaptation

The pinned SkillSpector [`_extract_metadata_texts`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/mcp_tool_poisoning.py#L195-L230) separates names, descriptions, triggers and parameter fields. [`_check_tp1`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/mcp_tool_poisoning.py#L388-L425) validates decoded base64 as UTF-8; its [description fixture](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/test_mcp_tool_poisoning.py#L421-L438) exercises encoded metadata. [`_check_tp3`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/mcp_tool_poisoning.py#L738-L780) reports a parameter-description override as MEDIUM.

These field-aware checks directly inspire the proposed analysis, but supported Skillshare schema paths, bounded decoding, original locations, secret redaction and severity policy need independent decisions. In particular, the proposed HIGH finding for override phrases in a skill description is not NVIDIA's MEDIUM parameter-description classification. The 50-character candidate floor and greater-than-500-character parameter threshold above are proposed Skillshare requirements subject to approval, not proof of matching behavior for every NVIDIA field.

## Alternatives Considered

Applying one text rule to every field loses identifier/default semantics.
Unrestricted base64 recursion can waste resources. A confusable warning alone
misses free-text and schema-default poisoning.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate 300–550 production and 250–450 test lines plus maintained Unicode
data if required; supported schemas and collection are part of the scope.
Cover clean multilingual identifiers, invisible/confusable cases, bounded
decoding, field severity, defaults, unsupported schemas and report redaction.
No implementation, test result or maintainer approval is claimed.

## Open Questions

- Which tool-schema paths and versions should be recognized initially?
- Which field-specific thresholds and severities avoid routine metadata noise?
- Should schema parse gaps affect strict completeness policy?
