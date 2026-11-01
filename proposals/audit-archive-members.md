# Feature Proposal: Inspect ZIP and Office container members during audit

Issue: https://github.com/runkids/skillshare/issues/702

## Problem

Skills can bundle scripts inside ZIP archives, wheels and Office documents.
The file filters in `internal/audit/audit_scan_skill.go` do not inspect those
containers. A scan of visible text does not establish what their members contain.

## Proposed Solution

Add bounded ZIP-container inspection with virtual member paths, without
extracting member files to disk or executing their contents.

- Discover eligible containers by ZIP signatures within an explicit bounded
  read policy, including `.zip`, `.whl`, `.docx`, `.xlsx` and `.pptx` inputs.
  Do not rely on the current text-extension filter or filename alone.
- Use Go's ZIP reader and validate the archive's structure. Magic bytes select
  a parser; they do not prove a valid ZIP or an Office document.
- Identify supported Office packages from their structure. Scan text members
  through the existing content rules and attribute findings to virtual paths
  such as `outer.zip!/scripts/setup.sh`, including nested containers.
- Propose shared per-skill ceilings of depth 3, 1,000 members, 25 MiB of
  expanded content, 1 MiB per member, expansion ratio 100:1 and 5 seconds.
  Also cap admitted compressed input at 25 MiB; define exact units in policy.
- Account against actual bytes read across nested archives, not only declared
  sizes/counts. Stop before further allocation or decompression exceeds a cap.
  Apply cooperative deadline checks to parsing and member reads.
- Reject absolute/traversal member names and symlink members. Report encrypted,
  corrupt, unsupported or over-budget content as uninspected with a reason.
- Propose a HIGH concealed-executable finding for executable/script content in
  document containers. Ordinary archive packaging alone is not malicious;
  Office macros and arbitrary binary code need an explicit supported policy.

Integrate through `internal/audit/audit_scan_skill.go`,
`internal/audit/audit_scan_content.go` and `internal/audit/audit_result.go`.
The archive feature must own its limits and inspection status. A future global
scan budget can coordinate accounting, but is not a prerequisite.

## Alternatives Considered

Filesystem extraction creates cleanup and path-handling obligations without
helping text inspection. Extension-only detection misses renamed archives.
Unbounded in-memory decompression still permits resource exhaustion.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines), including collection and result consumers

Estimated implementation: 450–800 production lines plus generated fixtures for
nested archives, Office packages, forged metadata, encrypted/corrupt members,
unsafe names and expansion limits. Test actual accounting and partial results.
This proposal includes no parser implementation or claim of complete binary analysis.

## Open Questions

- Approve the proposed ceilings and their configuration/override policy?
- Which document member types justify HIGH severity, including macros?
- Should partially inspected archives block installation by default?
- How should self-extracting ZIPs and unsupported compression be reported?
