# OMP native support (#409)

## Completed implementation

The integration follows Oh My Pi v18.6.1 rather than treating it as renamed Pi.
Native MCP, skills, instructions, `APPEND_SYSTEM.md`, code Hooks and explicit
account roots use the existing Skillshare ownership/backup/sync mechanisms.
Automatic profile routing remains separate work (#259).

The Extensions inventory distinguishes native/configured/plugin/ambient-hook
resources and configuration selection, not loaded state. Verified standalone
modules support preview/apply of `disabledExtensions` with native lock
interoperability, revision checks, symlink/identity guards, surgical YAML
preservation and selection-only state backups. Hooks/plugin-owned, ambiguous,
explicit-file-bypass and unknown rows remain read-only. Management does not
execute extension or hook code.

The dashboard now follows Pi's card layout: files are grouped by source, scope
and directory, shared paths appear once, and Details holds notes and derived IDs.
OMP plugin groups now use inspected native package roots and names, not extension
folder basenames. Declared versions are shown when available. Scoped names retain
full identity in tooltips/Details; files use plugin-relative folder labels. Details
matches Pi's labeled two-column layout. No runtime state or ownership is inferred.
Pi/OMP file lists now remain visible without card-collapse controls; Details
still toggles independently without changing drafts. Read-only reasons and
Hooks/Plugins links remain available; folder labels do not invent native identities.
Pi packages proven managed by Skillshare now share the package icon and
`Plugins · <name>` heading. Pi/OMP plugin cards omit redundant origin tags and
use an accessible Open Plugins navigation icon; scope, Pi's known version,
project override shape and all native selection controls remain. Unmanaged Pi
packages keep their native identity and Pi icon.

Marketplace-backed local/Git Plugins support reviewed fresh-cache installation,
import, scoped removal and reinstall in matching user/project scopes. The dashboard
uses the common Preview/Apply workflow, without an OMP-only consent checkbox. OMP accounts do not establish separate native plugin stores.
Native npm/Git/link packages remain unmanaged discovery.

## Scoped removal and remaining update limit

OMP 18.6.1 native uninstall/upgrade was reproduced deleting or replacing cache
still referenced by a project invisible from the command's working directory.
Skillshare never invokes those destructive commands. A scoped adapter now removes
only the verified installation registration, runtime selection and module link;
shared cache, marketplaces and plugin settings remain. This uninstalls the plugin
from the selected scope rather than disabling it. Interrupted removal is retryable.
Reinstall allocates a fresh marketplace/cache identity instead of overwriting cache
another project may still use. Ambiguous metadata/ownership, foreign links, real
module folders and dependency collisions refuse writes. Windows requires verified
private ACLs; no permission changes are made.

Native revisions invalidate stale reviews, but OMP plugin commands do not share
the removal adapter's lock. Concurrent native mutations are unsupported; the
adapter is not an atomic multi-file native transaction. Updates remain blocked.
The boundary and upstream evidence are recorded in
[the plugin cache report](../../ai_docs/reports/omp-native-plugin-cache-safety.md).

OMP-specific message keys are translated in all 11 dashboard locales. Candidate
radio indicators now align with the first line, including multi-line descriptions.

## Verification

- Devcontainer `make check` passed after combined changes.
- Dashboard after Pi plugin-card alignment: 100 test files / 915 tests passed;
  build and focused component lint passed. The prior full UI lint reported 47
  existing warnings; that full lint was not repeated in this follow-up.
- Earlier changed-scope React Doctor reported no issues. The Pi-card follow-up
  scans the previously unchanged Pi component too: 85/100 with one duplicate-JSX
  warning for the existing Pi/OMP pending toolbars, whose code was not changed.
  This is not a new header issue or a whole-app verdict; no rules were suppressed.
- English website build, context-router check and `git diff --check` passed.
- Isolated Extensions API runbook passed (1/1): preview does not write, apply
  preserves YAML, concurrent edits invalidate approval, and code is not executed.
- Actual pinned native FileLock addon passed bidirectional Go/native contention
  and release/reacquire checks under focused race tests.
- Native 18.6.1 Plugin removal runbook passed (2/2): user removal/reinstall leaves
  an invisible project's registry/link/cache bytes unchanged, and project
  deselection-plus-sync leaves the user installation unchanged. No extension ran.
- Scoped-removal ownership/staleness/recovery and fresh-cache identity regression
  tests passed; macOS/Windows ARM64 plugin tests cross-compiled.
- Browser verification covered actual Extension apply, inert reviewed Plugin
  installation, blocked removal and Clean/Playful light/dark screenshots.
  Card alignment was checked in all four themes, including collapse/draft/review,
  Details and read-only account navigation; no browser errors were reported.
  The follow-up browser pass completed actual removal and fresh-cache reinstall
  with Traditional Chinese preview/result messages, zero review checkboxes and
  enabled Apply. A multi-line candidate's radio was 2px below the first line,
  with `align-self: flex-start`; no browser errors were reported.
- The isolated preview now has local Pi 0.99.2 on PATH. UI install of the inert
  `pi-preview-plugin` succeeded; Pi card switches, collapse/expand and review/cancel
  were checked without changing the user's `superpowers` or `ponytail` state.
  Pi cards were inspected at 1440×1100 in Clean/Playful light/dark, and OMP's
  redundant tags were checked. No Agent session or extension code was started.
- Final name/Details follow-up passed native plugin metadata tests for real and
  linked module directories, missing versions and OMP API tests. Plugin-root
  grouping, relative paths, always-visible rows, Details/draft independence and
  localized read-only messaging have focused UI regressions. The combined full
  UI suite passed 915 tests; build, focused lint, website and context checks passed.
  Both Pi/OMP were inspected in all four desktop looks with Details open; native
  full identity/source and plugin read-only controls were verified. Traditional
  Chinese labels/reasons were checked in the DOM. Browser session was closed.

Linux runtime verification does not establish macOS or Windows runtime behavior.
Platform lock adapters cross-compiled, but native platform acceptance remains
unverified. Existing narrow-screen dashboard overflow was not redesigned.
Native support was committed as `ce96d5ce2`; no release, push or real-user Agent
mutation occurred.

## Post-commit review fixes

- Installation now resolves the reviewed runtime package name, binds its selected
  scope's destination to the preview and rechecks absence before native install.
  An existing unregistered directory or link is never passed to native replacement.
- Missing cache manifests no longer cause guessed runtime names during removal.
  A unique runtime-lock key with a link to the exact cache proves the actual name;
  unverifiable active ownership blocks removal without dropping the binding.
- Whole-view reasons, inventory/review warnings and unresolved-path guidance now use
  all 11 locales, including fixed backend messages and parameterized paths. Unknown
  external diagnostics and real paths/identities remain unchanged.
- Existing OMP tests, `make check`, focused UI/i18n tests (22/22), lint and UI build
  passed. The existing lifecycle fixture restores its reviewed local source before
  testing reinstall; no new translation or mocked tests were added.
- Real pinned OMP in `omp-review-fix-verified-409` passed four scenarios: foreign
  runtime data survives blocked installation; empty-destination installation succeeds;
  unverifiable missing-manifest ownership blocks removal; and link-proven removal
  deletes the real runtime link, selection and registry. No executable plugin
  resources or Agent session ran. Browser DOM verification confirmed Chinese readonly
  reasons/warnings on `omp-work`; native terms and config_dir remain intact.
- React Doctor reports no new issues against `ce96d5ce2` (`--base HEAD`). Its broader
  branch scan still reports existing component maintainability warnings; scores across
  different scopes are not comparable. Platform acceptance remains unverified.

The review fixes are included in the OMP pull request branch. Before opening the
PR, main at `6b5a79de4` was merged without rewriting branch history. The Targets
conflict preserves main's tab navigation, OMP account hook counts and OMP's
Extensions link. Merged `make check`, focused UI/i18n (24/24), lint, UI build and
context checks passed; the English website build also passed. The full merged UI
suite had 917 passing tests and one pre-existing Memory Refresh failure. That
component and test are identical to main, and the failure reproduces in isolation;
no unrelated memory code or validation was changed. No release or real-user Agent
mutation was performed.

## PR #423: first Codex review

- Confirmed stale imported bindings could not be forgotten after native uninstall
  when another plugin kept node_modules present. Extended the existing removal
  recovery test: it failed before the fix, then passed. Cache/registration/known
  runtime state absence now permits binding-only Forget without touching native files.
- Rejected the ancestor-override suggestion against the pinned native contract.
  OMP 18.6.1 resolves a plugin registry anchor separately, but its loader and manager
  resolve plugin-overrides.json from the session cwd. Executing the installed native
  path helpers confirmed nested cwd paths, not ancestor paths; no Agent or resource ran.
