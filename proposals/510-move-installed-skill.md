# Feature Proposal: Move an installed skill into another folder

Issue: [#510](https://github.com/runkids/skillshare/issues/510). Related: #247 (second half asks for the same), #108 (folder view; move was a non-goal there).
Depends on: PR #513 (`fix/install-missing-moved-skill`), assumed merged first.

## Problem

Install records live in one `.metadata.json` keyed by source-relative path, so moving an installed skill with `mv` strands its record at the old key (`doctor`: "N entries ... missing on disk"; dashboard: "Install missing"). `website/docs/how-to/daily-tasks/organizing-skills.md` now tells users to `uninstall` + `install --into`, which re-downloads, drops audit acceptances, and in project mode rewrites the lockfile pin. Dashboard "into folder" in `InstallDialog` is free text, so users mistype or cannot see existing folders.

PR #513 makes reconcile follow a plain `mv` by hash matching (`MetadataStore.MovedEntryKey`, `MoveEntry`). Its own "Not covered" list is this feature's reason to exist: records without `file_hashes`, skills edited after the move, and no way to do it from the dashboard. Hash inference is a recovery path; an explicit move should not depend on it.

## Proposed Solution

One core operation, exposed as `skillshare move` and `POST /api/resources/batch/move`, plus two dashboard entry points. A move takes skills or whole folders; the verb is `move` because every existing command is a full word (`mv`, `rename` would be the odd ones out).

### Core: `internal/skillmove` (new, mirrors `internal/uninstall`)

`Plan(discovered, names, dest, Options) []Planned` resolves and refuses without touching disk. `Run([]Planned, Options) Outcome` applies, with per-item results like `uninstall.Run`.

A name that resolves to a skill moves that skill, plus any skills nested below its directory (discovery keeps walking after a `SKILL.md`, so `foo/SKILL.md` and `foo/sub/SKILL.md` are both discovered; the rename carries both, so both need their records moved). A name that is a folder under the source moves the folder with every skill under it, keeping its base name: `move frontend archive` gives `archive/frontend/...`. `Plan` expands either into one `Planned` per discovered skill below it and refuses every one of them before any rename; a single refusal aborts the whole folder, so a folder never ends half-moved. Because discovery only sees skills, `Plan` also walks the whole directory tree of a skill or folder for `_`-prefixed tracked checkouts (even one with no discoverable skill) and followed links, and refuses on the first. The folder is then renamed once (`Root.Rename`), so empty subfolders and non-skill files move along; the per-skill steps below (store, lock, `.skillignore`, `.gitignore`) run for each skill, and the rename happens once per requested name.

Per skill, in this order:

1. `sourcefs.MkdirAllIn(source, dest, follow)` (as `handler_install.go` does for `--into`), then `Root.Rename(from, to)` (`internal/sourcefs/sourcefs.go`), with `from` the skill or the folder. Rename already refuses a link component and a rename across a followed link.
2. If the skill has a record: `store.MoveEntry(oldKey, to)` from PR #513. `MoveEntry` re-keys the entry and its `AuditAccepted` list but leaves `Group` to the caller. Do not copy reconcile's group snippet: save the store, then run the existing reconcile (`config.ReconcileGlobalSkills` / `ReconcileProjectSkills`), which sets `Group`, prunes, and in project mode rewrites `config.yaml` skills and calls `WriteProjectLock`.
3. Project mode, before that reconcile: carry the lock pin. `writeProjectLock` never moves a pin, so without this the pin would be re-created from the installed commit, losing a deliberately older pin. New `Lock.MovePin(old, new)` in `internal/install/lock.go`, saved with `install.LoadLock(projectdir.Resolve(root))`; `pruneProjectLock` in `cmd/skillshare/uninstall_project.go` is the precedent.
4. `.skillignore`: if a literal line equals the old relPath, `skillignore.RemovePattern` + `AddPattern` (`internal/skillignore/write.go`) with the new one. Glob rules are left alone; re-run `sync.DiscoverSourceSkillsAll` and warn when `Disabled` flipped (as `toggleOverrideError` does).
5. Project `.gitignore`: swap the old entry for the new one via `config.ProjectGitignoreTarget` + `install.RemoveFromGitIgnoreBatch` (reconcile's `onFound` only adds). Whether project uninstall removes skill lines today is unverified; check before coding.

The reconcile walk stops at an installed parent (`reconcile_core.go`: `existing.Source != ""` returns `SkipDir`) and `pruneStaleEntries` then drops every record absent from that walk, so a re-keyed nested record under an installed parent would be pruned with its audit acceptances. Step 2 therefore also changes reconcile to keep records whose key lies below a live installed record (own test in `internal/config`), rather than bypassing the prune.

Order is rename first, store second: a failed rename changes nothing; a failed store save after a rename is healed by #513's reconcile when hashes exist, and reported otherwise.

`MetadataStore` has no exported "key for relPath" (`GetByPath` returns the entry only), so add `KeyForPath(relPath) string` matching `KeyToRelPath(key, entry)`; needed for legacy basename keys that carry a `Group`.

Refusals (stable `Code`, shared by CLI and API):

| Code | Case |
|---|---|
| `skill_not_found`, `ambiguous_name` | resolved like `resolveUninstallTarget` (CLI) / `resolveUninstallSkill` (server): flat name, relPath, or unique basename; a name matching no skill falls back to a folder relPath, and a folder with no skill below it is `skill_not_found` |
| `dest_exists` | `<dest>/<base>` exists (skill or folder). Never overwritten or merged, `--force` does not change it |
| `inside_tracked_repo` | skill is under a `_`-prefixed git checkout (`install.IsTrackedCheckout`); `update` pulls into it. Same rule as `uninstall.checkMoveOut` (`ErrInsideRepo`). A folder that is or contains a tracked checkout is refused as a whole |
| `dest_inside_tracked_repo` | any ancestor of `dest` is a tracked checkout |
| `linked_folder` | skill is, or sits below, a followed source link (`sourcewalk.Follow.Resolve`), or `dest` is below one. The folder is the user's checkout. A folder that contains a link is refused as a whole |
| `dest_is_skill` | `dest` or an ancestor holds `SKILL.md` (guard; discovery behaviour for nested skills unverified) |
| `invalid_dest` | not `validate.IntoPath`; `.` means the source root. Segments starting with `_` fail `validate.SkillName`, so tracked-repo names cannot be typed |
| `dest_inside_source_folder` | moving a folder into itself or one of its descendants |
| `duplicate_dest` | two names in one batch resolve to the same destination path (`frontend/foo` and `backend/foo` into `archive`). Both are refused before any rename; `--force` does not change it |
| `overlapping_sources` | one requested root equals or sits below another (`move foo foo/sub archive`). Refused before any rename; the parent already carries the child |
| `same_folder` | already there; no-op, exit 0 |
| `name_collision` | forceable, below |

Empty folders and non-skill files inside a moved folder are not refusal reasons; they move along with it.

Extract the tracked-repo ancestor loop from `uninstall.checkMoveOut` into an exported helper in `internal/install` (next to `IsTrackedCheckout`) so both packages share it.

Name collisions: target names are `FlatName` (`grp__demo`) or SKILL.md names, depending on `target_naming`. For each target run `sync.ResolveTargetSkillsForTarget` on the discovered list with the moved skills rewritten, and report `Collisions` that include a moved path. Identical destination paths are `duplicate_dest`, checked first, and never reach this step. The `flat` branch of that function returns before collision detection (verified, `internal/sync/target_naming.go`), so also compare `FlatName` duplicates directly. A collision does not lose data (sync skips both entries), so `--force` accepts it. That is the only thing `--force` does.

Target filters are not rewritten. `include`/`exclude` match flat names (`internal/sync/filter.go`), so a rule naming `demo` stops matching `grp__demo`. Preflight evaluates each target's effective filter (`ShouldSyncFlatName`) for the old and the new flat name and warns whenever the result differs in either direction (a rule naming `demo` stops matching; an `exclude: archive__*` newly drops it; an `include` newly adds it); `sync` already warns on unmatched includes (`UnmatchedInclude.Warning`).

### CLI: `skillshare move <skill|folder>... <dest-folder>`

`rg '"mv"|"move"|"rename"' cmd/` finds only TUI key hints ("↑↓ move"), no command. `move` is free in the `commands` map in `cmd/skillshare/main.go`. The existing verb pair `link`/`unlink`, `enable`/`disable` has no conflict.

- Last positional is the destination, relative to the skills source; `.` moves to the root. At least two positionals. A folder argument moves under the destination (`frontend archive` → `archive/frontend`); a trailing `/` is accepted.
- Flags: `--dry-run`/`-n`, `--force`/`-f` (collision only), `--json`, `-p`/`-g` via `parseModeArgs`, `--help`. No `--kind`: agents are out of scope (see below).
- Handler split: `move.go` (parse, mode), `move_handlers.go` (resolve, render), project differences only where `config.LoadProject` paths differ, as `uninstall_project.go`.
- Output with `ui.Row(ui.MarkOK, name, "→ grp/demo")`, `ui.Done`, then `ui.Next("skillshare sync", "rename the links in your targets")`. `--dry-run` uses `ui.DryRun()` and prints the same rows.
- `--json`: `{"moved":[{"name","from","to","record":bool,"skills":n}],"failed":[{"name","code","error"}],"skipped":0,"warnings":[],"dry_run":false,"duration":"..."}`, shaped like `uninstallJSONOutput`; `skills` is 1 for a skill and the count below a folder. Non-zero exit when `failed` is non-empty.
- oplog: `oplog.NewEntry("move", status, d)`, args `names`, `dest`, `force`, `mode`; written with `oplog.Write(cfgPath, oplog.OpsFile, e)` as `enable.go` does. No command allow-list was found in `cmd/skillshare/log*.go` (unverified for the UI log filters).
- Completions: all five `completion_{bash,zsh,fish,powershell,nushell}.go` plus a literal in `tests/integration/completion_test.go`.

Sync is **not** automatic. `install`, `uninstall`, `enable`, `disable` and `link` all print `ui.Next("skillshare sync")` (`cmd/skillshare/install_handlers.go`, `enable.go`, `link.go`); sync touches every target and can refuse (symlink conflicts need `--force`, a different decision from move's). Until sync runs, the old target link is dangling, as after `uninstall`; the dashboard result view offers Sync now.

### Web API

`POST /api/resources/batch/move`, registered next to `batch/targets` and `batch/toggle` in `internal/server/server.go`; handler in new `internal/server/handler_move.go` (split follows `handler_toggle.go`, `handler_skills_batch.go`).

```json
// request
{ "names": ["demo", "frontend/other", "archive-me"], "dest": "grp", "force": false, "dryRun": false }
// 200 response (partial failure is still 200, like /api/uninstall/batch)
{ "results": [{ "name": "demo", "success": true, "from": "demo", "to": "grp/demo",
                "flatName": "grp__demo", "record": true },
              { "name": "x", "success": false, "error": "...", "error_code": "dest_exists" }],
  "summary": { "succeeded": 1, "failed": 1 }, "warnings": [], "dryRun": false }
```

- `names` entries may be skills or folder relPaths; a folder result carries `skills: n` and fails or succeeds as a whole.
- `decodeJSON(..., defaultJSONBodyLimit)`; hold `s.mu.Lock()` for the whole request like `handleUninstallSkill`; `s.skillsSource()`, `s.skillsWalk().Follow`, `s.skillsStore`, then `s.reconcileSkillsConfig` (`handler_install.go`) which already branches on `IsProjectMode()`. The lock pin and `.gitignore` steps use `s.projectRoot`, `s.gitignoreDir()`, `s.projectGitignorePrefix()`.
- Request-level errors use `writeCodedError`: 400 `invalid_body`, `invalid_dest`, `unsupported_kind` (a `kind` of `agent`). Per-item codes are the table above, in `error_code`; the UI maps them to i18n text (frontend rule: do not show backend English).
- `s.writeOpsLog("move", ok|partial|error, start, {"names","dest","count","scope":"ui"}, firstErr)`, as `handleBatchUninstall`.
- The API does not sync. The dashboard calls `POST /api/sync` or opens `SyncPreviewModal`.

### Dashboard

No new folder endpoint: `GET /api/resources` already returns `relPath`, `linkName`, `isInRepo`, `repoPath` per skill.

**(a) Move to folder…**
- `ui/src/api/resources.ts`: `moveResources(names, dest, opts)`; types in `ui/src/api/types/resources.ts`.
- `ui/src/components/resources/MoveDialog.tsx`: modelled on `UninstallDialog.tsx` (`DialogShell`, `Button`, result list, `syncReminder` note, `SyncPreviewModal`) and `LinkFolderDialog.tsx` (form). States: form → running → result. A `name_collision` result shows a "Move anyway" retry like `retryForce`.
- Entry points: `menuItems` in `ui/src/pages/ResourceDetailPage.tsx` (next to `uninstall`, icon `FolderInput`), `extraItems` in the `ResourcesPage.tsx` context menu, and a button beside Uninstall in the selection bar (`setUninstalling(selectedItems)` is the pattern). Hidden for agents, `isInRepo`, and `sourceLinkOf(skill)`. The `ResourcesPage.tsx` folder context menu also gets "Move to folder…" for a folder row (hidden for folders at or under a tracked repo or link; a folder that contains one is refused by the server with the codes above, shown in the result list).
- After success on the detail page, navigate to `resourceHref` of the returned `flatName`: the route is keyed by flat name, which changes.
- New event `skillsMoved` in `ui/src/lib/queryEvents.ts` (`skills.all`, `overview`, `syncMatrixAll`, `diff()`) and a row in `queryEvents.test.ts`; pages call `invalidate(queryClient, 'skillsMoved')`, no key literals.
- No `ConfirmDialog`: the move is reversible by moving back.

**(b) InstallDialog "into folder"**
- `ui/src/components/FolderPicker.tsx` (shared): `Select` (`ui/src/components/Select.tsx`; `SelectOption.group`/`note` heading) listing "Root", existing folders, then "New folder…" which reveals an `Input` (`ui/src/components/Input.tsx`). Validates against the same rules as `validate.IntoPath` for fast feedback; the server stays authoritative.
- `ui/src/components/InstallDialog.tsx`: replace the `Field label={t('install.url.into')}` input (the `into` state, ~line 783) with `FolderPicker`; `InstallDialog` already has `useSkillsQuery()` for the list. The `into` string and `api.install({ into })` payload are unchanged, so this part has no backend dependency. For `kind === 'agent'` the dialog writes under the agents dir, so the folder list must come from agent resources (how `useSkillsQuery` scopes `kind` is unverified).
- `MoveDialog` uses the same picker with the skill's current folder disabled; for a folder move, the folder itself and its descendants are disabled (`dest_inside_source_folder`).

### Which folders exist

Computed client-side by a pure function in `ui/src/lib/moveFolders.ts`: every ancestor prefix of every skill `relPath` (not only the first segment), via `buildTree` + `folderPaths` in `ui/src/components/resources/tree.ts`, minus:

- any path with a `_`-prefixed segment, or at or under a skill's `repoPath` (tracked repos, including `org/_team` installed with `--into org`);
- any path at or under a `linkName` (followed source links);
- any folder that is itself a skill.

Metadata `group` is not used: it exists only on records with a source (local skills have none) and can be stale until reconcile. Folders with no skills are not listed ("New folder…" covers them; moving the last skill out leaves an empty folder, as plain `mv` would). The server re-validates every destination.

### Docs and skill

- New `website/docs/reference/commands/move.md` (frontmatter `sidebar_position`, usage, flags table, examples, mermaid like `enable.md`); add to `website/sidebars.ts` ("Skill Management", after `enable`), `website/docs/reference/commands/index.md` (category row and table), and `website/src/data/featureMap.ts` (entry plus `COMMAND_COUNT` 37 → 38).
- `website/docs/how-to/daily-tasks/organizing-skills.md`: replace the "Do not move an installed skill with `mv`" warning (commit 189de5a2) with `skillshare move`, and keep `mv` for self-created skills. That commit also touched 4 `website/i18n/*` copies; follow the same convention.
- `website/docs/reference/commands/list.md` (line ~254 says "manual `mv` + `sync`"); `website/docs/reference/commands/sync.md` if it lists producers of renames.
- Built-in skill: `skills/skillshare/SKILL.md` and `skills/skillshare/references/install.md` (add `skillshare move`). Do not edit `CHANGELOG.md` by hand (assumed release-please generated; unverified).
- `README.md` has no per-command table (`uninstall` has no hit), so it needs no change.
- Run `python3 scripts/ai-context.py check` after docs.

## Alternatives Considered

- **Rely on #513 only.** Recovers a plain `mv` when hashes match, but not an edited skill, a record without `file_hashes`, the lock pin, or the dashboard.
- **`install --into` + `uninstall` as a move.** Today's documented workaround: re-downloads, loses audit acceptances and pins, impossible offline or for a deleted upstream.
- **Auto-run sync inside `move`.** Rejected above; keeps one rule across mutating commands.
- **Drag-and-drop in the tree view.** Nicer UI, but needs the same API; #108 deliberately excluded it. The dialog comes first and the API supports drag later.
- **A folder-listing endpoint.** The resource list already carries what is needed, so the client derives it and avoids a second source of truth.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Behavioural code is ~400 lines Go (core, CLI, handler) and ~350 lines TS; the file count comes from completions, docs, i18n and tests. Non-goals: agents, tracked-repo skills, renaming a skill or folder, rewriting target filters, drag-and-drop.

## Test Plan

Unit (devcontainer only; load `testing` first):
- `internal/install`: `KeyForPath` (full and legacy basename keys), `Lock.MovePin`, tracked-ancestor helper.
- `internal/skillmove` table tests: record re-keyed with audit acceptances; skill without record; edited skill (hashes differ); no `file_hashes`; every refusal code; `same_folder`; `duplicate_dest` with `--force`; `overlapping_sources`; reconcile keeps nested records under an installed parent; skill with a nested skill (both re-keyed, pin and acceptances kept); folder move (all skills re-keyed, empty folder and non-skill files move along, one refusal such as a nested tracked checkout (including one with no discoverable skill) or link aborts before any rename, `dest_inside_source_folder`); dry-run leaves disk and store untouched; literal `.skillignore` rewrite; flat-name collision with and without force; batch with one failure keeps the others.
- `internal/config`: extend `project_reconcile_test.go`: after a move the project `skills:` entry has the new group and the lock keeps the old commit pin.
- `internal/server/handler_move_test.go`: success, dry-run, partial failure, 400s, project mode, oplog entry written.
- `tests/integration/move_test.go` (`testutil.NewSandbox`): basic, `--dry-run`, `--json`, `-p`, refusals, `Next` hint; after `sync`, the target has `grp__demo` and no `demo`.
- Dashboard: `moveFolders.test.ts` (exclusions), `queryEvents.test.ts` row, `MoveDialog.test.tsx` (payload sent; force retry only after collision; folder and descendants disabled), `InstallDialog.test.tsx` (picker sends the chosen `into`). No layout tests, per the frontend topic.

E2E runbook `ai_docs/tests/move_installed_skill_runbook.md`: `ss init`; install one local-path and one git skill; edit one of them; `ss move` both into `grp`; `ss move grp archive` for a whole folder (one skill with a followed link or tracked repo below it must refuse the whole folder); check `.metadata.json`, `ss doctor` (no "missing on disk"), `ss sync` renames target links, `ss check` still resolves the source; repeat with `-p` and confirm `skills.lock.json` pin and `config.yaml` group; `curl` the batch endpoint against `ss ui`. Screenshot dashboard dialogs in Clean/Playful × light/dark.

## Implementation Plan

Prerequisite: PR #513 merged (`MoveEntry`, `MovedEntryKey`). Work on a branch from `main` after that.

| Step | Worker | Files | Needs |
|---|---|---|---|
| 1. Folder picker in InstallDialog | C | `ui/src/lib/moveFolders.ts`(+test), `ui/src/components/FolderPicker.tsx`, `InstallDialog.tsx`(+test), `ui/src/i18n/locales/*.json` | nothing (separate PR, can start now) |
| 2. Core and CLI | A | `internal/install/{metadata,lock}.go`(+tests), shared tracked-repo helper (also edit `internal/uninstall/uninstall.go`), `internal/skillmove/*`, `cmd/skillshare/{move,move_handlers}.go`, `main.go`, 5 completion files, `completion_test.go`, `tests/integration/move_test.go` | #513 |
| 3. Server API | B | `internal/server/handler_move.go`(+test), `server.go` route | A's `Plan`/`Run` signatures (A pushes the skeleton first) |
| 4. Move dialog | C | `ui/src/api/resources.ts`, `api/types/resources.ts`, `components/resources/MoveDialog.tsx`(+test), `ResourceDetailPage.tsx`, `ResourcesPage.tsx`, `lib/queryEvents.ts`(+test), locales | B's JSON contract (above; mock until B lands) |
| 5. Docs, skill, runbook | A | files in "Docs and skill", `ai_docs/tests/move_installed_skill_runbook.md` | final flags from step 2 |

Suggested PRs: (1) picker; (2) core + CLI + docs; (3) server + dashboard. Verify with `make check` in the devcontainer, `pnpm run lint`/`build` for `ui/` and `website/`.

## Decisions

Maintainer decisions on the open questions of the first draft.

1. **No auto-sync after `move`.** Print `skillshare sync` as the next step, as every other mutating command does; the dashboard offers Sync now.
2. **Skills below a followed source link are refused in v1.** The folder is the user's own checkout and `uninstall` already treats it specially.
3. **Target `include`/`exclude` rules naming the old flat name: warn only.** `config.yaml` is user-owned; rewriting patterns by guess is riskier than a missed filter.
4. **Whole-folder move is in scope for v1.** `skillshare move <folder> <dest>` moves the folder with every skill under it (`frontend archive` → `archive/frontend`). A folder containing a tracked checkout or followed link is refused as a whole, and one refusal aborts the folder before any rename.
5. **`--force` is limited to name collisions.** It never overwrites an existing destination and never moves tracked or linked content, so it cannot be a universal bypass.
