# Feature Proposal: Inventory external references and bound transitive audits

Issue: https://github.com/runkids/skillshare/issues/715

## Problem

Per-line URL rules do not provide a deduplicated inventory of remote skill
sources or show the uninspected frontier of referenced content. Fetching every
reference automatically would add network and resource exposure.

## Proposed Solution

Keep a useful local inventory and explicitly later transitive traversal visible.

- Propose `audit <skill> --external-refs` to collect references with source
  locations, canonical identities and original spelling preserved as evidence.
- Normalize supported GitHub blob/tree/raw forms, deduplicate locations and
  exclude verifiable self-references without collapsing distinct revisions.
- Emit an `external_references` array in JSON and a human inventory. Listing a
  reference does not fetch it, trust it or imply it is installable.
- Propose opt-in `--transitive`, depth/count/time caps and allow/deny prefixes
  for a later traversal stage. Define exact canonical-prefix comparison and
  redirect/destination policy before any request.
- Fetch to disposable locations through bounded ingestion, audit each admitted
  source and clean up after failure or cancellation; never execute fetched code.
- Report visited nodes, failures and a truncated frontier with reasons, retaining
  source edges and deterministic ordering. Cycles and duplicate refs share work.
- Inventory has no prerequisite. Transitive fetches genuinely require admission,
  byte/path/time limits equivalent to the
  [bounded remote-install proposal](https://github.com/runkids/skillshare/pull/684);
  cancellation and completeness contracts must be agreed, not assumed.

Relevant source: `internal/audit/markdown_links.go`, `internal/install/github_download.go`, `internal/install/gitea_download.go`.

## Alternatives Considered

Existing URL findings remain useful but are not an inventory. Unbounded
recursive installation hides both resource costs and missing coverage.
Inventory-only delivery is useful but does not complete transitive auditing.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Inventory estimate 180–300 production and 150–250 test lines; complete
traversal 650–1,100 production and 450–750 test lines.
Cover canonical forms/revisions, self-refs, duplicates, cycles, redirects,
prefix boundaries, time/count exhaustion and cleanup with fixture servers.
No implementation, test result or maintainer approval is claimed.

## Open Questions

- Which reference forms/revision semantics should canonicalization support?
- What allow/deny and redirect policy should transitive requests obey?
- Which truncated-frontier states must affect exit or installation policy?
