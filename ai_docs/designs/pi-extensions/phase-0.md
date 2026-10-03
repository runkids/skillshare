# Pi extensions — Phase 0 contract evidence (issue #342)

Original research checkpoint: no production code, settings, or design prototypes were changed.
Date: 2026-10-03. Historical findings and recommendations below describe that checkpoint.
The subsequent **Version support update** supersedes the original version and project-editing
restrictions: Pi 0.99.2 and 1.0.0 now pass the executed native matrix. Current implementation
and verification are in [`implementation-report.md`](implementation-report.md).

## Evidence base

Every claim below carries one of these labels:

| Label | Meaning |
|---|---|
| **Executed (0.99.2)** | Asserted by `scripts/pi/phase0-contract.mjs` against Pi's own classes in the devcontainer. A failing assertion exits non-zero |
| **Source** | Read in 0.99.2 `dist/`; not executed. Line numbers refer to 0.99.2 |
| **Docs** | Pi's installed `docs/packages.md`, `docs/security.md`, `CHANGELOG.md` |
| **Compared (1.0.0)** | 1.0.0 tarball files diffed against 0.99.2. Not executed |

| Kind | What | Where |
|---|---|---|
| Executed | `@earendil-works/pi-coding-agent` **0.99.2** (devcontainer, Node v24.21.0): `DefaultPackageManager`, `SettingsManager`, `ProjectTrustStore`, `resolveProjectTrusted`, `hasTrustRequiringProjectResources` | `scripts/pi/phase0-contract.mjs` |
| Source | 0.99.2 `dist/` | `/home/developer/.local/agent-clis/lib/node_modules/@earendil-works/pi-coding-agent/` in the devcontainer |
| Compared | npm tarballs of 1.0.0 (current `latest`, 2026-10-01), 0.85.0, 0.80.5, 0.80.3, 0.79.0, fetched with `npm pack` into a `mktemp` directory in the container and deleted afterwards | — |
| Skillshare | Current adapter | `internal/plugin/package_config.go`, `accounts.go`, `targets.go` |

To reproduce inside the devcontainer:

```sh
docker exec "$CONTAINER" bash -lc 'cd /workspace && PI_ROOT=/home/developer/.local/agent-clis/lib/node_modules/@earendil-works/pi-coding-agent node scripts/pi/phase0-contract.mjs'
```

Last run (2026-10-03, by `scripts/pi/version-matrix.sh` for Pi 0.99.2 and 1.0.0, against both `dist/core` and the CLI bundle): **29 scenarios, 90 assertions passed, exit 0** for each; see Version support. An earlier run of 21 scenarios, 53 assertions, also exit 0, no `/tmp/pi-phase0-*` left. Scenarios 18–20 cover string→object conversion of a package with a partial manifest and its controls, and single-file sources; scenario 21 shows that a rule with an unpaired surrogate escape names no file in Pi, while Go would read it as U+FFFD.

How the script stays isolated:
- `HOME` and every agent/project directory live under one `mkdtemp` root, removed in `finally`.
- `PI_OFFLINE=1`, except scenario 12, which turns it off but answers `skip` to the install hook.
- It resolves paths only. It installs nothing and imports no extension; each fixture extension throws if imported.
- Trust is never forced. Scenarios write a decision into the fixture's own `<agentDir>/trust.json` through `ProjectTrustStore`, and `resolveProjectTrusted` (no UI, `defaultProjectTrust: "ask"`) decides. These trust files are test inputs only; they say nothing about how a real project is trusted.

Scenarios are cited below as **S1–S17** (the numbers the script prints).

### What was not executed

- Pi CLI commands (`pi`, `pi config`, `pi install`). The contract was exercised through Pi's library classes.
- Real npm/git installs and loading any extension module.
- Interactive trust prompts, `--approve`, and `project_trust` extension handlers.
- Any version other than 0.99.2. 1.0.0 was only compared (see Version support).
- The bundled CLI (`dist/bundle/`, which `bin.pi` points to). Only the unbundled `dist/core` modules were executed and compared.
- Pi forks (`omo`, `senpi`) used as account CLIs.
- Windows path handling.

## 1. Filter syntax and entry form

Sources: `docs/packages.md` § "Select package resources"; `package-manager.js` `applyPatterns` :548, `applyPackageFilter` :1903, `collectPackageResources` :1848.

| Rule | Status | Evidence |
|---|---|---|
| String entry loads everything the package declares | Executed | S1: a, b, c, skill and prompt on |
| `-path` excludes one exact path | Executed | S2 |
| Omitted resource key → all of that type; `[]` → none of that type | Executed | S3: `extensions: []` → all off, skill and prompt still on |
| Plain glob includes; `!glob` excludes | Executed | S4 |
| `+path` force-includes an exact path (beats `!`); `-path` force-excludes (beats `+`) | Executed | S5: `!*`, `+a`, `+b`, `-b` → a on, b off, c off |
| Paths are relative to the package root; minimatch against relative path, basename, or absolute path | Source | `matchesAnyPattern` :473, `matchesAnyExactPattern` :500 |
| Filters only narrow the package manifest (`pi` key in `package.json`) and never add undeclared files | Docs + Source | `collectManifestFiles` :1934 |
| Extension discovery: a directory with `package.json` `pi.extensions` or `index.ts`/`index.js` is one extension; otherwise top-level files, honoring `.gitignore`/`.ignore`/`.fdignore`, skipping dot entries and `node_modules` | Source | `collectAutoExtensionEntries` :408, `resolveExtensionEntries` :381 |
| Other resource filters, empty arrays and unknown keys survive a native write | Executed | S14: before the write, `skills: []` and `!prompts/commit.md` are in effect (both off). After `setPackages` + `flush`, the file deep-equals the original with only `extensions` replaced: `skills: []`, the prompt filter, `themes: []`, nested `x-acme` and top-level `x-top` unchanged |
| Settings are parsed as strict JSON | Executed | S17: a `//` comment → load error for scope `global`, and no packages are loaded from that file |

Identity (`getPackageIdentity` :1392; Executed, S13):
- npm: package name without version (`npm:@acme/tools@1.0.0` equals `npm:@acme/tools`).
- git: `host/path` without ref (`git:github.com/acme/tools@v1` and `https://github.com/acme/tools` both give `git:github.com/acme/tools`).
- local: absolute path resolved from the settings file's directory, **without** resolving symlinks (a symlink to the same package has a different identity).
- A bare `git@github.com:x/y.git` without `git:` gives a `local:` identity (Source: `isLocalPath`, `utils/paths.js:35`; `parseGitUrl`, `utils/git.js:150`).

Native per-resource toggle (Source: `config-selector.js` `togglePackageResource` :475):
- It converts a string entry to `{source}`.
- It removes earlier patterns whose target equals this resource's path.
- It appends `+path` or `-path`.
- When no resource key remains, it collapses the entry back to the bare `source` string. That collapse would drop unknown keys, so Skillshare must not copy it.

## 2. Scope precedence and inheritance

Sources: `docs/packages.md` § "Understand scope and identity"; `resolve` :704, `dedupePackages` :1412, `findAutoloadDeltaBase` :1059, `applyPackageDeltaFilter` :1919, `addResource` :2102 (first writer wins); `config-selector.js` `setProjectPackageOverride` :582, `createPackageOverrideSource` :674.

Packages are collected project-first, then user. The first entry to add a path decides it.

| Project entry for the same identity | Result | Status |
|---|---|---|
| None | Global entry decides | Executed (S1–S5) |
| `{source, autoload: false, extensions: [...]}` (**delta**) | Only the paths named by its patterns are decided by the project. Every other path is inherited from the global entry, including its filters. | Executed, S6: global `-c`, delta `+c -a` → a off, b on (inherited), c on; skill and prompt inherited |
| Delta but no global entry | Only the named paths are added; all other resources of every type are absent | Executed, S7: only `a`; skills and prompts empty |
| Object without `autoload: false` | Replaces the global entry completely. Global filters do not apply. | Executed, S8 |
| String | Same as above | Executed, S9 |
| Any entry in a project resolved as untrusted | Ignored; global decides | Executed, S10 (saved `false`), S15 (no decision, no UI) |

Inside a delta, later patterns override earlier ones for the same path (Source: `applyAutoloadDisabledPatterns` :594). `pi config` models exactly three states per resource in project scope (Source):
- **load:** `+path` in the delta.
- **unload:** `-path` in the delta.
- **inherit:** no pattern.

If the delta has no pattern left, `pi config` deletes it.

**Effective selection, as Pi computes it:** project delta patterns first, then the global entry's filter, then the manifest defaults. A replacing project entry ignores the global entry completely. An untrusted project contributes nothing. Runtime loading is separate and unknown to Skillshare.

For a project target this computation depends on trust (§4). Skillshare can compute what the two settings files configure, but not whether Pi will honor the project file, so that result is a *configured* selection, not Pi's effective selection.

**Account roots:** a Pi account target is a separate agent directory (`PI_CODING_AGENT_DIR`, Skillshare `accounts.go:21`). It has its own `settings.json`, `npm/`, `git/` and `trust.json`. Skillshare models accounts as global-only (`accounts.go:42` rejects `ProjectRoot`). Account selection therefore uses the same global-scope rules, with no inheritance.

## 3. Package locations and static discovery

Sources: `getManagedNpmInstallPath` :1769, `getNpmInstallPath` :1787, `getGitInstallRoot` :1806, `resolveManagedPath` :1824.

| Source | Install location (user scope) | Project scope |
|---|---|---|
| npm | `<agentDir>/npm/node_modules/<name>`. Falls back to a legacy global npm/pnpm root, which Pi finds by running `npm root -g` / `pnpm list -g` | `<cwd>/.pi/npm/node_modules/<name>` |
| git | `<agentDir>/git/<host>/<path>` | `<cwd>/.pi/git/<host>/<path>` |
| local | No copy; the path resolved from the settings file | same |
| project delta | Resolves through the **user** entry's source and install (`findAutoloadDeltaBase`) | — |

**Installed versus declared resources.** The declaration lives in settings; the files exist only after install. Online, `resolve()` passes a missing npm/git source to the caller's `onMissing` hook, which may install it (Executed, S12: both sources reached the hook, answered `skip`). Offline mode skips them without calling the hook (Executed, S11).

**Dependencies.** Pi installs npm/git package dependencies. Local packages are never installed or modified (Docs, "Declare dependencies").

**Safe static discovery.** Read `package.json`'s `pi` key and the conventional folders, then apply the rules above. Do this only for the managed paths and local paths; never run npm to find the legacy root. Discovery reads files only; extension factories run later in `resource-loader.js` (Source: `loadExtensionsCached`, :506).

## 4. Writes, trust, and loading

Sources: `settings-manager.js` `loadFromStorage` :233, `assertProjectTrustedForWrite` :388, `persistScopedSettings` :425; settings lock :108 (`proper-lockfile` 4.1.2 on the file path, which creates `settings.json.lock`; Pi writes in place with `writeFileSync`); `trust-manager.js` `hasTrustRequiringProjectResources` :151, trust store `<agentDir>/trust.json` :174; `project-trust.js` `resolveProjectTrusted` :17; `docs/security.md` § Project trust.

### Native behavior

| Question | Answer | Status |
|---|---|---|
| Does writing settings load code or install? | No. Writing is a file write. Install and load happen at the next Pi startup or reload. | Source |
| Does Pi write project settings when the project resolved as untrusted? | No. `setProjectPackages` throws `Project is not trusted; refusing to write project settings`; after `flush` the file is byte-for-byte unchanged. | Executed, S15 |
| How does Pi decide trust? | In this order, first answer wins: (1) `trustOverride` (`--approve` and similar); (2) **no trust-requiring resources under `.pi` → trusted, nothing asked**; (3) `project_trust` events from loaded extensions; (4) nearest saved decision in `<agentDir>/trust.json`; (5) `defaultProjectTrust` (`always`/`never`; `ask` falls through); (6) without UI → untrusted, with UI → prompt (which also offers session-only trust). | Source (`project-trust.js:17-58`); steps 2, 4 and 6 also Executed (S10, S15, S16) |
| What does "no project resources" mean? | Step 2 returns `true` only because there is nothing to protect yet. The same folder becomes trust-requiring as soon as `.pi/settings.json` exists, and then resolves untrusted without UI. | Executed, S16 |

### What Skillshare can and cannot know

- `trust.json` and `defaultProjectTrust` are readable files, but they are steps 4–5. Step 3 runs first and can answer either way, so `trust.json = true` or `defaultProjectTrust: "always"` alone may not match Pi's real verdict.
- Step 3 needs the user's extensions to be loaded and executed. Skillshare must never do that.
- Session-only decisions (from the prompt or from an extension result without `remember`) are not stored anywhere.
- Step 1 depends on how Pi is launched. Skillshare must never pass `--approve` or fabricate a trust decision.
- Conclusion: **Skillshare cannot obtain a reliable native trust verdict statically.**

### UI policy (stricter than native)

- A project target is read-only unless a reliable native trust verdict exists. Today none does, so **project apply is disabled**. The project tab stays visible; Apply is not offered.
- "No `.pi` yet" is not treated as trusted. Pi's step 2 describes an empty folder, not a permission to create project settings.
- `trust.json` and `defaultProjectTrust` may be shown as hints ("Pi saved: trusted"), never as a write gate.
- Global and account targets are unaffected: trust applies to project settings only.

## Version support

| Version | Delta (`findAutoloadDeltaBase`) | Evidence |
|---|---|---|
| 0.79.0, 0.80.3 | absent | Compared (tarball source) |
| 0.80.4 | — | Not published on npm. Its changelog entry introduces project-local overrides in `pi config` (Docs) |
| 0.80.5, 0.85.0 | present; files differ from 0.99.2 | Compared (tarball source) |
| **0.99.2** | present | **Executed** (17 scenarios, 40 assertions) |
| **1.0.0** | present | **Compared, not executed** (details below) |

The delta boundary is therefore "absent in 0.80.3, present in 0.80.5", based on these samples and the 0.80.4 changelog entry only. Versions between the samples were not inspected, and the presence of the code in 0.80.5–0.85.0 says nothing about whether its behavior matches 0.99.2.

1.0.0 compared to 0.99.2 (full `dist/` diff plus `package.json`):
- Byte-identical: `core/package-manager.js`, `core/trust-manager.js`, `core/project-trust.js`, `core/extensions/runner.js` (emits `project_trust`), `core/pi-manifest.js`, `modes/interactive/components/config-selector.js`, `utils/git.js`, `utils/paths.js`, `docs/packages.md`.
- `core/settings-manager.js` differs only in the getters for `quietStartup` and `tuiMode`. Load, lock, trust assertion and write code are identical.
- Dependencies used by writes and filters are pinned identically: `proper-lockfile` 4.1.2, `minimatch` 10.2.6.
- Other changed modules (`main.js`, `core/agent-session.js`, `modes/interactive/interactive-mode.js`) were searched for changed lines mentioning trust, locks, settings writes or packages. The only hit is `projectTrusted: false` in a new Radius MCP login helper that reads `mcp.json`, unrelated to packages.
- `dist/bundle/` (the actual `pi` binary) differs because it is rebuilt; it was not compared.

**What this proves.** The package/trust contract in Pi 0.99.2's library classes behaves as asserted. 1.0.0 ships the same source for that contract. Neither result proves that a Skillshare write path is correct: that needs its own tests in the implementation. Production editing support is **not** established by Phase 0.

**Proposed version gate** (for implementation review):
- Editable candidates: 0.99.2 (executed) and 1.0.0 (same contract source). Before enabling 1.0.0, rerun the script against an installed 1.0.0.
- Anything else, including unknown versions, gets a read-only Extensions tab. To widen support, rerun the script against that version.
- Below 0.80.5 there is no delta, so only "follow global" or "full project entry" exists there.

**Update (implementation, 2026-10-03).** The gate above is now backed by execution, not by
comparison. `scripts/pi/version-matrix.sh 0.99.2 1.0.0` installs each exact version with npm
under `/tmp/pi-versions/<version>` (own lockfile and cache, no install scripts, nothing
global) and runs four checks per version:

| Check | What it runs |
|---|---|
| `contract/core` | `phase0-contract.mjs` against `dist/core` |
| `contract/bundle` | the same script with `PI_ENTRY=bundle`, against `dist/bundle`, which the `pi` CLI executes |
| `native-lock` | `TestPiNativeLockHoldsAgainstPi`: Pi's proper-lockfile against Skillshare's lock |
| `project-native` | `TestPiProjectOverridesResolveInPi`: Pi's `DefaultPackageManager` (through `scripts/pi/resolve-probe.mjs`, trust passed in, so no trust store is read or written) resolves project settings that Skillshare's editor wrote |

Both versions passed all four, with 29 scenarios and 90 assertions each; scenario 28 verifies query-bearing Git identity and first-global/last-project precedence, and scenario 29 verifies that an invalid UTF-8 source can decode to and own a valid later local source. Neither installs or loads the packages. The integrity hashes
and results are in `scripts/pi/version-evidence.json`, and `PiVerifiedVersions` must equal the
versions listed there (a Go test enforces it; superseded by the minimum-version update below). Scenario 27 adds the project-only delta:
with no global entry, `+path` loads, `-path` and unnamed paths stay unloaded, and no other
resource of the package loads. The project trust decision below is superseded in one way:
a project's settings are now edited, as `pi config` writes them, while trust is still never
written or assumed.

**Update (minimum version, 2026-10-04).** The exact list became a minimum. Any plain `X.Y.Z`
at or above `PiMinVersion` (0.99.2, the oldest version with executed evidence) is editable;
older, prerelease or unparsable versions are read-only with the reason `unsupportedVersion`.
Pi 1.0.1 was added to the matrix (`version-matrix.sh 0.99.2 1.0.0 1.0.1`) and passed all four
checks. The Go test now requires `PiMinVersion` to be in the evidence and every recorded
version to pass. A later Pi that breaks the contract is no longer read-only by default; rerun
the matrix against new releases to catch it.

## Decisions for the design and the first delivery

1. **Downgrade A is not needed for the candidate versions.** Per-extension inheritance is native through the `autoload: false` delta. A would apply only to versions before 0.80.5, which are read-only anyway.
2. **Inheritance structure** (for projects this is display-only until the trust blocker is resolved; see decision 6):
   - **Delta or no project entry:** show the rows as they are now (inherit / override load / override unload). "Use pi (global)" removes the path's pattern. Remove the delta when it becomes empty.
   - **Replacing project entry** (string, or object without `autoload: false`): no inheritance. Show "This project replaces pi (global) for this package", hide "Use pi (global)", and edit that entry's filter directly. Turning it into a delta changes the meaning of the entry, so make it a separate explicit action, deferred.
   - **Package only in the project:** never create a delta (S7 would drop every skill and prompt). Edit the project's own entry.
3. **Wording and trust-dependent values.**
   - Global and account targets: "Pi uses now" becomes **Effective selection**.
   - Project targets: show **Configured selection (if Pi trusts this project)**, computed from both settings files. The trust-dependent **Effective selection is Unknown**. Never present the configured value as what Pi actually uses.
   - A row may say "pi (global) applies" only when Pi's untrust is known. Today Skillshare cannot know it statically either, because a `project_trust` handler can override even a saved `false`. Skillshare does not execute extensions or create trust to find out.
   - Runtime stays Unknown everywhere.
4. **What gets written.**
   - Only exact `+path` / `-path` entries in `extensions`, matching `pi config`, relative to the package root.
   - Leave globs, `!` rules, other resource keys (`skills`, `prompts`, `themes`, including `[]`), unknown entry keys and every other top-level key untouched.
   - Never collapse an object back to a string when other keys remain.
   - Rows affected only by a glob or `!` rule show that rule read-only. An exact toggle still works, because `+`/`-` win.
5. **Stale protection.**
   - The revision is the hash of the target file's raw bytes. For a project target, it also covers the global file's hash, because effective selection depends on both.
   - Hold Pi's own lock (create `settings.json.lock` the way `proper-lockfile` does) for the duration of the write, plus Skillshare's flock.
   - Re-read and compare the raw bytes before an atomic rename.
   - Refuse if anything changed.
   - Refuse to write a file that is not strict JSON, because Pi can't read it either.
6. **Project trust — unresolved implementation blocker for project apply** (coordinator decision A, 2026-10-03).
   - Project targets (`<project>@pi`) stay visible and read-only: inventory, provenance and configured selection are shown; Apply is disabled with "Skillshare can't confirm Pi trusts this project."
   - No write is allowed because `.pi` is absent, because `trust.json` says `true`, or because `defaultProjectTrust` is `always`.
   - No Skillshare-owned consent mechanism replaces Pi trust. Skillshare never executes extensions, never passes `--approve`, and never writes `trust.json`.
   - The earlier "trusted project can apply" flow is **not verified** and is not part of the first delivery. Narrowing the product scope further, or adding a trust bridge with Pi, is a later decision for the user.
   - **Superseded (user decision, 2026-10-03, #342 revision):** project settings are editable. Apply writes only `.pi/settings.json`, as `pi config` does, and never depends on or changes trust: Pi uses the result only if it trusts the project, which the UI says. The no-trust-write rules above still hold.
7. **Global and account targets are gated too.**
   - Editing is not blanket-enabled. A target is editable only when its own Pi version and CLI pass the version gate above. Versions below the minimum (see the minimum-version update) are read-only.
   - Accounts (`pi-work`): package install and remove stay as they are. Selection uses global-scope rules in the account's `settings.json`. Individual controls stay read-only until the account's own Pi version is verified and the account CLI is Pi itself. Fork CLIs (`omo`, `senpi`) are unverified, so read-only.
8. **Inventory.**
   - Static, from managed install paths and local paths only.
   - A declared but not yet installed npm/git package shows "Not installed yet — sync or run Pi" with its resources unknown.
   - Legacy global npm installs are not located; show them as unknown.

## Existing adapter notes (not fixed here)

- `piInventory` (`package_config.go:43`) reads only `source` from object entries. Edits must start from the raw entry. The current object is fine for listing.
- `piInventory` accepts `git@host:…` as a remote source (`package_config.go:65`), but Pi treats it as a local path. Report this as a separate follow-up.
- `readPackageConfig` (`package_config.go:153`) accepts JSONC through `hujson`. Pi does not. A Pi write path must parse strictly and refuse non-strict JSON; it must not emit JSONC through `hujson`.
- From the OpenCode write path (`applyOpenCode`, `package_config.go:234`) reuse only the safety pattern: no symlink, flock, raw-byte compare, atomic write. Do not reuse its parser or patch writer. Add Pi's own `settings.json.lock` on top.

## Project trust decision

Asked: how should the first delivery handle project targets without a reliable native trust verdict?
- **A.** Global and account editing only (each still version-gated); project targets read-only.
- **B.** Project Apply after a per-project confirmation in Skillshare (Skillshare consent, not Pi trust).
- **C.** Defer project targets entirely until Pi exposes a trust query.

Coordinator answer (2026-10-03): **A**. It applies the existing rule "unknown or unverified capability is read-only". B and C are not adopted. Project apply remains an open implementation blocker; any change to that scope is for the user to decide.

## Required design revisions

Revise the design prototype according to the decisions above:
- Effective selection wording for global and account targets.
- Project targets: read-only, Apply removed; "Configured selection (if Pi trusts this project)" beside an Unknown effective value; "pi (global) applies" only when untrust is known.
- Remove the prototype's "edit with a note" path for untrusted projects (README open decision 2).
- A replacing-entry state.
- No delta for project-only packages.
- A version gate that makes global, account and fork targets read-only when unverified.
- Glob rules shown read-only.

After that, start the first delivery on a new branch or worktree, with the scenarios in `scripts/pi/phase0-contract.mjs` as golden fixtures for Go tests.
