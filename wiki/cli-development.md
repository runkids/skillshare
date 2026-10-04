# CLI Development

Use when adding or changing Go CLI commands, flags, handlers, interactive TUIs, domain packages, or any mutating behavior.

## Before Starting

- Search for the closest existing command pattern instead of guessing APIs from memory or documentation.
- List affected command, domain, test, schema, documentation, and Web API files.
- Define acceptance criteria before changing public behavior. Ask only when ambiguity would materially change scope or interfaces.
- Use TDD: create a reproducible failing test, then write the smallest implementation that passes it.

## Command Layering

`cmd/skillshare/<command>.go` should contain only flag parsing, validation, and mode dispatch. When a handler approaches roughly 300 lines or mixes concerns, split it using existing suffixes:

| Suffix | Responsibility |
|---|---|
| `_handlers.go` | Core orchestration |
| `_project.go` | Project-mode behavior |
| `_render.go` / `_format.go` | Output rendering and formatting |
| `_prompt.go` / `_prompt_tui.go` | Decisions and prompts |
| `_tui.go` | Full-screen Bubble Tea UI |
| `_batch.go` | Batch orchestration |
| `_resolve.go` | Target/resource resolution |
| `_context.go` | Mode-specific context |

Put testable core logic in `internal/<domain>/`. Do not let the CLI, Web API, and UI implement separate versions of the same business logic.

## Global and Project Mode

Most commands route through `parseModeArgs()` for global (`-g`) or project (`-p`) mode. Before changing behavior, verify:

- whether the modes use different configuration, registry, source, or target paths;
- whether project mode needs a separate handler;
- whether their flag sets are actually the same;
- whether tests cover mode precedence, working directory, and missing configuration.

## Output and Interaction

- Ask inline questions with `internal/ui/prompt.go` (`Select`, `MultiSelect`, `Confirm`, `Input`, built on `huh` with theme colors); keep full-screen Bubble Tea components for browsing lists. Do not add another prompt framework.
- Full-screen TUIs share the frame in `cmd/skillshare/tui_frame.go`: a title line (`skillshare <cmd> · scope · counts`, tabs on the right), the list and its details side by side without divider lines, and one key line with the position on the right. Show only the common keys there and the rest under `?`; ask confirmations and take filter input on the key line, not on a separate screen. `esc` clears the filter, then goes back, then quits. Action keys are lowercase with one meaning per letter: `d` remove, `u` update, `s` sync, `e` edit settings, `n` new, `i` import, `c` collect, `r` restore, `t` enable/disable, `m` manual only, `o` sort, `!` audit. Uppercase is only for whole-set actions such as `D` empty trash, always with a confirmation. Screenshot every changed TUI with `scripts/screenshots/record-tui.sh`.
- A question asked without a terminal takes its default. Print each decision with the flag that changes it instead of failing or silently doing less.
- Confirm a single action (remove, delete, apply anyway) with `ui.ConfirmAction`: esc answers no, and a decline prints `ui.Cancelled("removed")`. Without a terminal `ui.Confirm` reads one line from stdin (`y`/`yes`, `n`/`no`, anything else is the default), so `echo y | skillshare …` keeps working.
- Print results with `internal/ui/rows.go`: `ui.Row(mark, label, value, ui.RowWidth(labels...))` per item (`✓`/`!`/`✗`, or `ui.MarkNone` for plain information), `ui.Section` for a bold block name, `ui.Done` for the closing line with its duration, `ui.Note` for dim detail and `ui.Next` for up to three follow-up commands. Leave zero counts out. Integration tests match a row with `AssertRowContains(t, label, value)`.
- Preserve dispatch order: structured JSON → TUI when interactive and allowed → empty state → plain text.
- Structured-output stdout must remain machine-readable; progress, spinners, and diagnostics must not contaminate JSON.
- When adding or changing a flag, inspect `--help`, completions, website command documentation, and tests.
- Completions are five hand-written scripts (`cmd/skillshare/completion_{bash,zsh,fish,powershell,nushell}.go`) that do not read the CLI's dispatch, so they drift silently. Any new or renamed command, subcommand, or flag must be added to all five in the same change; the literal-content check in `tests/integration/completion_test.go` should name it. Installed completions are a copy, so users only see changes after rerunning `skillshare completion <shell> --install`.
- Noninteractive automation should use explicit flags. Never treat `--force` as a universal prompt bypass.

## Mutating Behavior

Operations that change configuration, sources, targets, or managed files must:

- follow existing dry-run, backup, rollback, and conflict-handling patterns;
- write an oplog entry to `operations.log` with the operation, status, duration, and necessary arguments;
- send security scan events to the audit log rather than the regular operation log;
- preserve path validation, scope checks, and ownership checks;
- use domain uninstall/remove flows for managed state rather than replacing them with filesystem deletion;
- write the skills source through `internal/sourcefs`, which refuses paths with a link component. The ratchet in `internal/sourcefs/ratchet_test.go` fails on any new raw `os` write call until it gets a truthful reason in `testdata/raw_writes.tsv`.

## Tests

Integration tests use `internal/testutil.NewSandbox(t)` and `RunCLI` or `RunCLIInDir`:

```go
func TestFeature_BasicCase(t *testing.T) {
    sb := testutil.NewSandbox(t)
    defer sb.Cleanup()

    sb.CreateSkill("test-skill", map[string]string{
        "SKILL.md": "---\nname: test-skill\n---\n# Content",
    })

    result := sb.RunCLI("command", "args...")
    result.AssertSuccess(t)
    result.AssertOutputContains(t, "expected output")
}
```

For a bug fix, prove that the test fails before the fix. Add an E2E runbook in `ai_docs/tests/<slug>_runbook.md` for new commands, install/uninstall/sync flows, security behavior, multi-step workflows, or OS/network/permission cases that integration tests cannot cover.

All command execution and isolation rules live in `testing`. Never run the CLI or tests on the host.

## Web API

When a dashboard endpoint is needed:

1. Use existing `writeJSON` and `writeError` helpers in `internal/server/handler_<name>.go`. Decode request bodies with `decodeJSON` or `decodeJSONWith` (`internal/server/json_body.go`), which cap the body size and return 413 when it is too large; do not call `json.NewDecoder(r.Body)` directly.
2. Register the method and route in `internal/server/server.go`.
3. Handle scope explicitly through `s.IsProjectMode()` or the existing guards.
4. Add handler tests and verify UI client types and query invalidation.
5. Share an `internal/` package when the CLI and API expose the same operation; do not shell out between them.

Sync is the main example: the CLI and the server both run per-target sync through `internal/sync` (`SyncSkillTarget`, `RunAgentSync`, `RunExtraTargets`). Both follow CLI semantics: every target runs, and a failed target is reported as a partial failure instead of stopping the loop. Change sync behavior in these runners, not in a command or handler loop.

## Completion Criteria

- The failing test now passes, along with proportionate neighboring tests.
- Handler split, dual-mode behavior, structured output, oplog, Web API, and all five shell completions were reviewed.
- Public behavior changes were synchronized after loading `documentation`.
- Commands were verified inside the devcontainer using `testing` guidance.
