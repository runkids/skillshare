# Audit tracked updates before publication

## Goal and scope

Implement issue #645 and proposal #646 for `update` (CLI and dashboard) and `install --track --update`. Audit an isolated candidate, then publish exactly its approved commit. Rejection, scan errors, and cancelled confirmation leave the active checkout, target links, and metadata unchanged. Atomic multi-file publication and non-tracked updates are out of scope. No new dependencies.

## Design

Use a temporary local clone with the active checkout's Git configuration, sparse selection, and local changes. Snapshot the permanent audit metadata outside the candidate and reject unresolved or escaping symlinks, so relocation cannot hide integrity findings or change the scanned content. Share existing partial-checkout recovery with candidate publication to retain compatible local edits on publication failure. Before forced publication, snapshot staged and unstaged edits without changing the worktree or stash ref; recover writable paths and the original index independently if publication fails. Both entry points keep their existing pull and dirty-checkout policies, but pull only inside this candidate. Accepted findings remain keyed to the active path. A candidate-mode audit gate rejects without resetting the active checkout. Publication imports the approved local commit without contacting upstream again, checks that the active checkout has not changed, and applies that commit while preserving allowed local edits.

## Implementation and verification

- [x] Add deterministic regressions that inspect published symlinks at the audit boundary, and advance upstream during confirmation to prove exact-revision publication.
- [x] Add shared candidate lifecycle and publication in `internal/install`; integrate `internal/update/tracked.go` and `internal/install/install_tracked.go`.
- [x] Preserve force, skip-audit, dry-run, scan-error, accepted-findings, merge, followed-checkout, and local-edit behavior; test candidate cleanup and publication failure.
- [x] Update tracked-audit error consumers and documentation, including existing translations.
- [x] Run focused tests, the project review skill with `fix`, `python3 scripts/ai-context.py check`, and `make check` in the devcontainer. Report any unverified checks.

## Verification results

- `python3 scripts/ai-context.py check` and `git diff --check`: passed.
- Devcontainer `make check` (documentation checks, formatting, lint, unit and integration tests): passed.
- Non-root devcontainer permission regressions: passed for clean publication, compatible local edits, forced staged/unstaged edits (including edits within the read-only directory), staged deletions/renames, sparse-checkout index flags/exclusions, and incoming symlinks; existing failed-pull recovery tests also passed.
- Five-pass project review and focused re-review: confirmed findings fixed with regression tests. Atomic multi-file publication and native Windows/macOS execution were not tested or added.

## Review focus

Rejected candidates must not trigger a live reset. Confirmation can take arbitrarily long, so upstream advancement must not change publication. Local commits must survive normal merge updates. Allowed tracked local edits and untracked files must survive staging and publication. Followed source links and accepted-finding keys must continue to refer to the permanent destination, never the temporary clone.
