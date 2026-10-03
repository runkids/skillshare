# #342 Pi Extensions tab — implementation report

Branch `runkids/feat-pi-target-extensions` (base `84bbd407`), worktree
`feat-pi-target-extensions`. Implementation and verification record for issue #342;
no release or deployment was performed.

## What the feature does

- `internal/plugin/pi_extensions*.go`: read model, discovery, matcher, preview/apply.
- `internal/server/handler_pi_extensions.go`: `GET /api/targets/{name}/pi-extensions`,
  `POST …/preview` and `POST …/apply` (oplog `pi-extensions`).
- UI: an Extensions tab on `pi` and Pi-account target pages, and on the project
  page of a project that syncs to Pi, which saves only that project's `.pi/settings.json`.
- Docs: `website/docs/reference/commands/plugin.md` and its ja/ko/zh-Hans/zh-Hant copies,
  `skills/skillshare/references/plugins.md`, runbook `ai_docs/tests/pi_extensions_runbook.md`.

## Revision: usability, project editing, version support

User-approved revision of #342. It supersedes the earlier decision to keep project
configuration read-only; it does not touch trust, run extensions, or publish anything.

| Area | Change | Evidence |
|---|---|---|
| Table | Two columns: Extension and **Configured** (On / Off / Can't tell). Runtime and Effective are gone; file presence is a "File missing" badge, not a column. Can't tell names the pattern or rule and says to use `pi config`. Source, rules, global rules and kept keys moved into a per-package Details region (`aria-expanded`). | `TargetPiExtensions.test.tsx` 17/17; RED 12 failing before the rewrite. |
| Info note | One Info button (hover and focus) says the page changes settings only, Pi decides loading and trust, and a reload is needed; on a project it adds the trust explanation and the saved/default hints. | Vitest tooltip test; screenshot below. |
| Read-only reasons | `fork`, `noCli`, `unsupportedVersion` (names the found and the minimum version), `settings`, `credentials`, `reference`, folders (owner-specific: Extras or the folder) each end in a next step. | Vitest; 11 locales with the same key set, 0 missing or unused keys in the component. |
| Project write | `piProjectPlan` / `applyPiProject` (`pi_extensions_project.go`). A global package gets `{"source", "autoload": false, "extensions"}` with the source written as `pi config` writes it (npm/git verbatim, local relative to `.pi`, which may contain `../`), and it must resolve back to the same identity. Default removes only the exact project rule; an entry left with only `source` and `autoload: false` is removed. Replacement and project-only entries edit their own list. | `TestPiProjectOverrideOfAGlobalPackage`, `…OverrideReferences`, `…DeltaDefaults`, `…ReplacementEntries`, `…Batch`, `…DeltaWithoutGlobalBase`, `…KeepsAnEntrysCredentialsAndFields`, `…OverrideOfAPartialManifestPackage`. |
| Native check | Pi's own `DefaultPackageManager` resolves the files Skillshare wrote, trusted and untrusted, through `scripts/pi/resolve-probe.mjs` (trust passed in; trust store never read or written). | `TestPiProjectOverridesResolveInPi` (version matrix check `project-native`). |
| Write safety | Only `.pi/settings.json`, through an `os.Root` at the project. Revision binds both files' bytes (and the project file's absence), the scoped changes and the plan. An already refused or stale preview creates no folder, lock or backup. If Apply aborts after recording, only that attempt's new record is discarded; successful history is kept. Lock order: plugin lock, then Pi's `settings.json.lock`. A new file is linked into place (an existing one makes it stale); an existing one is renamed over. Backup and oplog as for global. After the backup, the plan is rebuilt from both files and the packages and must reproduce the revision before the write. | `TestPiProjectApplyIsBoundToBothFiles`, `…StaleApplyHasNoSideEffects`, `…WriteProtections`, `…ApplyIsBoundToDiscovery`, `…LockLostMidApply`, `…ApplyRevalidatesAtTheWrite`; server `TestPiExtensionsAPIProjectSavesOnlyProjectSettings` (global target refuses the project revision with 409). |
| Unsafe forms | Credentials or a query in a global source: read-only, never copied; the note says to change that global entry to a source without them (adding a second entry would not help, since Pi keeps the first). A project entry Skillshare can't read, or a global entry whose source it can't read: whole project view read-only. A first global entry that Skillshare can't read (bad rules, `autoload: false`) still owns its package: no rows, no override, and later entries of the package are not offered. The global view marks those later entries `duplicate`. An unresolved earlier global identity or later project identity leaves potentially shadowed entries `sourceUnknown`, with no rows/edit targets; inherited deltas likewise stay read-only. | `TestPiProjectSettingsForms`, `TestPiProjectFirstGlobalEntryGoverns`, `TestPiExtensionsUnreadFirstEntryStillDedupes`, `TestPiUnresolvedRegistrationMakesOwnershipUnknown`, `TestPiBadSourceMakesPackageOwnershipUnknown`. Unreadable source values likewise create precedence uncertainty; Native scenario 29 confirms replacement-decoded invalid UTF-8 can own the same identity as a later valid local source. Native probe on 0.99.2: global `[{pkg, ["*", "!…\ud800…"]}, {pkg, ["-a"]}]` keeps `a.ts` on (the second entry is ignored); with a project delta `[+a, -b]` trusted, `b.ts` stays on, so the result is not predictable and read-only is correct. |
| Version support | `PiMinVersion` = 0.99.2: any plain `X.Y.Z` at or above it is editable, backed by `scripts/pi/version-matrix.sh` (see `phase-0.md`, minimum-version update). Older, prerelease or unparsable versions are read-only. | `scripts/pi/version-evidence.json`: 0.99.2, 1.0.0 and 1.0.1 pass `contract/core` and `contract/bundle` (34 scenarios, 141 assertions), `native-lock`, `project-native`. |

Revalidation boundary: the project file is locked; the global settings are not (taking
Pi's global lock would write a lock directory into the global agent folder). The global
writer locks its own settings file but not the packages. Both writers therefore rebuild the
plan after the backup, right before the write, and refuse as stale unless it reproduces the
revision (`TestPiExtensionsApplyRevalidatesThePackageAtTheWrite`: the changed file removed,
the manifest changed, or a `skills` folder added under an extensions-only manifest before a
string entry is converted). What remains is the time between that replan and the rename;
package files and manifests are independent files, so no atomic snapshot is claimed. Removing a
package file that no change touches is not stale by design: the revision binds the bytes
written and the rows reviewed, which stay the same.

## Plugin inventory clarity follow-up (2026-10-03)

The Plugins page now labels its left list **Managed by Skillshare**, with its own package
count, and its right rail **Tools**. Native counts say **registered**, not installed or
managed: Pi's registrations can include a package that is not present on disk. The rail
explains that its counts also include already-managed packages. The empty state explains
why native registrations can coexist with zero managed packages and offers the existing
reviewed import flow, without reinstalling. All 11 locales were updated.

Two test-first regressions cover zero managed packages alongside native registrations,
the Pi Extensions link, and the managed count increasing only after an approved import.
The import test mocks the existing API; visual verification did not import or install any
package or alter fixture configuration. Verification: 46 focused UI/localization tests,
lint (0 errors / 47 existing warnings), UI build, and `make check` in the existing task
verification copy (15 UI files refreshed; integration 67.3s). React Doctor remains
45/100, with the same 17 errors / 286 warnings as before this follow-up. Inspected
1360×1000 zh-TW screenshots for Clean/Playful × light/dark at
`.playwright-mcp/pi342-ia-{clean,playful}-{light,dark}.png` and the expanded Pi row.
A locale hot reload briefly invalidated the i18n context; a full page reload restored it.
No backend or package-lifecycle behavior changed. Before preparing the PR, the full UI
suite was rerun on the final source: 95 files, 790/790 tests passed, without concurrent
Go verification.

## Review fixes in an earlier round

| # | Requirement | Change | Evidence |
|---|---|---|---|
| 1 | Unicode: never a wrong known answer | `piGlobMatch` is tri-state. A `?` pattern against a subject with a rune above U+FFFF, or with invalid UTF-8, is Unknown, because minimatch's `?` matches one UTF-16 code unit. | Golden data regenerated from Pi's minimatch: 696 cases, now with BMP (`é`, `中`) and astral (`😀`) patterns and subjects. Known answers must equal minimatch; Unknown is allowed only for that class. Before the fix Go was wrong on 7 cases. |
| 2 | Absolute exact rules | Removal, `Removed`/`Added` and the phantom-row check all compare the rule against both the relative path and the absolute path (`path.Join(pkg, rel)`). The raw rule that was removed is reported. | `TestPiExtensionsAbsoluteExactRuleBelongsToItsFile` covers select and default; mutating either half fails it. |
| 3 | Revision binding | Revision = sha256 of the target, settings path, raw bytes, the sorted canonical changes, plan entries and plan rows. A mismatch is stale and nothing is written. | `TestPiExtensionsApplyRefusesARevisionOfAnotherPreview`; server test: pi's revision applied to `pi-work` gives 409 and the file is unchanged. |
| 4 | No automatic backup pruning | Pruning was removed. A rule record is written before each apply and kept on success; an aborted attempt discards only its own new record. | `TestPiExtensionsApplyNeverPrunesBackups`: 25 old rule records, then 26 (not settings file copies). |
| 5 | Native lock | `piNativeLock` holds Pi's `settings.json.lock` dir the way proper-lockfile does. It renews the mtime every 2s, under the 10s stale age. "Owned" means the same inode **and** the mtime we last set. One `owned()` check guards renew, `verify()` (run right before the atomic write, and also failing if the lock is older than stale − 3s) and `release()`. Renewal/release never touch a lost or foreign lock; acquisition may reclaim an unchanged empty stale directory (follow-up below). | Deterministic tests: renewal, too-old refusal, takeover by replace, touch or remove (each case also asserts the other party's lock is left exactly as it was). Interop: `TestPiNativeLockHoldsAgainstPi` runs Pi's own proper-lockfile (`scripts/pi/lock-probe.mjs`); it reports "locked" after 12s and acquires the lock after release. Without renewal Pi steals the lock (control). `go test -race … -count=3` ok. |
| 6 | Project inventory | Pi 0.99.2 (`addAutoDiscoveredResources`) always loads the global `agentDir/extensions` with the global overrides, and loads the project's `.pi/extensions` only when it trusts the project. The project view lists both, as `scope: project` and `scope: global`, each evaluated by its own settings, with Extras provenance. The UI tags them, and all rows are read-only. | `TestPiExtensionsProjectListsGlobalAndProjectFolders` RED (global omitted) → GREEN; Vitest folder-scope test. `scripts/pi/extensions-crosscheck.mjs` now checks both directions: every non-builtin extension Pi resolves must be listed (`omitted`). |
| 7 | Fork | A fork returns read-only before any CLI run. The native `pi --version` probe is kept. | The account test's fork Runner calls `t.Fatalf` if it is invoked. |
| 8 | "Use default" semantics | The button is **Remove rule**. It deletes the exact rule, and the file follows the remaining rules. A pending removal shows no switch and "→ Shown in review"; the preview computes the result. Rows under `extensions: []` are read-only, with the `[]` origin text as the reason. | `TestPiExtensionsRemovingARuleFollowsTheRemainingRules` (glob keeps it On; `!` keeps it Off); `TestPiExtensionsEmptyListRowsAreReadOnly`; Vitest pending-removal test. |

## Source credential redaction (final security fix)

`redactSource` used to mask only URL userinfo, so a source like
`git:https://example.com/repo?access_token=…` reached GET and preview unchanged.

- For URL and git sources (`://`, `git:`, or `user@host:path`), the whole query is now replaced
  with `?***`, as well as the userinfo. Local paths and `npm:` names are shown as written.
- A git source with a query is now Unknown (`piGitLocation` refuses it). Its identity and install
  path no longer carry the query, and it is read-only. Discovery/loading of such sources remains unverified. The latest native contract verifies query-normalized identity and first-global/last-project precedence on both supported versions, but Go does not infer identity from that partial evidence: an earlier unresolved global or later unresolved project entry makes potentially shadowed entries read-only.
- Display only: changes match an entry by index plus the redacted source on both sides, and the
  write uses the raw entry. A string entry converted to an object keeps the source byte for byte.

Tests (dummy values only):

- RED before the fix:
  - `TestPiExtensionsSourceDisplayCarriesNoURLCredentials`: 3 cases failed (BasicAuth with a
    query, a query-only token, a git form without `://`).
  - `TestPiExtensionsGitSourceWithAQueryIsUnknown`: 2 cases failed (an identity carrying the
    token).
  - `TestPiExtensionsAPINeverSendsSourceCredentials`: GET sent `dummy-` values. The test also
    checks that the preview sends none and that GET and preview leave the settings bytes
    unchanged.
- Already passing before the fix, kept as a guard: `TestPiExtensionsConvertingAnEntryKeepsItsRawSource`.
  A BasicAuth git string entry is converted on apply, the view and plan JSON hold no dummy value,
  and the file keeps the raw source.

GREEN after the fix:

- `go test ./internal/plugin` ok.
- `go test ./internal/server -run PiExtensions` 5/5 ok.
- `go test -race ./internal/plugin -run 'PiNativeLock|TakenOver|PiExtensionsApply|Source|Convert'` ok.
- `make check` in a refreshed byte-identical valid-git copy (2290 files, `diff -rq` clean): exit 0,
  29 unit packages, integration ok.
- The task-owned API on :49423 was rebuilt and restarted.
- UI and website are unchanged by this fix, so their earlier results stand.

UI request: the project note is one short line, «Read-only: Skillshare can't confirm whether Pi
trusts this project.», followed by an Info button (`aria-label` "About project trust", localized).
The button is wrapped in the existing `Tooltip` and opens on hover and on keyboard focus. The
tooltip holds one concise explanation plus a separate line of saved/default hints, labeled as
hints and not Pi's decision. The duplicate `projectHint` paragraph and its key were removed. Stale,
busy and error alerts are unchanged, and project Apply stays unavailable. Vitest checks that the
tooltip is hidden before hover, appears on hover and on Tab focus, and that the detail never
appears inline.

## Other resources on string-to-object conversion, and single-file sources

A first rule turns a string entry into `{"source": ...}`. Pi 0.99.2 `collectPackageResources`
treats the two forms differently when the package has a `pi` manifest. A string entry takes
skills, prompts and themes only from their manifest fields. An object entry, for each type it
does not filter, uses the manifest field, or the convention folder when the manifest leaves the
field out. A package whose manifest lists only `extensions` and that has `skills/` or `prompts/`
therefore loads them after conversion. Preserving the entry's other keys does not prevent this.
The old guard compared extension rows only, so it allowed the conversion.

- `piLayout.conversionLoadsOthers` is true when equivalence can't be shown. That is the case
  when there is a manifest, some of `skills`, `prompts` and `themes` is not a string array in it,
  and the package root has anything at that name. The name is checked with `os.Root.Lstat`, so a
  link, a dangling link or an unreadable entry counts and no package problem is raised. This
  does not claim that extra resources load: an empty folder is treated the same. Which files Pi
  would load is not evaluated, and no `skills`/`prompts`/`themes` keys are written to compensate.
- A string entry of such a package gets `readOnly: "otherResources"`: its rows keep their
  inventory and selection but have no switch, and the card says why. `piGuard` refuses the same
  case before any write, so a forced preview or apply changes nothing. Object entries, packages
  without a manifest, manifests that declare every other type (including `[]`), and
  extension-only packages stay editable.
- A local source that is one file gets `readOnly: "singleFile"`. Pi adds it as an extension and
  ignores its filters, so the previous switch could never be applied. Pi's resolver lists any
  such file, a `.md` too, as an enabled extension and its loader takes it without a type check.
  The row shows that configured result (Loads), with Runtime Unknown as everywhere.

Evidence:

- Native, in `scripts/pi/phase0-contract.mjs`, which only resolves; fixture extensions throw if
  imported:
  - scenario 18: the partial manifest gives `a:on` and no skills, prompts or themes as a string,
    and `a:off`, `review:on`, `commit:on`, `dark.json:on` as
    `{source, extensions: ["-extensions/a.ts"]}`.
  - scenario 19: no manifest, a full manifest (`skills: []`, `prompts: [...]`) and an
    extension-only package keep the same skills and prompts across conversion.
  - scenario 20: a single-file source with `-one.ts` still loads it, and a `.md` file is listed
    as an enabled extension.
  - Run: 20 scenarios, 50 assertions, exit 0.
- RED before the fix:
  - `TestPiExtensionsConversionThatWouldLoadOtherResourcesIsRefused` failed.
  - `TestPiExtensionsSingleFileSourceIsReadOnly` failed.
  - The four controls in `TestPiExtensionsConversionThatKeepsOtherResourcesIsAllowed` passed, and
    still pass.
  - The vitest "says why a package has no switches" failed.
- Mutations, each reverted and checked with `cmp`:
  - With `readOnly()` returning "", the domain tests and the new API test
    `TestPiExtensionsAPIRefusesAConversionThatLoadsOtherResources` fail. That API test checks
    GET, a refused preview, a refused forced apply, and byte-identical settings.
  - With `piGuard` skipping the check, `TestPiGuardRefusesReadOnlyPackages` fails.
  - With `themes` dropped from the gate, the themes case of
    `TestPiExtensionsConversionWithThemesOrALinkIsReadOnly` fails. That test also covers a
    `skills` link leaving the package and a dangling `prompts` link, which stay read-only with
    no package problem and their extension row listed.
- Checks run after the fix:
  - `go test ./internal/plugin` ok; `go test ./internal/server -run PiExtensions` ok.
  - UI: `TargetPiExtensions.test.tsx` plus `i18n.test.ts` 21/21; lint 0 errors (47 existing
    warnings); build ok. The full UI suite was not rerun.
  - `make check` in a refreshed byte-identical valid-git copy (2290 files): exit 0, 29 packages
    ok, integration 64.1s.
  - The website build was not run; the docs change is prose only.
  - The task API on :49423 was rebuilt and restarted with Pi on its PATH (view editable, Pi
    0.99.2).
- `TestPiExtensionsSingleNonCodeFileShowsPisSelection` checks that a `.md` single-file source
  shows Loads and has no switch.

## Strings Go and Pi decode differently

`encoding/json` (`decode.go`, `unquote`) replaces an unpaired UTF-16 surrogate escape with U+FFFD,
and it also replaces invalid UTF-8. `JSON.parse` keeps the lone surrogate. Pi resolves the rules
`["*", "!extensions/\ud800.ts", "-extensions/a.ts"]` against `a.ts` and a file named with a
literal U+FFFD as `a:off` and `U+FFFD.ts:on`. Skillshare read the middle rule as
`!extensions/U+FFFD.ts`, showed that file as skipped, and would have written that changed rule
back when another extension was switched.

- `piLossless` scans the raw JSON bytes of a value before Go decodes it. It accepts valid
  surrogate pairs, a literal or escaped U+FFFD, and `\\u…` (an escaped backslash followed by
  text). It rejects a lone high or low surrogate escape and invalid UTF-8. It does not implement
  Pi's string semantics. It only marks where Go would differ.
- It applies to resource-bearing fields only:
  - A package source (string or `source` key) or an entry's `extensions` rules: the entry gets
    the existing `unsupportedEntry`, with no rules or rows, so preview and apply have nothing to
    change.
  - The manifest's `pi.extensions`: the existing `manifestGlob` problem.
  - The settings' top-level `extensions`: the folder gets `unsupportedEntry`.
  - Other keys stay opaque and are not checked.
- A source that is `null`, empty or whitespace only is now `unsupportedEntry`. Before, Go read
  `null` as `""`.

Evidence:

- Native, scenario 21 of `phase0-contract.mjs`: the settings file holds the `\ud800` escape, Pi
  gives `a:off` and `U+FFFD.ts:on`, and the literal-U+FFFD rule turns that file off.
- RED before the fix:
  - `TestPiExtensionsUnpairedSurrogateRuleIsUnsupported`: the view showed
    `Rules:[* !extensions/U+FFFD.ts -extensions/a.ts]` and both files as skipped.
  - `...ElsewhereIsUnsupported`: the manifest, top-level and source cases failed.
  - `TestPiExtensionsEmptySourceIsUnsupported` failed.
  - The five controls in `TestPiExtensionsLosslessStringsStaySupported` passed: a pair, a
    literal and an escaped U+FFFD, an escaped backslash, and an emoji with BMP text.
- GREEN after the fix:
  - `TestPiLossless` covers the scanner's edge cases.
  - `TestPiExtensionsAPIRefusesAnEntryGoCannotReadLosslessly` checks that GET sends no rules or
    rows, that the preview and a forced apply are refused, and that the settings stay
    byte-identical.
- Mutation: with `piLossless` always true, the four domain cases and the API test fail.
- Checks run:
  - `go test ./internal/plugin` ok; server PiExtensions 7/7.
  - Native contract: 21 scenarios, 53 assertions.
  - `make check` in a refreshed byte-identical valid-git copy (2290 files): exit 0, integration
    65.3s.
  - The task API on :49423 was rebuilt and restarted (view editable, Pi 0.99.2).
  - UI component code is unchanged, so the UI suite was not rerun.

## Verification

Run in container `ss-pi-ext` unless marked host.

### This revision (2026-10-03)

- **`make check`** in a refreshed copy `/tmp/pi-ext-gitcheck` (tar with the excludes below,
  `git init`, 2295 files, `diff -rq` byte-identical to `/workspace`): exit 0. docs-check,
  gofmt, vet, and 28 packages ok, 0 FAIL; integration 67.1s. This ran on the final source,
  after the global-writer boundary check and the credentials copy; afterwards only this
  report changed.
- **Go, narrow.** `go test ./internal/plugin` ok; `go test ./internal/server -run PiExtensions` ok;
  `go test -race ./internal/plugin -run 'PiProject|PiExtensionsApply|PiNativeLock'` ok. The
  independent review's probes were RED first, then GREEN. Before the fixes, the second
  same-package global entry was editable in both views, and apply returned nil after a global
  or package change at the write: in the project writer, and in the global writer for all three
  package cases.
- **Version matrix.** `scripts/pi/version-matrix.sh 0.99.2 1.0.0`, rerun on the final Go source:
  every check passed for both versions, and `scripts/pi/version-evidence.json` came out
  byte-identical.
- **UI.** Full `vitest run` with no Go job running, also after the final locale change: 95 files, 788/788. `pnpm run lint`: 0 errors,
  the 47 existing warnings, none in changed files. `pnpm run build` ok. react-doctor
  (`--scope changed`, which falls back to a full scan in the container): 17 errors, 286
  warnings, none in `TargetPiExtensions` after `RowSwitch`, `RowPath`, `EntryDiff` and
  `ReviewFailure` were split out of `ExtensionRow` and `ReviewDialog`.
- **Website.** `pnpm run build`: en, ja, ko, zh-Hans, zh-Hant ok.
- **Host.** `python3 scripts/ai-context.py check` ok.
- **Independent coordinator verification.** Re-ran the version matrix: both pinned versions
  passed all four checks (core and bundle: 27 scenarios / 72 assertions each, native lock,
  editor-written project settings resolved natively). The final focused Go regressions passed
  under `-race` with Pi 1.0.0, including first-global-entry precedence and write-boundary
  revalidation. Inspected the four-theme screenshots and hover Info note; host context-router
  and `git diff --check` passed. No full-suite rerun by the coordinator was needed after the
  worker's final checks; subsequent coordinator edits only corrected this report.
- **Fixture E2E** (API on 49423, binary rebuilt, fixture `/tmp/pi-ext-e2e`, runbook step 8):
  - The project view `acme@pi` is editable on Pi 0.99.2 with saved trust `trusted`.
  - The preview left the global settings, `trust.json` and the project file byte-identical.
  - The global target refused the project revision with 409 `pi_extensions_stale`.
  - The project apply did two things. It created
    `{"source":"npm:@acme/reviewer@0.4.2","autoload":false,"extensions":["-src/slow-check.ts"]}`,
    and it removed `-extensions/guard.ts` from the existing delta. Only `.pi/settings.json`
    changed, and the oplog records `pi-extensions` for `acme@pi` with status ok.
  - Pi's resolver, trusted: `slow-check.ts` and `guard.ts` off. Untrusted: `slow-check.ts` on,
    because Pi ignores the project settings.
  - Crosscheck after the write: pi 8/8, pi-work 4/4 and acme@pi 8/8, with no mismatches and
    nothing omitted.
- **Screenshots** (desktop 1360px, zh-TW, inspected), in the git-ignored `.playwright-mcp/`:
  `pi342-project-clean-light.png` (full page), `pi342-project-info-clean-light.png` (Info
  tooltip on hover), `pi342-project-clean-dark.png` (Details open),
  `pi342-project-review-playful-light.png` (review dialog: an existing npm delta's pending
  rule update and exact rule removal; not applied), `pi342-project-playful-dark.png`, `pi342-fork-playful-dark.png`
  (read-only note with a next step).

### Earlier rounds

- **`make check` in a valid-git copy.** In `/workspace`, `.git` points to a host gitdir that is
  not mounted, so the copy is used instead. Exit 0: docs-check, fmt-check, vet, 29 unit packages,
  and integration ok in 90.2s. Commands:
  ```sh
  cd /workspace && tar -cf - --exclude=./.git --exclude=node_modules --exclude=./bin \
    --exclude=./website/build --exclude=./website/.docusaurus . | tar -xf - -C /tmp/pi-ext-gitcheck
  cd /tmp/pi-ext-gitcheck && git init -q && git add -A && git commit -qm snapshot   # 2290 files
  diff -rq --exclude=node_modules --exclude=.git --exclude=bin --exclude=build --exclude=.docusaurus \
    /workspace /tmp/pi-ext-gitcheck   # no output: byte-identical
  make check
  ```
  This describes the first verification checkpoint. Subsequent refreshed passes are recorded
  above; the latest includes the resource-default and JSON-string guards (integration 65.3s).
  UI checks are recorded separately below.
- **Narrow Go checks.** `go test ./internal/plugin` ok. `go test ./internal/server -run PiExtensions`
  ok. `go test -race ./internal/plugin -run 'PiNativeLock|TakenOver|PiExtensionsApply' -count=3` ok.
- **RED/GREEN by mutation.** Each mutation below fails a test:
  - lock: no verify, no renewal, no margin, inode-only release;
  - revision ignores the changes;
  - pruning restored;
  - removal compares the relative path only;
  - phantom row;
  - fork CLI runs;
  - `[]` rows editable;
  - astral `?` answered as known;
  - project view drops the global folders;
  - UI: tooltip detail shown inline, a switch shown during a pending removal, folder scope tag removed.
- **UI.**
  - Full `vitest run` at the class-cleanup checkpoint, before the later package read-only notes,
    with no Go or `make check` job running in parallel: 95 files, 782/782 passed. The earlier
    HooksPage 5s timeout did not recur; the test and its timeout are unchanged. After the
    read-only notes changed, the feature and localization tests passed 21/21; the full suite
    was not rerun on that later UI source.
  - `pnpm run lint`: 0 errors, the 47 pre-existing warnings, none in changed files.
  - `pnpm run build`: ok.
  - react-doctor: the coordinator reran `npx react-doctor@latest --verbose --scope changed`
    inside the container on the final UI source. The host-only gitdir is unavailable there,
    so the tool explicitly fell back to a full scan: 374 files, 45/100, 303 issues (17 errors,
    286 warnings), with category counts identical to the 370-file base archive. No new
    TargetPiExtensions diagnostic appeared. The earlier extra complexity hit in `ExtensionRow`
    had been removed by splitting out `PendingSelection`.
  - Final class edits: the pending selection uses `text-link` (was `text-accent`), and the review
    diff box uses `rounded-lg`, which maps to `--r-box` (was `rounded-[10px]`). Lint, build and the
    full vitest run above came after these edits. In the review dialog the box radius is 12px in
    Clean light and dark, and 14px in Playful light and dark, equal to `--r-box` each time; the
    pending text is the link color (Playful dark `rgb(157, 189, 240)` = `--accent`).
- **Website.** `pnpm run build`: ok for en/ja/ko/zh-Hans/zh-Hant, 0 broken links.
- **Coordinator final checks.** The selected lossless/source/conversion domain regressions
  passed with `go test -race`; all PiExtensions server tests passed. The fixture API returned
  HTTP 200, native Pi 0.99.2 and an editable global view; Vite returned HTTP 200.
- **Host.** `python3 scripts/ai-context.py check` and `git diff --check`: ok.
- **Retained resources.** Container `ss-pi-ext`; task volumes `ss-pi-ext-home`,
  `ss-pi-ext-ui-nm`, `ss-pi-ext-web-nm`. The Go cache volumes are shared. The existing
  `skillshare_devcontainer_dev-home` is mounted read-only at `/opt/agent-clis` using only
  its `.local/agent-clis` subpath; no private home/settings paths are mounted.
- **E2E runbook on a fresh fixture** (`/tmp/pi-ext-e2e-rb`, port 49424): all 12 API/CLI steps pass.
  - Preview writes nothing.
  - A held Pi lock gives 409 and is kept.
  - Apply changes only the one `extensions` line, and Pi still reads the file.
  - Reusing the revision gives 409 stale.
  - The fork is read-only.
  - The project is read-only and lists its project folder plus the global folder (2 rows); its
    file checksum is unchanged.
  - A non-Pi target gives 404.
  - Crosscheck: pi 8/8, pi-work 4/4, acme@pi 8/8, with no mismatches and nothing omitted. The
    negative control exits 1 with 4 omitted.
  - The oplog shows error, ok, error and there is 1 backup.
  - Pi's proper-lockfile interop passes in 12.1s.
- **Runbook correction.** The first run on the new code failed step 5. The runbook took the
  revision from the GET view; it must take it from the preview, now that the revision covers the
  changes. The runbook was fixed. The UI already used the preview revision, and a live
  Remove rule → preview → Apply on the dashboard succeeded.
- **Earlier screenshots (desktop, zh-TW).** Inspected ephemeral local captures (not
  included in the repository); suffixes below identify those earlier captures:
  - Resting project view without the tooltip: Clean light `-27`, Clean dark `-32`, Playful light
    `-35`, Playful dark `-38`. The icon contrast fix is in `-40`, `-41` and `-42`.
  - Tooltip on hover: Clean light `-31`, Playful light `-37`, Playful dark `-39`. On Tab focus:
    Clean dark `-34`.
  - Global and project folders with scope tags: `-28`.
  - Pending Remove rule `-44`, review dialog `-45`, applied `-46`.
  - After the class edits, pending text and review dialog: Clean light `-49`/`-50`, Clean dark
    `-53`/`-54`, Playful light `-55`/`-56`, Playful dark `-57`/`-58`. Nothing was applied, and the
    pending toggle was discarded.
- **Preview** (kept running): Vite at http://127.0.0.1:45176, using the API at
  http://127.0.0.1:49423 with fixture `/tmp/pi-ext-e2e`.

## Limitations and deferred work

- Pi's built-in extensions are not shown in the tab; the crosscheck skips them explicitly.
- Git package installs are not mapped by the crosscheck (none in the fixture). If one were
  installed, its rows would be missing from the comparison, and the omission check would
  report them.
- Editing needs the target's own native Pi to be 0.99.2 or 1.0.0, with strict JSON settings.
  Other versions, a missing CLI, forks and unreadable settings are read-only. A project's
  settings are editable on the same terms, but Pi applies them only if it trusts the project;
  Skillshare shows its saved and default trust as hints and never decides or writes trust.
- The tab shows what the settings select, never whether Pi loaded an extension: Skillshare
  never runs extensions.
- Verification ran only in the isolated Linux devcontainer, with temporary HOMEs, local fixture
  packages, an npm package laid out in place, and Pi 0.99.2's own code read but no extension
  loaded. There was no runtime validation on Windows or macOS, and no real `pi install` from npm
  or git.
- A string entry is read-only when its package has a `skills`, `prompts` or `themes` entry that
  its manifest leaves out. Skillshare can't show that converting the entry keeps those resources
  unchanged. This is conservative: an empty folder or a link is enough, even if Pi would find no
  file there. Its extensions can still be chosen with `pi config`, or after the entry is
  rewritten as an object by hand.
- A single-file source is read-only. It shows Pi's configured result, Loads, even for a non-code
  file. Whether Pi's loader accepts such a file was not run.
- Supported input boundary: a source, entry rules, a manifest's `pi.extensions` or the top-level
  `extensions` with an unpaired UTF-16 surrogate escape or invalid UTF-8 is read-only, and so
  is an empty source. Skillshare does not model JavaScript strings beyond that.
- Deferred: the full Diagnostics UI, duplicate-load detection, runtime export, mobile layout,
  converting a replacement into a delta, and a trust bridge.
- Both writers rebuild the plan right before the write, but files outside the locked
  settings file (the global settings for a project, and package files and manifests) are not
  locked: a change in the short time between that last check and the rename is not seen.

## Filtered import and stale-lock follow-up

- Approved scope: reclaim an unchanged empty native lock older than 10 seconds; retain
  refreshed/replaced/nonempty directories, files and symlinks. A directory-only rmdir is
  nonrecursive; the inode stays anchored while inspected (the Windows follow-up below
  replaces the original os.Root anchor). A competing acquisition
  after removal is refused, never removed. Existing owned renewal/release checks are unchanged.
  As in proper-lockfile, mtime expiry is a lease policy, not process-death evidence; the final
  stat/rmdir is not an atomic compare-and-swap.
- Verified Pi object registrations can be adopted without native settings writes or lifecycle
  commands. Preview exposes only sorted field names. Exact entry bytes (including opaque
  numbers/escapes) are stored privately, mode 0600 under a 0700 directory on Unix;
  Windows uses the ACL protection described below. YAML/API bindings
  carry only a content digest. Records are scoped to the target/settings path/native ID, are
  never automatically pruned, and changed records are refused rather than overwritten.
- Normal sync/update preserve live entries. Uninstall captures current native rules/options,
  not the old import snapshot. Reinstall restores the object before pi install, avoiding a
  transient default-enabled string registration. Native trust remains Pi's decision. Failed
  native install remains pending and retries only the identical restored entry; external edits
  and ambiguous ownership are refused, including at the last write boundary.
- Unsupported versions/sources/encoding/option shapes and local references that Pi would
  normalize are not imported. Private state must accompany these bindings; missing or
  cross-target records block restoration. Filtered OpenCode imports remain unsupported.
- Evidence: stale-lock RED/GREEN, refresh/replacement/content races; filtered lifecycle in
  global/project/account, latest-rule capture, zero-write preview/adoption, private-value
  exclusion, stale approval, cross-account/tampered records, late writes and native retry.
  Native 0.99.2/1.0.0 core+bundle: 31 scenarios/127 assertions each, native-lock/project-native
  passed. The native persistence probe uses real local install and remote registration methods,
  not real npm/Git installation; offline update does not establish network update breadth.
- Verification before incorporating the already-published main merges: valid-Git make check;
  UI 95 files/800 tests; lint zero errors/47 existing warnings; UI and five-locale website
  builds; changed-scope Doctor unchanged at 90/100, five warnings. After preserving those
  remote commits in a branch merge: valid-Git make check, Windows amd64 cross-compilation,
  UI 98 files/823 tests, lint/build, and five-locale website build passed again. Task API/Vite
  were restarted after an external container restart; the retained global view reports
  Pi 0.99.2/editable, without changing fixture settings.
  Windows cross-compilation is not Windows runtime evidence. New visual screenshots were not
  run: browser automation is available only on the host, outside this task's tooling boundary.

## Review: legacy global npm fallback

A missing user-scoped managed npm directory now yields Unknown/sourceUnknown with
no extension rows or write candidates. It does not establish notInstalled: native
Pi can use legacy global npm/pnpm paths. No host-global root is probed. Project
npm misses remain notInstalled, because native Pi has no legacy fallback there.
`TestPiMissingManagedNpmRootDoesNotClaimLegacyIsMissing` reproduced RED and now
passes; all plugin race tests passed. The isolated native fallback probe uses a
substituted root lookup, actual native getNpmInstallPath and an empty fixture root;
0.99.2/1.0.0 core/bundle each pass 32 scenarios/130 assertions, plus lock/project-native.

## Review: three reproduced correctness failures at 16d4f8ac

- Removing the last exact rule now keeps an empty winning project override when
  deletion could expose an earlier same-identity or unresolved registration.
  Only the requested extension changes; older resource filters remain shadowed.
  `TestPiProjectDefaultDoesNotRevealShadowedResources` reproduced RED for earlier
  replacement/delta/unresolved sources and verifies preview, applied selection,
  retained precedence, unchanged earlier resource fields, global settings and trust.
- A filtered restore batch uses Apply-local receipts keyed by settings path. Each
  receipt records the reviewed hash and only the exact bytes our successful atomic
  restore wrote. Later restores still validate at acquisition and the write boundary;
  no native/external post-command snapshot is adopted. The nine global/project/account
  batch cases cover two successful restores, external mutation refusal and a native
  failure that preserves its retry while the second restore succeeds. All three
  successful-batch scope cases reproduced RED before the fix.
- `autoloadFalse` accepts only the explicit JSON literal `false`, matching native
  `=== false`. Sixteen global/project cases cover null, booleans, a string, number,
  array, object and whitespace. The null cases reproduced RED; other values retain
  ordinary filtering semantics and bytes.
- All plugin race tests pass. Added native scenarios demonstrate the unrequested
  extension/skill/prompt changes caused by deleting a winner, preservation by keeping
  it, and strict-false filtering. Both supported versions' core/bundle contracts pass
  34 scenarios/141 assertions, plus native-lock/project-native checks. Valid-Git
  make check, focused plugin/Pi API race tests, Windows ARM64 cross-compilation,
  five-locale website build, context-router and diff checks also passed. A container
  stop interrupted the first broad run; the same container was restarted without
  removing data and the interrupted checks were rerun to completion.
- At this correctness checkpoint, Windows work remained separate: ARM64 UTM evidence
  at pinned 39294b56 confirmed both
  native launchers, lock interoperability and project resolution under full/basic
  tokens, but stale reclamation fails because OpenRoot's initial Windows handle does
  not share deletion; a POSIX-mode-only record assertion also fails. Those issues
  were not fixed by that correctness follow-up. No user settings were used.

## Windows follow-up: #358

Pinned `2dc6564ba5a12a2c147e5836f969a5fcc636e274` passed actual Windows 11 Home
ARM64 UTM acceptance with both Interactive desktop full and basic-user tokens.
Developer Mode was off. The original checkout, #350's worktree/preview, existing
kits and reports were retained. No user settings, trust or global Pi installs were used.

- Stale-lock anchoring uses an explicit share-delete handle. Windows removes the
  anchored empty directory with POSIX disposition; Unix retains directory-only
  rmdir. Unsupported filesystems fail closed. Fresh/future/nonempty/file locks,
  renewed/replaced owners and concurrent contents remain untouched. Final mtime
  check/removal still does not claim an atomic CAS.
- Go's Windows Lstat defers file-ID lookup by pathname. Acquired lock identity is
  now captured through a handle before verification/release, so a replacement
  with matching mtime cannot be adopted. That new regression reproduced RED in
  both tokens at `ebf4e062` before the fix. The stale fixture likewise captures
  its old ID through the retained anchor rather than resolving the reused path.
- The original records inherited interactive/service/logon-class grants in the
  isolated public fixture: POSIX 0600 did not establish Windows privacy. New
  directories are created with protected current-user/SYSTEM inheritable DACLs
  before writing raw bytes. Existing directory/file ownership and DACLs are
  validated without rewriting ACLs. Only the current user and privileged
  SYSTEM/Administrators may be granted access; privileged default owners on full
  Windows tokens still require a current-user grant. Unsafe/unknown ACLs refuse
  import/restoration and retain settings, records and existing ACLs.
- RED at `073fc025`: stale/replaced locks, actual private ACL assertions, and
  broadly accessible existing-record reuse failed in both tokens. GREEN at the
  pinned fix above: 14 top-level regressions per token; native launcher and two
  native tests per version (0.99.2/1.0.0) all exited zero. The initial regression
  pass intentionally omits PI_ROOT and skips its native-lock test; both separate
  native passes executed it. Basic token symlink creation was unavailable and
  that subcase skipped; full token executed the symlink case. No extension
  factories or real npm/Git package install/update breadth is claimed.
- Reproduction: `scripts/windows/build-pi-kit.sh`,
  `scripts/windows/e2e-pi-extensions.ps1`, and
  `ai_docs/tests/windows_pi_extensions_runbook.md`. The kit is built from a pinned
  archive in the devcontainer. Runner HOME/config/temp/npm roots are isolated;
  child streams and UTF-8 copies are retained, and nonzero/missing checks fail.
