# Feature Proposal: Non-interactive status filtering for `list`

Issue: [#244](https://github.com/runkids/skillshare/issues/244)

## Problem

The interactive `list` TUI can filter enabled and disabled entries with the `s` key or a `status:` search tag. That behavior was requested in #172 and shipped in v0.20.0 via #174. The plain-text and JSON paths do not expose the same filter.

On v0.20.21, an isolated source with one enabled skill and one skill disabled through `.skillignore` produces:

```console
$ skillshare status --json | jq '.skill_count'
1
$ skillshare list --json | jq 'length'
2
$ skillshare list 's:enabled' --json | jq 'length'
0
```

`status.skill_count` counts enabled skills, while `list` returns the full inventory and marks disabled entries. Both views are useful, but scripts have no direct way to request one status from `list`. The TUI tag is treated as an ordinary name pattern outside the TUI, which makes the last command return an empty array.

Callers can filter the JSON with `jq`, but they need to know that `disabled` is omitted for enabled entries. Plain-text callers can parse the `[disabled]` label, but there is no native or structured filter.

## Proposed Solution

Add a long-form status filter:

```console
skillshare list --status all
skillshare list --status enabled
skillshare list --status disabled
```

The proposed behavior is:

- `all` is the default. Running `skillshare list` without the flag keeps its current output.
- Plain-text and JSON modes return only discovered entries matching the selected status.
- In the TUI, the flag sets the initial `Status:` filter. Users can still press `s` to cycle through all three states.
- The flag works in global and project mode, for `list`, `list agents`, and `list --all`.
- Status combines with the positional pattern and `--type` using AND semantics.
- `--status value` and `--status=value` are accepted case-insensitively. Missing or invalid values return an error that lists the accepted values. No short flag is added because `-s` already means `--sort`.
- A status-only plain-text query with no matches reports `No <resources> matching status "<value>"`; JSON returns `[]`.

The TUI should continue loading both enabled and disabled entries after applying the existing pattern and type filters, then initialize its status filter. Applying status destructively during loading would make it impossible for the existing `s` key to restore entries. The CLI status filter and a `status:` search tag continue to compose with AND semantics, so conflicting values produce an empty view.

The JSON output remains a top-level array of the existing item objects. This proposal does not add an envelope or change the current `disabled` field behavior. All five shell completions should advertise `--status`; backends that already support option-value completion should also offer `all`, `enabled`, and `disabled`.

The command help and list documentation should state that the default `all` view contains the entries returned by current discovery, including entries marked disabled.

No new dependencies are needed.

## Alternatives Considered

### Keep JSON filtering in callers

Callers can use `jq 'map(select((.disabled // false) == false))'`. This does not help plain-text callers, and every script must learn the omitted-field detail.

### Interpret `status:` tags outside the TUI

This would reuse the TUI search syntax, but positional arguments currently mean name, path, or source patterns. Adding a long flag is easier to discover, validate, and complete in shells.

### List enabled entries by default

Changing the default would hide disabled inventory and break scripts that rely on the current array length or contents. Keeping `all` as the default preserves compatibility.

### Replace the JSON array with an object containing counts

An object such as `{ "items": [...], "enabled": 1, "disabled": 1 }` could make counts explicit, but it would break existing consumers. Aggregate count design can be discussed separately.

## Compatibility and Non-goals

- The default plain-text and JSON output stays unchanged.
- Explicit `--status all` should behave like omitting the flag.
- This proposal does not change `.skillignore` or `.agentignore` discovery and disabled-marking semantics. It filters the `Disabled` state on entries already returned by `list`.
- This proposal does not redefine `status.skill_count`.
- Usage tracking, unused-skill detection, cleanup automation, and legacy tracked-repository migration are out of scope.

## Scope

Estimate the scope of changes:

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Expected areas:

- `list` option parsing and shared filtering
- Global and project list paths
- Initial TUI status state
- Integration tests for skills, agents, project mode, plain text, and JSON
- Parser and TUI tests covering `--status=value`, invalid values, explicit `all`, and cycling away from the initial status
- CLI help, shell completions, command documentation, and the bundled Skillshare skill

The large classification comes from file count: completions and documentation are stored separately. The behavioral code remains localized to the list command and should stay below 500 meaningful lines.

## Open Questions

- Should a non-`all` status filter hide the tracked-repositories summary, matching the current behavior of pattern and type filters? The proposed default is yes because the existing summary is built from status-unfiltered discovered skills. Explicit `--status all` would keep it visible.
- Should the plain-text footer include both the matched count and the existing pre-filter inventory denominator, such as `1 of 2 skills (status: enabled)`? The proposed default is yes, preserving the current denominator used by pattern and type filters.
- Should explicit `--status all` count as an active filter for footer and summary rendering? The proposed default is no, so it remains equivalent to omitting the flag.
