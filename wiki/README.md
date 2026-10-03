# skillshare Wiki Router

The wiki stores development background, procedures, references, and history, but no task needs all of it at once. Rules that apply to every task live in the root `AGENTS.md`. `wiki/ai-context.json` is the only source of truth for topic-to-source mappings.

```sh
python3 scripts/ai-context.py list
python3 scripts/ai-context.py <topic>
```

## Task Routing

| Topic | Use when | Main sources |
|---|---|---|
| `architecture` | Tracing repository structure, data flow, or CLI/Web API boundaries | `architecture.md` |
| `cli-development` | Adding or changing Go CLI commands, flags, handlers, TUIs, or mutating behavior | `cli-development.md` |
| `testing` | Building, testing, reproducing bugs, using the devcontainer/ssenv, or running E2E runbooks | `testing.md`, `CONTRIBUTING.md` |
| `frontend` | Changing React, CSS, layout, or visual behavior in the dashboard or website | `frontend.md`, `website/AGENTS.md` |
| `documentation` | Updating README files, website docs, flags, translations, or the built-in skill | `documentation.md`, `CONTRIBUTING.md` |
| `release` | Writing changelogs or release notes, bumping versions, tagging, or releasing | `release.md` |
| `audit` | Running read-only consistency audits | `audit.md` |
| `pull-requests` | Opening a pull request or handling its review comments, including Codex review | `pull-requests.md` |
| `ai-context` | Maintaining router topics, budgets, and orphan checks | `ai-context.md` |

Run `python3 scripts/ai-context.py check` after changing topics. This table is the human entry point; the JSON configuration controls what the loader renders.

## History (Milestone Logs; Read on Demand)

Completed milestones are recorded here and read only when relevant.

| File | Contents |
|---|---|
| [Shared memory notes](history/extras-memory.md) | Shared Markdown source, CLI/API/dashboard, versioned edits and native memory boundaries |
| [Git hooks management](history/git-hooks-management.md) | Config-mode destinations, includes, ownership, native verification and execution limits |
| [Native hooks management](history/hooks-management.md) | Native formats, CLI/dashboard scope, ownership and verification evidence |
| [MCP: Pi built-in only and tool policy](history/mcp-pi-builtin-tool-policy.md) | 0.23.0 Pi migration, per-Agent tool policy translation, evidence and limits |
