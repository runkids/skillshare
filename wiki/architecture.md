# Repository Architecture

Use when deciding which layer should own a change, tracing CLI or Web API data flow, or building an initial mental model of the codebase.

## Product Boundaries

skillshare's source of truth is either the global configuration directory (`~/.config/skillshare/` by default on macOS/Linux) or project-local `.skillshare/` state. The CLI transforms or synchronizes skills, agents, extras, MCP servers, hooks, and plugins into each AI tool's native locations. Never infer runtime behavior from README files or documentation alone; verify the current implementation and embedded configuration.

## Repository Map

| Path | Responsibility |
|---|---|
| `cmd/skillshare/` | CLI entry point, flag parsing, mode routing, TUIs, and command orchestration |
| `internal/config/` | Global/project configuration, registry, migrations, and target resolution; `targets.yaml` defines built-in targets |
| `internal/<domain>/` | Domain logic such as install, sync, audit, MCP, hooks, plugins, and backup |
| `internal/server/` | Dashboard HTTP API; routes are registered in `server.go` and handlers live in `handler_*.go` |
| `internal/testutil/` | Shared isolated `Sandbox` and CLI runner for integration tests |
| `tests/integration/` | CLI integration tests |
| `ui/` | React 19 and Vite dashboard; its API client corresponds to `internal/server/` |
| `website/` | Docusaurus public documentation and marketing pages |
| `ai_docs/tests/` | Reproducible CLI E2E runbooks |
| `skills/skillshare/` | Built-in skill for skillshare users, not internal development instructions |
| `schemas/` | Public YAML/JSON schemas; inspect these when configuration shapes change |
| `scripts/`, `Makefile`, `mise.toml` | Build, test, devcontainer, and verification entry points |

## Request Flow

A typical CLI request follows this path:

1. The `commands` map in `cmd/skillshare/main.go` dispatches the command.
2. `cmd/skillshare/<command>.go` parses flags and selects global or project mode.
3. `cmd/skillshare/<command>_*.go` composes domain operations, prompts, and output.
4. `internal/<domain>/` performs filesystem, Git, configuration, or audit logic.
5. Mutating operations write to `operations.log` through `internal/oplog`; security scans use the audit log.

A typical dashboard request follows this path:

1. `ui/src/api/client.ts` sends the request.
2. A route registered in `internal/server/server.go` enters `handler_<domain>.go`.
3. The handler uses the same `internal/` domain package as the CLI. Do not duplicate core behavior in the frontend or handler.
4. Project/global differences use the server mode and existing helpers rather than a separate data model.

## Shared Memory Notes

`internal/memory` owns note discovery, bounded UTF-8 reads, content-version
writes/deletes/moves, backups, index inspection/link append, and scope/hash-marked
guidance. CLI `extras memory` and dashboard handlers share its store and existing
extra source resolution. Initialization preserves notes and creates missing
`INDEX.md`/`LEARNED.md`; the source-only extra requires no targets. Dashboard
initialization requires both readable starters and the configured extra; note
creation repairs missing starters before using the refreshed index. Unsupported
notes remain listed and do not block valid notes. Explicit dashboard index actions
append at EOF with version checks/backups; failed linking preserves a created
note. Broken links are reported without rewriting user-owned index sections.
Prepared writes and deletions recheck the version after backup immediately before
replacement/removal; new notes use exclusive creation to reject concurrent creates.

`handler_memory_guidance.go` resolves existing target read chains and instruction
assignments. Plan returns per-file before/after content, skips, reader/size
warnings, and a token; apply recomputes it before backed-up writes and syncing
shared copies. Each file is checked against its reviewed content and existence
before writing, including after backup. A later conflict preserves that file and
reports `memory_guidance_stale` in the partial result alongside applied paths.
New guidance files use exclusive creation; a competing creator also produces
`memory_guidance_stale` without overwriting its file.
It preserves other content, assignments, and connection modes.
Each block records its update mode (`passive`, the default and unmarked, or
`active`) in its begin marker, and status is checked against that mode's text.
Plan and apply take a mode per target; a target left out keeps its block's mode.
Targets reading one file share its block, so different modes for one file are
rejected. A target whose live chain holds blocks of both modes is
broken/`mixed_modes`; switching a target that reads blocks in several files is
skipped as `multiple_blocks`, since rewriting one file would leave the others.
Intact outdated blocks can be updated after review; modified/malformed blocks
are protected. Non-UTF-8 instruction files are reported as broken/unsupported
and skipped without rewriting their bytes; instruction files have no new size
cap (known character limits remain review warnings). Guidance paths inside a project are relative to the project root;
external sources remain absolute. GET reports file-based configuration state,
never agent reads. Fresh-session read events provide manual verification only.

The Memory tab uses the shared tree/Markdown editor. Conflicts retain drafts,
display latest saved content, and require confirmation before a version-checked,
backed-up replacement. History/restore links filter Backup Files by absolute
note path, and a requested path without backups shows an empty state.
Windows history links compare normalized separators while backup API calls retain
the native path; POSIX names retain literal backslashes. Project
backup scope includes its configured memory source even outside the repository;
without a configured memory extra, external backup access remains denied.
Backup handlers hold the configuration lock from scope validation through reads
or restore writes, so removing a memory extra cannot invalidate an active check.
Initialization failures, including partial file creation, are logged.
Dashboard move/rename preserves note content and permissions, creates
missing parent folders, rejects stale versions and existing destinations, and
backs up the source path. Links and path-keyed history remain at their original
paths; index inspection reports broken links after a move. Native automatic
memory, telemetry, and Obsidian integration are outside this boundary.

## Sources of Truth

- Command flags and behavior: `cmd/skillshare/*.go`.
- Target names, aliases, and default paths: `internal/config/targets.yaml`.
- Built-in audit rules: `internal/audit/rules.yaml` and related table-driven analyzers.
- Public configuration contracts: `schemas/*.json` and configuration parsing/validation code.
- Dashboard design values: `ui/src/components.css` and `ui/src/index.css`.
- Website design values: `website/src/css/custom.css` and page CSS modules.
- Build and test commands: `Makefile`, `mise.toml`, and CI workflows.

## Trace Checklist

Before changing code:

1. Search for the existing symbol, flag, or route and find its definition, callers, and tests.
2. Determine whether both global and project mode are affected.
3. If the CLI and Web API expose the same behavior, verify that they share domain logic.
4. For state changes, inspect backup, rollback, oplog, dry-run, and audit implications.
5. Load `documentation` for public contract changes, `cli-development` for implementation, and `testing` before execution.
