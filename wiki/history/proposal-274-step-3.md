# Proposal 274 — step 3

## Outcome

[Proposal 274](../../proposals/274-skillfollow.md) step 3 is implemented on the
`runkids/274-step3` integration branch, after the step 2 write migration. This
records branch implementation, not a published release. `.skillfollow` and
`.skillfollow.local` opt strict first-level link names into discovery for groups
and tracked repositories. The public feature is experimental; single-skill
entries, `follow`/`unfollow`, and a dashboard declaration editor remain step 4.

| Slice / branch | Result / evidence |
|---|---|
| 3a / `runkids/274-step3a-follow` | Parser, classification, canonicalization, logical one-hop sourcewalk, initial list/status/doctor/sync plumbing; [report](../../ai_docs/reports/274-step3a-follow.md), commits `5402316f` through `7fc17990` |
| 3b / `runkids/274-step3b` | Link identity/ownership/status, conservative prune/copy pause, reconcile, output and junction simulation; [report](../../ai_docs/reports/274-step3b-ownership.md), commits `936f4db8` through `da763482` |
| 3c / `runkids/274-step3c` | Followed update policy, resolved-root audit/rollback, Git staging/incoming guards, doctor ignore diagnostics; [report](../../ai_docs/reports/274-step3c-git.md), commits `37e6a4b7`, `508d9cdf` |
| 3d / `runkids/274-step3d` | Remaining CLI/hub/server discovery, uninstall/write refusals, list suffix, matrix; [report](../../ai_docs/reports/274-step3d-follow.md), commits `a5ae2c31`, `080266ea`, `09360503`, `60f583eb` |
| 3e / `runkids/274-step3e` | Install-time reconcile, target merge-status and dashboard sync propagation; commits `1d32fea8`, `ab2e44db`, `1154fde6`, matrix `589076f3`, report `ai_docs/reports/274-step3e-wiring.md` on the integration branch |
| 3f / `runkids/274-step3f` | README/website/built-in skill guidance, proposal reconciliation, wiki milestone and translations |

The 3d report's `pending 3b`/`pending 3c` rows describe that isolated slice.
Current matrix source, not those historical rows, defines the integrated contract.

## Behavior and boundaries

- FollowSet parsing/classification lives in `internal/sourcewalk/{parse,follow}.go`;
  `Options{Follow: &set}` enables traversal. Nil-follow APIs retain legacy behavior.
  States are `missing`, `not-link`, `invalid-target`, `cycle`, `target-overlap`,
  `inside-git-root`, `entry-overlap`, `single-skill`, `followed`, `undeclared-link`.
  Logical source paths survive physical traversal; child links remain unfollowed.
  Read failures mark the entry missing and `Err()` prevents partial discovery.
- Relative skills links preserve the followed entry in their text. Identity is
  separate from ownership; status remains an aggregate connectivity count.
  Prune pauses while any declared entry is unavailable, even with force. Standard
  managed copies of unprovable origin are kept; merge relinks warn of returning
  collisions. Reconcile keeps unavailable-prefix metadata and does not create or
  refresh followed-tree metadata.
- Sourcefs refuses crossing or replacing a link. Followed roots and descendants
  cannot be uninstalled, even in dry runs. Dashboard content, toggle, target and
  source edits refuse; hide via source-root `.skillignore`. The generic error
  remains `<path> is a link; edit its target directly`, not the proposed
  followed-specific/unfollow message. The proposed special doctor sidecar check
  did not ship.
- The shared update policy is in `internal/install/followed_update.go`, adapted
  by Git. Followed repos require a clean tree, use `--ff-only --no-rebase`, and
  reject explicit force; dirty/diverged/status errors fail per item with the
  resolved path. Audit scans canonical roots with logical reporting and can
  hard-reset to the pre-pull commit. Snapshots are not concurrency locks.
- `git.FollowedLinksStaged` and `CheckSourceMutation` protect subprocess seams.
  Reachable links must be untracked and ignored with anchored no-slash lines;
  `.skillfollow.local` stays local. Source pull fetches/checks a pinned revision;
  incoming link paths, declared or not, and indexed declarations are refused.
  Checkout checks the selected existing revision without an implicit fetch.
  Discard preserves ignored followed links. Direct external Git is not guarded.

### Deliberate implementation deviations

`FollowSet.SourceRoot()` was added to return the canonical declaration parent
without resolving the final link for staging reachability. `handlePatchSkillSource`
refuses before metadata lookup independently; `findMetadataEntry` still refuses
linked lookup. CLI `audit.go` plumbing moved from 3d to 3c so the fallback audit
and root-resolution adapter had one owner. No new metadata kind, manifest schema,
automatic ignore/declaration mutation, or follow command was added.

## Integrated behavior matrix

Fixture: followed `_repo` with Git, followed `group`, declared `missing`, declared
regular-file `rejected`, and an undeclared link. The undeclared skill stays absent
from every discovery row; read/dry-run and refused mutation rows preserve source
and external bytes. Evidence: `tests/integration/skillfollow_matrix_test.go` and
`internal/server/handler_skillfollow_test.go` at `9f2ab980`, with the 3e additions
at `589076f3` inspected on the integration branch.

| Surface | Observable result |
|---|---|
| CLI list JSON / plain | Logical repo/group skills; `_repo` tracked; plain `→ <resolved>` |
| CLI status JSON | Followed repo count; missing/invalid-target states; `prune_paused` |
| CLI check JSON | Followed repo reported dirty in fixture; local group is not a tracked repo |
| CLI update-all dry-run | Followed dirty repo fails per item, not silently skipped |
| CLI audit | Three discovered skills in the CLI fixture, scanned through resolved roots |
| CLI doctor JSON | Followed passes, missing/invalid-target warnings, undeclared info |
| CLI uninstall dry runs | Root, descendant, basename, glob and all refuse link mutations |
| CLI diff | Followed logical skills reported as new target entries |
| CLI sync JSON / target info/list (3e) | `prune_paused` names blockers; followed links count shared/linked, not local |
| Server skills / overview | Logical repo/group skills; correct skill/top-level counts |
| Server check / update-all | Followed repo dirty; update item is an error; group excluded |
| Server hub / candidates / content | Followed skills visible/readable under logical paths |
| Server single mutations | Repo/skill uninstall, toggle, targets, content write: link refusal, HTTP 409 |
| Server batch mutations | Toggle/uninstall/targets return per-item failures without external writes |
| Server sync / targets (3e) | `TestServerSkillfollowSyncAndTargets`: missing/rejected entries pause prune, stale link kept, `linkedCount: 3`, `localCount: 0`; per-target `prune_paused`/`kept` and CLI warning text |

## Verification and limits

The four implementation reports record isolated Linux devcontainer `make check`
passes, including vet, both ratchets, unit and integration suites. 3a also records
no-declaration byte-equivalence checks, a Windows ARM64 sourcewalk cross-build,
and an initially timed-out existing init test that passed in isolation and on a
full retry without changes. 3b records junction simulation; 3c records Windows
amd64 git/sourcewalk cross-compilation and linked-root audit regressions. 3d records
its isolated matrix and no-declaration comparisons. These are report evidence,
not fresh executions by 3f. The 3e wiring report records its caller inventory,
pre-3e regression reproduction and new matrix tests; its final `make check`
result is delivered separately through worker_done. This docs task inspected the
3e report, handler and `TestServerSkillfollowSyncAndTargets` on branch tip
`01c616a6` after the coordinator identified that its base predates 3e.

This documentation-only task read the proposal/reports, relevant code strings,
and integrated matrix source on the host. `python3 scripts/ai-context.py check`
and `git diff --check` passed. A focused host-side relative-link/anchor inspection
validated the new page and its links. No CLI, Go tests/build, `make check`, website
production build/typecheck, frontend package scripts, or Windows runtime was run
by 3f. The documentation topic specifies a devcontainer website build, not a
host package-script lint; that build remains for the coordinator's verification.

**No real Windows junction or Developer Mode feature-runtime matrix was run.**
Earlier standalone Windows probes in the proposal and cross-compilation do not
establish full feature-runtime correctness. Nested links and single-skill entries
remain unsupported; update/link checks are snapshots, not filesystem locks.

### Questions resolved against evidence

- The proposal's `use unfollow`/followed-specific write text is not the shipped
  error: sourcefs and both matrices pin the generic link refusal. The separate
  source-change refusal and linked metadata-lookup refusal remain in
  `internal/server/handler_skill_content.go`; 3c/3d explain the split.
- The isolated 3d matrix report is stale for merged check/update/audit rows:
  `9f2ab980` supplies the current expectations, without rewriting its report.
- The docs worktree initially lacked server sync `Follow` propagation. The
  coordinator pointed to 3e `1154fde6`/`ab2e44db`; inspection of those commits,
  the 3e report and test resolved it. It is not an outstanding product limitation.
- No `doctor.go` sidecar diagnostic exists, despite the proposal text; the
  proposal now records that omission rather than promising it.
- Website instructions said English-only, but four locale trees contain 110
  translated docs each, including filtering/doctor. Actual content and configured
  locales are the evidence for keeping additions translated; the instruction
  was corrected without changing search/plugin configuration.

## Localization

`README.md` is the English source; language links keep English first then
`ja`, `ko`, `zh-CN`, `zh-TW`, as CONTRIBUTING specifies. This task updates the
four translated READMEs and website additions in `ja`, `ko`, `zh-Hans`, `zh-Hant`
under the existing current-doc paths. English code/commands, JSON keys, exact
runtime diagnostic strings and state identifiers remain literal for recognition.
The built-in skill, proposal and wiki remain English. No UI locale strings or
changelog/release history were changed. Existing unrelated untranslated headings
and code examples were not expanded into a broader translation cleanup.
