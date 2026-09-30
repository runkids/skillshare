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
