# AGENTS.md — skillshare

skillshare is a Go CLI with a React dashboard and a Docusaurus documentation site. It synchronizes skills, agents, extras, MCP servers, and plugins across AI coding tools. This file contains only rules needed for every task; load task-specific details through the context topics below.

## Language

- Instruction files, internal documentation, public English documentation, code, identifiers, comments, and commit messages use English.
- Reply to users in their language; default to Traditional Chinese when the user writes Chinese.
- `README.md` is the public English source. Keep translated READMEs aligned with its existing localization conventions. Preserve history files in their original language.

## Always Applies

- **Use executable evidence.** Verify facts that affect conclusions or implementation against current source, configuration, installed versions, or actual command output. Distinguish facts, assumptions, and uncertainty.
- **Preserve user changes and keep scope minimal.** Inspect the working tree before editing, then trace relevant definitions, callers, tests, and existing patterns. Never overwrite unrelated changes or perform opportunistic cleanup.
- **Keep the docs true.** When documentation disagrees with code, verify the code and fix the stale documentation in the same task. Update a topic whenever a change alters behavior it describes. Put completed milestone logs in `wiki/history/` and add them to the index. Run `python3 scripts/ai-context.py check` after documentation changes. The maintenance procedure is in the `ai-context` topic.
- **Run commands in the correct environment.** Execute the skillshare CLI, Go builds/tests, UI tooling, and website tooling inside the devcontainer. The host is only for file edits, read-only searches, Git inspection, and the context-router check. Load `testing` before execution or debugging.
- **Verify in proportion to risk.** Start with the narrowest check that proves the change. Reproduce bugs and add regression tests when practical. Never skip, disable, or weaken validation to obtain a passing result.
- **Respect authorization boundaries.** Reviews, investigations, and diagnoses are read-only by default. Do not commit, push, publish, deploy, install global tools, or perform destructive or irreversible actions unless explicitly authorized.
- **Use Git safely.** Never overwrite user changes. When asked to commit, stage only task-owned files and follow repository conventions; otherwise use an English Conventional Commit with a body explaining why.
- **Keep helper files in the repository.** Put reusable helper scripts in `scripts/<area>/` and test runbooks in `ai_docs/tests/`. Do not store persistent task artifacts outside the repository.
- **Report outcomes precisely.** State what changed, why, actual verification results, limitations, and preserved unrelated changes. Explicitly identify checks that were not run.

## Hard Limits

- Never run `ss`, `skillshare`, `go test`, `make test`, or frontend package scripts on the host.
- Never expose, hardcode, or commit secrets, tokens, credentials, or private user-path contents.
- Never bypass audits, hooks, tests, trust prompts, or native client confirmation.
- Never change permissions, delete persistent data, rewrite Git history, commit, tag, push, or release without explicit authorization.
- Resolve and confirm the exact target before a destructive action. Never recursively delete a workspace root, home directory, or unresolved broad path.

## Load Context Per Task

```sh
python3 scripts/ai-context.py list      # list topics and selection guidance
python3 scripts/ai-context.py <topic>   # print only that topic's sources
python3 scripts/ai-context.py check     # validate paths, headings, orphans, and byte budgets
```

The router runs on the host and needs only Python 3.8+. On Windows, use `py` or `python` when `python3` is not available.

| Topic | Use when |
|---|---|
| `architecture` | Tracing repository structure, data flow, or CLI/Web API boundaries |
| `cli-development` | Adding or changing Go CLI commands, flags, handlers, TUIs, or mutating behavior |
| `testing` | Building, testing, reproducing bugs, using the devcontainer/ssenv, or running E2E runbooks |
| `frontend` | Changing React, CSS, layout, or visual behavior in `ui/` or `website/` |
| `documentation` | Updating README files, website docs, command flags, translations, or the built-in skill |
| `release` | Writing changelogs or release notes, bumping versions, tagging, or releasing |
| `audit` | Read-only consistency checks for flags, docs, tests, targets, handler splits, oplog, or Web API |
| `pull-requests` | Opening a pull request or handling its review comments, including Codex review |
| `ai-context` | Adding, moving, or splitting wiki topics, or fixing router checks |

Pick the single closest topic by default. Load a second only when a task genuinely crosses two seams. If no topic fits, read `wiki/README.md`.

To add a topic:

1. Put its content in `wiki/`.
2. Map it in `wiki/ai-context.json`.
3. Add it to this table and `wiki/README.md`.
4. Run `check`.

Split an over-budget topic instead of raising its cap. Skills are workflow adapters that load topics; they are not the sole source of repository rules.

## Verification

```sh
python3 scripts/ai-context.py check

# Code changes: run inside the devcontainer after loading `testing`
make check
```
