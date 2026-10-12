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

## PR review: configured project backup scope

Project backup access only extends outside the project when a valid folder
memory extra is configured. An external `sources.extras` path alone no longer
exposes backups from its default `memory` subdirectory. The shared scope check
guards listing, versions, preview and restore; in-project access is unchanged.

A regression first reproduced the unconfigured external directory appearing
in the backup list. After the fix, it also verifies both preview routes and
restore return `file_backup_outside_project`, preserve external content, and
continue listing in-project backups. The existing configured-source restore
test and all FileBackups API tests pass. Devcontainer `make check`, context-router
and whitespace checks passed. This increment changes no UI code or screenshots.

## PR review: lock backup authorization through access

Restore now acquires the configuration write lock before checking the requested
path and holds it through the file write. Backup list and preview handlers hold
a read lock through validation and reads. Scope helpers require callers to hold
that lock, preventing memory-extra deletion between authorization and access.

A regression verifies a previously permitted external path is rejected after
the memory extra is deleted, with rejection decided under the restore write
lock and external content preserved. It failed against the earlier unlocked
validation. FileBackups API tests pass with the race detector; devcontainer
`make check`, context-router and whitespace checks pass. No UI code changed.

## 2026-10-12: Memory presence on the dashboard

The dashboard shows configured Memory and up to four recently updated readable
notes, linking directly to the existing note browser. Untouched built-in
starters have a separate introductory state. Counts, file update times and
configured-target counts appear without extra guidance shortcuts; configuration
does not imply an agent has read a note. Updates use disk modification times,
including external edits. All eleven UI locales include the new copy.

The Memory tab keeps its full note count while searching, shows update times,
and stacks the tree above the content when available space is narrow. Existing
editing, recovery and reviewed agent-guidance flows remain in place.

Metadata and initialization recovery tests, 29 scoped UI tests, devcontainer
`make check`, scoped ESLint, UI and documentation-site builds, React Doctor,
context-router and whitespace checks passed. Browser checks covered Clean and
Playful in light and dark modes, narrow layout, and empty/starter states using
synthetic Memory responses without changing actual notes. Requested design
source files were removed after implementation. No commit, push or deployment
was performed.

### Review fix: existing notes with missing starters

The dashboard now shows existing notes even when setup is incomplete because
`INDEX.md` or `LEARNED.md` is missing. The initialization action remains available
in the list footer. A browser check reproduced the hidden-note state before the
fix and verified the note link and initialization action afterward with mocked
responses. Initialization recovery coverage includes both empty and populated
incomplete setups; existing starter-preservation tests also pass.
Devcontainer `make check`, scoped UI tests and ESLint, UI and documentation-site
builds, React Doctor, context-router and whitespace checks passed. The partial
setup was inspected in Playful light at 1280 px and Clean dark at 700 px.

The overview is mounted only for a configured folder memory extra. Unconfigured
memory stays out of the dashboard; initial setup remains in Extras. Missing
starter files after setup still allow opening existing notes and restoring
starters from the overview.

### PR follow-up: bounded overview and preview preference

The dashboard now uses a separate recent-notes endpoint. It counts eligible
Markdown entries from metadata, reads only two starters and up to four recent
candidates, and retains the same hidden-folder and source-link exclusions as
the full list. The API is shared by global and project scopes. Regression
coverage checks total counts, candidate order, starter/incomplete states and
the six-read bound, plus cache invalidation for the overview.

Preview expansion is remembered in browser storage across notes and reloads.
Collapsed previews allow 360 px instead of 230 px. Storage failure keeps the
control usable for the current page.

Configuration, extras and instruction changes invalidate the memory caches so
counts and guidance follow the current source. The three corresponding cache
regressions failed before the fix; a Go overlay that read all candidates also
made the six-read regression fail while leaving its four output rows intact.
Final devcontainer `make check`, 84 scoped UI tests, ESLint, UI and website builds,
React Doctor, context-router and whitespace checks passed. Browser assertions
confirmed expansion and collapse survive reloads, expansion follows note
selection, collapsed height is 360 px, and the dashboard uses the recent endpoint
without requesting the full notes list. React Doctor retains one maintainability
warning about overview component complexity.


## 2026-10-12: Memory feedback, trial and organization

Both guidance modes now ask for decision/lesson dates, reasons and available
sources or evidence, distinguish assumptions, and report only successful writes
with the saved content, project/shared scope and file path. Existing intact
blocks become reviewable updates; user-modified blocks and real notes are not
silently changed. The editor also shows a provenance hint.

**Memory tools** in the existing agent-guidance section copies a three-step
cross-session trial: append a temporary INDEX.md value, retrieve it in a fresh
conversation, and remove only that line. The retrieval prompt contains the
current source but never the test value. Values survive dialog reopening and
page reloads per source in browser session storage; unavailable storage falls
back to the open page. The organization prompt reviews accessible indexed
notes for duplicates, contradictions, outdated claims, broken links and missing
provenance, and asks for path/evidence/before-after proposals before approval.
It also reviews current conversation/tool evidence and user-provided session
logs for reusable lessons, checks existing docs/guardrails before suggesting
new ones, and prioritizes automatic checks for mechanical errors. Approved
writes stay in the memory source; project checks/guidance need separate consent.
Dashboard reuses the same dialog and its recent response carries the resolved
root. Missing roots disable the trigger. Both global and project root
regressions failed before the response included the path.
The previous standalone verification-copy action is folded into the trial's
read step, leaving one entry point for verification and organization. Skillshare
does not execute these prompts or claim agent-read telemetry.

Verification passed devcontainer `make check`, 24 scoped UI tests, full ESLint
(with existing warnings), scoped ESLint, TypeScript/UI build and all five
website locale builds. All eleven dashboard locales include the tools and
editor hint. React Doctor changed-scope comparison against HEAD, including the
untracked new component, found no new issues (91/100); the full-repository scan
still reports pre-existing findings. Context-router and whitespace checks pass.
Browser checks with synthetic notes verified the copied read prompt excludes
the value, save/cleanup values match, reload persistence, approval-first
organization, denied-storage fallback and editor guidance. Both tool views
were inspected in Clean/Playful light/dark at 1280x900 and Playful dark at
390x844, including scrolling to cleanup without dialog horizontal overflow.
No real notes or tool guidance files were modified. A real fresh-agent trial
and Windows browser behavior were not exercised.


Follow-up verification for the retro prompt and dashboard trigger passed
`make check`, 26 scoped UI tests, scoped ESLint, UI build and all five website
locale builds. React Doctor reports no new issues (91/100). API regressions
verify the recent response's root in global and project scopes without extra
note reads. Live preview checks verified both entry points resolve the source,
trial values match and stay out of the read prompt, organization copy succeeds,
and missing-root responses disable the trigger. Clean/Playful light/dark were
inspected at 1280x900; Clean dark at 390x844 has no dialog horizontal overflow.
The preview backend was rebuilt to include the new response field.

## 2026-10-12: Host-owned retrospectives and Dreaming

The built-in Skillshare skill now routes session retrospectives and Dreaming to
`references/memory-review.md`. Retrospectives use the current conversation and
explicitly supplied evidence. Dreaming can compare saved lessons with provenance
from multiple sessions; private history scans, a transcript archive, and new
journals are unnecessary. Reviews propose exact paths and edits before approval,
check the resolved project/shared source, preserve intervening changes, and use
versioned CLI writes with backups. Scheduled host reviews return proposals only.
Ordinary passive/active saves retain their existing authorization. Updated
reading guidance suggests useful review opportunities once without starting one.

The shared dashboard/Memory dialog now has three inline modes with concise steps,
clear copy actions, and complete scoped prompts folded by default. Expanded
prompts scroll within a bounded area. All eleven UI locales and all five website
locales describe the workflows. The former standalone verification-copy action
is documented as part of the cross-session trial.

Verification passed devcontainer `make check`, 30 scoped UI/i18n tests, scoped
ESLint, TypeScript/UI build, and all five website builds. React Doctor's changed
scope including untracked files reports no issues (91/100). Isolated CLI checks
confirmed local skill installation and Claude/Codex synchronization include the
new reference, global/project roots match, stdin writes work, stale updates
preserve saved content, and both guidance modes retain their write policy.
Browser checks covered both entry points, Clean/Playful light/dark, desktop and
390 px layouts. Context-router and whitespace checks passed. The live preview
uses an isolated demo source and the rebuilt backend. No real user notes or tool
guidance were changed, and no commit, push, or deployment was performed.
Fresh-agent adherence to the review skill and Windows behavior were not tested.

### Review fixes: confirmed scope and consistent write safeguards

The shared Memory tools trigger now waits for confirmed overview metadata;
loading, failed requests, and project responses without a project directory
cannot expose a copied prompt with a guessed global scope. It reuses the existing
overview query cache for both entry points. Regression tests exercise the real
AppProvider and pending, failed, and incomplete overview responses.

All eleven retrospective prompt translations now require version-checked CLI
writes with backups, a matching resolved source, and stopping on unavailable CLI
or conflicts. They also require reporting partial failures, matching the Dreaming
prompt and built-in skill. All five website locales describe these safeguards.

Verification passed devcontainer `make check`, 34 scoped UI/i18n tests, scoped
ESLint, TypeScript/UI build, and all five website builds. React Doctor reports no
issues in the changed scope including untracked files (91/100). The live preview
confirmed the resolved scope and localized safeguards in the full prompt.
Context-router and whitespace checks passed. Actual agent writes and Windows
behavior were not tested; no commit or push was performed.
