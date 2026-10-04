# 274 step 4: dashboard `.skillfollow` tab

The dashboard can now show and edit the `.skillfollow` declaration files under
**Settings → Files → `.skillfollow`**. Before this, the dashboard listed followed
skills but had nowhere to see or change the declarations.

## API contract

Both routes use the operation's follow snapshot (`s.skillFollowSet()`) and the
mode's skills source (`s.skillsSource()`), so global and project dashboards each
edit the files their discovery reads. `/api/skillignore` has no global-only
refusal, so these routes have none either.

### `GET /api/skillfollow`

Returns the same entries, warnings, and prune-pause messages that
`status --json` reports under `source.skillfollow`. Undeclared links are left out,
as in status. With no declaration files, `active` is false and the lists are empty.

```json
{
  "base":  { "path": "/home/u/.config/skillshare/skills/.skillfollow", "exists": true,
             "content": "# Team repositories\n_team-skills\n_old-archive\n" },
  "local": { "path": "/home/u/.config/skillshare/skills/.skillfollow.local", "exists": true,
             "content": "_scratch\nbad/name\n" },
  "active": true,
  "local_active": true,
  "entries": [
    { "name": "_team-skills", "state": "followed",
      "resolved_target": "/home/u/work/team-skills", "reason": "following directory" },
    { "name": "_old-archive", "state": "missing",
      "reason": "lstat /home/u/.config/skillshare/skills/_old-archive: no such file or directory" }
  ],
  "warnings": [
    "/home/u/.config/skillshare/skills/.skillfollow.local:2: invalid first-level entry \"bad/name\"",
    "_old-archive: missing: lstat …: no such file or directory"
  ],
  "prune_paused": [
    "prune paused: _old-archive is missing; restore or fix /home/u/.config/skillshare/skills/_old-archive, or remove _old-archive from .skillfollow[.local], to resume cleanup"
  ]
}
```

### `PUT /api/skillfollow`

```json
{ "file": "base" | "local", "content": "...", "delete": false }
```

- Every non-blank, non-`#` line must pass `sourcewalk.ValidEntryName`, the parser's
  own check. The first bad line is rejected with 400 and the file is not touched:
  `{"error": ".skillfollow:2: invalid first-level entry \"../up\"; use one direct child name per line", "error_code": "validation"}`.
- `file` must be `base` or `local` (400 otherwise). `delete: true` requires empty
  content and removes the file. Empty content without `delete` writes an empty file.
- The write goes through `sourcefs.Root.WriteFileAtomic` (temp file and rename
  inside the source root). A linked declaration file is refused rather than
  written through. A failed write leaves the old file and no temp file behind.
- It never creates or removes links, never touches `.gitignore`, and never syncs.
- Each PUT logs an ops entry: `skillfollow` with `{scope: "ui", file, delete}`,
  status `ok` or `error`.
- The response is the GET payload, recomputed after the write.

No `raw_writes.tsv` entry was needed: the handler uses no raw `os` writes.

## UI

The `.skillfollow` file tab sits between `.skillignore` and `.agentignore`
(deep link `/config?tab=skillfollow`).

- A segmented switch chooses `.skillfollow` or `.skillfollow.local`. Each file
  keeps its own unsaved edits and dirty dot. Saving one file does not reload the
  other's edits. Leaving the tab with either file dirty asks first.
- Editor (240 px; the shared Expand dialog still works) with the hint "Names of
  first-level links to follow; one per line; # comments."
- Save calls PUT. Its response replaces the cached GET, so the table updates at
  once. Skills, overview, diff, and doctor queries are invalidated. A 400 appears
  inline above the editor (`role="alert"`) and the edit is kept. This uses a new
  `onError` option on `useEditableFile` in place of the toast.
- An "Entries" table shows name, state (`ss-tag`: `followed` ok, `not-link`
  neutral, other states warn), resolved target, and reason.
- Below the table, one warning note lists the warnings that do not repeat a table
  row (for example rejected declaration lines), followed by the prune-pause
  messages.
- A side panel replaces the ignore-list panel. It explains `.local`, gives the
  `.gitignore` reminder from the reference page (anchored, no trailing slash,
  plus `/.skillfollow.local`), and says that saving does not link, edit
  `.gitignore`, or sync.
- With no declaration files, the editor is empty and the table shows "No entries
  declared."
- Strings are under `config.skillfollow.*` in all 11 dashboard locales. State
  values stay in English.

Screenshots, taken at 1440 px desktop width in zh-TW against the `/tmp/sfdemo`
fixture (one followed group, two missing entries, one invalid local line), are in
the gitignored `.playwright-mcp/` folder of the worktree:

- `.playwright-mcp/sf-clean-light.png`: Clean light, base file
- `.playwright-mcp/sf-clean-dark-local.png`: Clean dark, `.local` selected
- `.playwright-mcp/sf-clean-dark-error.png`: Clean dark, inline 400 after saving `C:drive`

Playful light was checked before the final layout fixes (no-wrap names, wrapping
paths, deduplicated warnings). It was not captured again after them.

## Tests

- `internal/server/handler_skillfollow_api_test.go`:
  - GET: no files, base only, base plus local, rejected entry (`missing`, with a
    warning and a prune pause).
  - PUT: valid write returns states and logs ops; invalid name returns 400 and
    leaves the file unchanged; delete only with the flag; a failed write
    (read-only source) keeps the file, leaves no temp file, and logs an error ops
    entry; a linked file is refused; project mode writes the project source.
  - The container runs as root, so the read-only case skips there. It also ran
    and passed as `nobody` from a compiled test binary.
- `ui/src/pages/ConfigPage.test.tsx`: four `.skillfollow` tab tests (entry states
  and prune pause; empty state; inline 400 with the edit kept; `.local` save with
  `('local', content)` and the returned states shown). The i18n parity tests
  cover the new keys.

Results, in a throwaway devcontainer-image container:

- `make check`: `✓ All tests passed!` (`ok skillshare/tests/integration 210.400s`).
- `pnpm exec vitest run`: 99 files, 877 tests passed. The first full run, made
  while `make check` was running alongside it, had 9 failures. Two reruns passed.
- `pnpm run build` passed.
- `eslint` on the changed files: 11 warnings, all already present at HEAD (line
  numbers shifted), and no new ones.
- Website `pnpm run typecheck` and `pnpm run build` passed for all locales.
- `python3 scripts/ai-context.py check` printed `ok`.

## Docs touched

- `website/docs/reference/commands/ui.md` and the ja, ko, zh-Hans, and zh-Hant
  copies: `.skillfollow` intro sentence, Settings → Files row, and the two API
  rows.
- `website/docs/reference/skillfollow.md` and its four locale copies: Dashboard
  bullet (tab description replaces "no dedicated editor tab yet"); the Limits
  line no longer lists a dashboard editor.
- `skills/skillshare/references/skillfollow.md`: dashboard line about the tab,
  and the editor dropped from future work.

## Follow-ups

- The prune-pause sentence now exists in two places: `cmd/skillshare/skillfollow.go`
  `skillfollowPauses` and `internal/server/handler_skillfollow.go`. The
  `cmd/skillshare` copy was off-limits for this task. A later change could move
  the sentence into `sourcewalk` so both use one copy.
- In project mode, `GET/PUT /api/skillignore` reads `s.cfg.EffectiveSkillsSource()`,
  not `s.skillsSource()`. This looks like an existing inconsistency with
  `.skillfollow`, which uses the project source. It was not changed here.
