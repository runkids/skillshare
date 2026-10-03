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

The Extras Memory tab uses a collapsible folder tree and a Preview/Raw pane,
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
Rename/move, native automatic memory, and Obsidian integration remain unimplemented.

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
