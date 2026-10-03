# Shared Memory Notes

The `memory` folder extra stores user-owned Markdown notes. Initialization
creates missing `INDEX.md` and `LEARNED.md` templates, preserves existing files,
and registers no targets. Global/project source
overrides use the existing extras resolution.

`internal/memory` is shared by the CLI `extras memory` commands and dashboard
`/api/extras/memory/` routes. Reads are UTF-8 and limited to 1 MiB; nested links,
hidden paths and traversal are excluded. Writes require the last-read content
hash, back up changed notes and reject stale edits. Deletion also requires the
last-read hash and backs up the saved file before removal.

The Extras Memory tab uses a collapsible folder tree and a Preview/Source pane,
following the Skill detail file browser. It supports nested note creation,
search, editing, deletion with confirmation, relative note links, and copyable
instructions pointing to the canonical source. Instruction files remain user-managed outside the marked reading-guidance block.
Native automatic memory, transcript ingestion, automatic index rewriting and
graph visualization are outside this increment.

Regression coverage lives in `internal/memory`, `internal/server`,
`tests/integration/extras_memory_test.go` and `MemoryNotes.test.tsx`.
The reproducible workflow is `ai_docs/tests/extras_memory_runbook.md`.

The original English sharing-memory guide included nine screenshots captured with the
dashboard locale set to English. CLI/API deletion, backup restoration, and
global/project behavior passed `make check`; the runbook passed all five steps.
The Memory UI tests, TypeScript, ESLint, and production build passed. All five
website locale builds passed. Clean/Playful light/dark views were inspected.

## Connection and note workflow improvements

The dashboard now connects selected tools through **Connect to agents → Review
changes → Apply changes**. Plan/apply resolves each existing read chain and
writes a scope/hash-marked guidance block to the existing instruction file or
shared source. All other content, assignments, and connection modes are retained.
Existing files are backed up; changed plans require a fresh review. Intact
outdated blocks are reviewable updates; user-modified or malformed blocks are
preserved. Unsynced and unreadable instruction files are skipped. Non-UTF-8
instruction files are broken/unsupported and skipped with their bytes preserved.
Instruction files have no new size cap; known character limits are warnings.
Configuration status is based on files, not a claim that an agent read them.
Verification uses a fresh session, an actual file read event, the full note path,
and a temporary user-added value. There is no guaranteed read telemetry.

Project guidance uses paths relative to the project root for in-repo sources,
including when instructions live in subfolders. External overrides stay absolute.
CLI `instructions` prints this same block without adding connection flags.

Note creation offers explicit **Link from INDEX.md**, checked by default when
its index is readable. It appends at EOF with a version check and backup; failure
leaves the new note intact. **Add to INDEX** links existing unindexed notes and
broken links produce warnings. Unsupported notes stay visible without blocking
normal notes. A conflict retains the editor draft and shows latest saved content;
confirmed replacement uses the refreshed version and a backup. History and
post-deletion restore open Backup Files filtered to the absolute note path.
Native automatic memory and Obsidian integration remain unimplemented.

The tutorial is now localized in all five website locales, with English-only UI,
note, and dialog screenshots. Connection review and configured-state screenshots
are `memory-connect-demo.png` and `memory-connected-demo.png`; conflict and
recovery use `memory-conflict-demo.png` and `memory-restore-demo.png`. Manual copying is
an advanced fallback. Memory UI text covers all 11 dashboard locales.

Verification for this documentation/localization update: the devcontainer website
build passed for `en`, `ja`, `ko`, `zh-Hans`, and `zh-Hant` with exit code 0 and no
broken links. Host context-router and diff whitespace checks passed. All 67
Memory/backup keys and interpolation names match across 11 dashboard locales;
static Memory UI keys and dynamic connection/detail/skip keys are present.
All 13 tutorial screenshot references and relative page links resolve. Screenshot
production and application tests belong to the coordinator's implementation work.


Final implementation validation passed `make build` and `make check`, 22 scoped
UI tests, TypeScript, ESLint, and the production UI build. The isolated CLI
runbook passed all six steps, including unsupported-note isolation. A real
Claude writer created a random marker and decision in a project fixture; a fresh
Codex session retrieved both through explicit AGENTS.md → INDEX.md → note reads.
That smoke test verifies explicit retrieval, not automatic native loading.
Both copy buttons preview their full content on hover/focus. The English tutorial
now includes `memory-verification-demo.png`, and the empty/starter screenshots
were refreshed to match the new connection controls.


## Dashboard redesign and move/rename

The page header now owns **New note**. The viewport-height note card keeps search
and the folder tree on the left, and a fixed **Preview** / **Source** header with
icon actions above separately scrolling note content. Connection controls use a
compact agent side rail; both copy actions retain hover/focus previews.

**Move or rename** accepts a new relative Markdown path through
`POST /api/extras/memory/move`. It creates missing folders, preserves content and
permissions, requires the last-read version, and exclusively creates the
new destination so existing notes cannot be replaced. The source is backed up
before removal. Dashboard success selects the new path, clears the previous
search, refreshes index warnings and exposes history for the old path. Markdown
links and path-keyed history are not rewritten. Keep the root `INDEX.md` in place
because reading guidance references it. CLI commands and flags are unchanged.

Verification passed `make build`, `make check`, 39 scoped UI tests followed by
14 updated MemoryNotes tests including stale moves, TypeScript, scoped ESLint,
and the production UI build. Domain/API regressions cover renaming, nested moves,
permissions, source backups, existing destinations, unsafe paths, stale versions,
and both global/project scopes. Visible Chrome checks verified rename, cross-folder
move, collision rejection, selected destination and cleared search. A fresh demo
home completed Create memory → Connect → Review → Apply. All 14 previous
screenshots were recaptured with English UI, notes and dialogs; the new move
screenshot brings the total to 15. Tutorial and reference changes cover all five
website locales, whose production builds passed. All 74 Memory keys and their
interpolations match across 11 dashboard locales. React Doctor against HEAD
reports 86/100 and no errors, with four component-complexity warnings; the prior
HEAD comparison also scored 86/100. Build warnings about existing bundle size,
Browserslist data and Rspack configuration remain outside this increment.


## PR review: final conflict checks

Prepared note writes and deletions now re-read the saved version after backup,
immediately before replacement or removal. Missing notes conflict with an
existing-note version. New drafts use exclusive creation so competing creates
cannot overwrite the first saved note. Guidance apply checks each reviewed
file's content and existence, including after backup, and reports
`memory_guidance_stale` for conflicts in its partial results while retaining
already-applied paths. The built-in skill reference now documents dashboard
move/rename instead of claiming it is unavailable.

Regression tests first reproduced stale prepared writes, post-backup deletion,
and edits to later guidance files during a multi-file apply. They now pass,
along with a concurrent-create test, both affected package suites, the scoped
race suite, and devcontainer `make check`. Context-router and diff whitespace
checks passed. UI code and screenshots were not changed by these review fixes.


## PR review: initialization and recovery

Dashboard initialization now requires the configured extra and readable
`INDEX.md`/`LEARNED.md` files, so an empty or removed source can be initialized
again. Creating a note initializes missing starters and fetches the new index
version before appending the optional link. Init logs both success and errors,
including partial starter creation and configuration-save failures.

History honors an explicitly requested path with no backups instead of
selecting another file. Project backup list, preview and restore include the
configured memory root resolved through `sources.extras` outside the repository;
unrelated sibling paths remain excluded. Per-extra project `source` paths still
follow their existing in-project validation.

Regression tests reproduced all three review findings before the fixes and now
pass. Devcontainer `make check`, 17 scoped UI tests, scoped ESLint, TypeScript and
the UI production build passed. React Doctor on the same two production files
scores 90/100 versus 89/100 on the prior HEAD, with the same three existing
warnings. English empty-source and missing-history views were inspected in
Clean/Playful light/dark at 1440 by 900. An isolated browser workflow initialized
an already-configured empty source and opened a new note's History while a
separate note had a backup; no unrelated restore action appeared.

## PR review: exclusive guidance creation and Windows history

New guidance files now use exclusive creation after the final review check.
If another process creates the destination in that interval, apply reports
`memory_guidance_stale` and preserves the competing file. Existing instruction
file writes retain their reviewed-content checks and backups.

History selection normalizes separators for Windows drive and UNC paths,
including mixed-separator links from the Memory browser and recovery actions.
Backup API calls retain the selected native path; POSIX names with literal
backslashes remain distinct.

Regression tests reproduced both findings before the fixes. Guidance tests,
20 scoped UI tests, devcontainer `make check`, scoped ESLint, TypeScript and
the UI production build passed. React Doctor on FileBackups scores 90/100
versus 89/100 at the reviewed HEAD, with the same two existing accessibility
warnings. No visual layout or screenshot changed. Windows path matching was
tested in the UI suite inside the Linux devcontainer, not a Windows browser.
