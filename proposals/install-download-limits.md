# Feature Proposal: Bound remote installation work

## Problem

Remote installation should have a predictable resource ceiling before candidate
content is published. The GitHub client already limits each HTTP request to
15 seconds, but recursive downloads do not share a byte, entry, depth or total
time budget. The API download path copies response bodies without a size cap.

## Proposed Solution

Add one ingestion budget shared by the requests and files belonging to an
installation. Reuse the existing download clients, name validation, staging
and cleanup paths rather than adding another downloader.

- Start with proposed defaults of 100 MiB of file content, 10,000 entries,
  depth 64, path length 4,096 bytes and 60 seconds overall. Keep the existing
  per-request timeout. Expose limits through an `install` configuration section.
- Enforce API-response and file limits while reading, including bodies without
  a trustworthy Content-Length. Count directories and files before descending
  or creating more entries, with one shared budget across recursive requests.
- Apply the elapsed deadline to supported remote fetch operations. Inspect a
  cloned working tree before copying or publishing it, without following links,
  and reject content that exceeds the admitted tree limits.
- Return typed limit errors identifying the exhausted budget. Remove only the
  operation's own partial staging content and leave existing installations intact.
- Propagate the same configuration and failures through CLI and dashboard
  installation paths. Document the distinction between transfer and tree limits.

Relevant source is `internal/install/github_download.go`,
`internal/install/gitea_download.go`, the Git staging paths in `internal/install`,
and `internal/github/client.go`. Symlink policy is a separate concern; this
proposal reuses existing path checks and does not define a new link policy.

## Alternatives Considered

A limit on one file body leaves recursive entry counts and aggregate work
unbounded. Per-request timeouts already exist and do not bound a sequence of
requests. Rejecting only after copying consumes the resources the check is
intended to protect.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines), potentially larger with all consumers
- [ ] Large (10+ files, 500+ lines)

Implementation would include configuration/schema documentation, shared budget
accounting, forge/Git integration and fixture-only tests. Tests should cover
exact boundaries, a streaming body, recursive aggregate exhaustion, excessive
depth, deadline cancellation, typed errors and cleanup that preserves an
existing installation. No implementation or release change is included here.

## Open Questions

- Are the proposed defaults suitable for large legitimate skill repositories?
- Should configuration permit disabling a limit, or only raising its ceiling?
- For Git, is an admitted working-tree cap sufficient? That is not a hard cap
  on packfile transfer bytes or clone-time disk use. A strict transport/storage
  guarantee needs an additional design before being advertised.
- Should GitHub and Gitea metadata response bytes share the content allowance
  or have a separate, smaller ceiling?
