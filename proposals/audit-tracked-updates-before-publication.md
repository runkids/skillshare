# Feature Proposal: Audit tracked updates before publication

## Status

**Implemented in [#647](https://github.com/runkids/skillshare/pull/647)** (merged 2026-10-26).

- Candidates are staged and audited in a private temporary clone, shared by both tracked update entry points.
- Approval publishes the exact audited commit into the existing checkout. Atomic multi-file publication is not guaranteed and remains outside this proposal's scope.

The sections below preserve the original proposal, pre-fix behavior, and design questions.

## Problem

Issue: https://github.com/runkids/skillshare/issues/645

Before #647, including in v0.26.4, tracked repositories were exposed to consumers via symlinks into their active checkout. Updating a tracked repository pulled upstream changes directly into the active checkout before running the audit gate (`internal/update/tracked.go`: `git.PullWithProgress` followed by `auditPulled` -> `install.AuditGate.Run`). If audit detected security findings above threshold and rejected the update, `AuditGate.block` reset the working tree (`gitResetHard`).

This created a visibility window: concurrent readers could read rejected candidate files before the reset occurred. While successful rollback restored the pre-update commit, it could not revoke data already read.

Reproduction:
1. Create a local Git repo with two skills and commit them.
2. Install with `skillshare install /path/to/repo --track --name trial -g` and `skillshare sync -g`.
3. In the upstream repo, commit a `SKILL.md` change containing an audit violation (e.g. instruction to exfiltrate tokens).
4. Run `skillshare update _trial -g --json` while concurrently reading the installed target `SKILL.md`.
5. The unapproved candidate text is visible to the reader until audit fails and rollback completes.

Observed scope: observed file visibility during concurrent read in v0.26.4; does not demonstrate agent execution or scanner evasion.

## Proposed Solution

Audit update candidates in an isolated staging location before modifying the active checkout. The active checkout and published symlinks must remain untouched until the candidate passes audit.

- **Internal changes**:
  - In `internal/update/tracked.go` (shared by CLI and server update handlers), fetch candidate revisions outside the published checkout.
  - Apply the same boundary to `install --track --update`, whose `internal/install/install_tracked.go:updateTrackedRepo` also pulls before calling `auditTrackedRepoUpdate`; reuse a shared mechanism where practical.
  - Run `install.AuditGate` against the candidate tree.
  - On audit rejection, error, or cancelled confirmation: discard candidate; leave active checkout and published targets untouched.
  - On approval / override: publish the exact audited commit to the active checkout.
- **Interactions with existing features**:
  - Preserves dirty-checkout checks, `--force`, `--skip-audit`, audit thresholds, and accepted-findings recording.
  - Interactive confirmation inspects the staged candidate tree without mutating published paths.
  - Dry-run behavior remains side-effect free.
- **Dependencies**: No new dependencies.

### Acceptance Criteria

- **Old bytes preserved on rejection**: While candidate audit runs, installed target links expose only pre-update bytes. On rejection, active checkout commit, file contents, and metadata remain unchanged.
- **Exact audited commit published**: On clean or overridden audit, the exact revision that was scanned is published to the active checkout, and symlinks resolve to the updated content.
- **Every tracked update entry point**: Exercise both `update` (CLI/server) and `install --track --update` against these criteria.

## Alternatives Considered

- **Status quo (pull then reset)**: Restores state after rejection, but cannot prevent reads during the audit window.
- **Audit candidate then pull into active checkout**: Introduces a race where a second pull may fetch a commit different from the audited one.
- **Remove symlinks during update**: Breaks tool availability and causes transient errors for consumers.
- **Operational freeze (pause readers)**: Cannot be enforced across independent processes or agents.
- **Change local-path edit policy**: PR #622/#623 explicitly scopes local non-git installs; tracked git updates are a distinct mechanism.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines)
- [ ] Large (10+ files, 500+ lines)

*Original scope estimate. This PR provides only the proposal per CONTRIBUTING.md; implementation was completed separately in #647.*

## Original Open Questions

1. Which candidate staging mechanism is preferred (e.g. detached `git worktree`, temporary clone, or in-repo object fetch)?
2. Should publication guarantee atomic multi-file switch for readers (e.g. symlink swap), or is in-place checkout update after audit sufficient?
3. How should interactive prompts and CLI reporting format candidate file paths without exposing ephemeral staging directories?
