# Feature Proposal: Classify bundled bytecode and disguised executables by file headers

Issue: https://github.com/runkids/skillshare/issues/722

## Problem

The skill walker filters by size and extension before reading content. Its
binary check looks for a NUL byte in the first 512 bytes and contributes to
aggregate analyzability reporting. That does not identify executable formats,
disguised scripts or Python bytecode under skipped or misleading names.

## Proposed Solution

Inspect a bounded header before ordinary text size/extension filtering, with
explicit hidden-file and `__pycache__` coverage inside the audited skill.

- Read at most 512 bytes per eligible file and classify ELF, Mach-O, PE, Python
  bytecode, ZIP/JAR and WASM signatures. Validate enough structure to distinguish
  PE from a file that merely starts with MZ; report uncertain/truncated headers.
- Recognize shebang scripts under document, image, JSON or hidden names. Classify
  extension/signature mismatches without treating every executable bit as proof.
- Propose HIGH for executables disguised as documents/images or hidden payloads,
  MEDIUM for other executable artifacts and HIGH for Python bytecode. Treat
  ZIP/JAR as archive classification rather than proof of executable behavior.
- Use a pinned, maintained Python magic-number table with documented supported
  versions. Report unknown bytecode-like headers without claiming a full decode.
- Traverse the required hidden content and cache paths deliberately, retaining
  `.git`, internal-metadata, depth and source-boundary exclusions. Do not follow
  new external links just to inspect headers.
- Keep binary/skipped counts separate from auditable text bytes and show them in
  CLI, JSON and dashboard summaries. A classified binary has not been analyzed
  for its runtime behavior, and an oversized file has only had its header read.

The collection and current NUL check are in `internal/audit/audit_scan_skill.go`;
`internal/audit/audit_result.go` defines current analyzability reporting.
No prerequisite feature is required. Shared collector changes are merge risks.

## Alternatives Considered

An extension denylist misses renamed payloads. NUL detection cannot identify
formats or non-NUL shebang scripts. Full disassembly, archive-member inspection
and execution analysis are separate later work, not guarantees of header sniffing.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines)
- [ ] Large (10+ files, 500+ lines)

Estimate includes header classification, deliberate hidden/cache traversal,
summary consumers, Python data maintenance and synthetic byte fixtures. Cover
renamed binaries, legitimate archives, shebang mismatches, large files, truncated
headers and known/unknown bytecode. No executable scanner is implemented here.

## Open Questions

- Which document/image extensions and hidden executable cases should count as disguise?
- Which Python versions should be recognized, and how are magic updates reviewed?
- Should ordinary bundled native executables be MEDIUM by default or informational?
- What total entry/header-read budget should complement existing depth limits?
