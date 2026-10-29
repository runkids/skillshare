# CLI E2E Runbook: Native plugin lifecycle

Validate complete plugin management and sync selection without touching real user installs.

**Origin**: Plugin support — shared CLI/TUI/UI service and native adapters.

## Scope

- Source discovery, preview cancellation, stale preview rejection, partial failure/retry.
- Claude and Codex native install, exclusion, reinstallation, removal.
- Claude native update and project-scoped install/removal.
- Native enabled state is independent of Skillshare sync selection.

## Environment

Run in the devcontainer. The Go fixtures create isolated HOME, XDG, CODEX_HOME,
and CLAUDE_CONFIG_DIR directories and remove them through `t.TempDir` cleanup.
They contain only a greeting skill, with no hooks or MCP servers.

The optional native suite requires Claude Code 2.1.276 and Codex CLI 0.154.0 on
PATH. For local validation these may be installed under
`/tmp/skillshare-plugin-native-tools`, not globally. It deliberately skips unless
`SKILLSHARE_PLUGIN_NATIVE_E2E=1` is set; a skipped native suite is not a pass.

## Steps

### 1. Core contracts and CLI/API routing

```bash
cd /workspace
go test ./internal/plugin ./cmd/skillshare ./internal/server -run 'TestPluginOptions|TestPluginAPI|TestSyncSelection|TestDiscoverPreserves|TestNativeInventory|TestPreviewCancel|TestProjectCodex' -count=1
```

**Expected**:
- Exit code 0.
- All three packages pass.

### 2. Real native lifecycle

```bash
cd /workspace
export PATH=/tmp/skillshare-plugin-native-tools/node_modules/.bin:$PATH
codex --version
claude --version
SKILLSHARE_PLUGIN_NATIVE_E2E=1 go test ./internal/plugin -run TestPluginNativeLifecycle -count=1 -v
```

**Expected**:
- Exit code 0.
- `--- PASS: TestPluginNativeLifecycle`
- No changes outside the tests' temporary homes and fixture directories.

### 3. Frontend contracts

```bash
cd /workspace/ui
pnpm exec vitest run src/pages/PluginsPage.test.tsx src/i18n/i18n.test.ts
```

**Expected**:
- Exit code 0.
- Sync checkbox remains independent of native enabled state.
- Applying sync requires a preview; partial results stay visible.

### 4. Additional target lifecycle

```bash
cd /workspace
go test ./internal/plugin -run 'TestAdditionalPlugin|TestLocalPlugin|TestAntigravity|TestOpenCode|TestPiFiltered' -count=1
PATH=/tmp/skillshare-plugin-extra-tools/node_modules/.bin:$PATH SKILLSHARE_PLUGIN_EXTRA_E2E=1 go test ./internal/plugin -run TestAdditionalNativeLifecycle -count=1 -v
cd ui
pnpm exec vitest run src/components/plugins/PluginAddDialog.test.tsx
```

**Expected**:
- Whole Cursor/Antigravity bundles survive add/update/reselection; local edits are preserved.
- Antigravity project sync does not touch global paths; `agy` alias resolves correctly.
- Pi 0.85.1 real install/remove normalizes native relative source paths.
- OpenCode 1.18.31 registration lifecycle preserves unrelated JSONC entries/comments.
- OpenCode v2 is fixture-tested only; no claim of tested v2 runtime loading.
- UI lists all six targets, with no Gemini CLI target; project restrictions apply.
- Native runs use temporary HOME/XDG/PI_CODING_AGENT_DIR; no model calls.

## Pass Criteria

All steps pass, including the opt-in native test (not skipped). Installing source
files is not evidence that an Agent has loaded a skill or trusted a hook; those
runtime activation claims are outside this test.


### 5. Source compatibility and new native target

```bash
cd /workspace
go test ./internal/plugin -run 'TestSource|TestUnsafe|TestManifest|TestBrokenCatalog|TestExplicitOpenCode|TestOpenCodeProject|TestNativePackage|TestFailedUpdate|TestDroidPluginTargetNotSupported' -count=1
PATH=/tmp/skillshare-plugin-clients/node_modules/.bin:$PATH SKILLSHARE_PLUGIN_EXPANDED_E2E=1 go test ./internal/plugin -run TestExpandedNativeLifecycle -count=1 -v
```

**Expected**:
- Safe internal file/directory links survive snapshots; unsafe links are rejected.
- Multiple catalogs and per-target metadata preserve valid formats.
- Copilot CLI 1.0.86 completes add, deselect/sync, reselect/sync, update, and removal
  in temporary HOME. A skipped native test is not a pass.
- Antigravity CLI uses `agy`, separate from the desktop alias. Native 1.2.6
  install/list/remove was checked separately with isolated HOME/XDG directories.
- Kimi/Hermes/Devin remain discovery-only. Grok install/update requires native trust.
- Factory Droid is not offered as a plugin target.
- OpenCode project imported update never runs a global native command.
- A failed native update restores the previous managed source snapshot.

### 6. Dashboard compatibility

```bash
cd /workspace/ui
pnpm exec vitest run src/components/plugins/PluginAddDialog.test.tsx src/pages/PluginsPage.test.tsx src/i18n/i18n.test.ts
```

Check Clean and Playful in light/dark: advanced source options, per-target reasons,
registered/loading-unverified state, and identical Antigravity Desktop/CLI logos.

### 4. Two-machine shared declarations and private receipts

These deterministic fixtures simulate two machines with separate native homes,
config paths and state directories. Only config bytes pass between them. They
exercise global Codex/Cursor/Antigravity and project Claude/Antigravity, plus Pi
source and import flows:

1. A adds a source; B receives only its declarations and previews its own install.
2. B applies and repeats sync without changing shared YAML; A removes the package.
3. B receives the changed declarations, previews local cleanup, applies it, and
   confirms the next sync is empty.
4. B imports a native ID into A's source declaration. The shared recipe remains
   byte-identical and A keeps its reviewed source installation.
5. Each machine reads legacy YAML before a remote v2 edit. Its saved hints survive
   restart; a failed legacy target or whole-package removal is retried by sync.

```bash
cd /workspace
GOMAXPROCS=4 go test -p 4 ./internal/plugin -run 'TestSharedSourceLifecycleAcrossHostsAndScopes|TestPluginHostStateFirstNativeImportPreservesSourceIntent|TestLegacyReadPersistsEvidenceBeforeRemoteMigration|TestLegacyPendingRemovalRetriesAfterMigration' -count=1 -v
```

**Expected**:
- Exit code 0.
- `--- PASS: TestSharedSourceLifecycleAcrossHostsAndScopes`
- `--- PASS: TestPluginHostStateFirstNativeImportPreservesSourceIntent`
- `--- PASS: TestLegacyReadPersistsEvidenceBeforeRemoteMigration`
- `--- PASS: TestLegacyPendingRemovalRetriesAfterMigration`

### 5. Second-machine import guidance in doctor

A second machine receives an imported declaration without the first machine's
native ID. Doctor must warn about the skipped import, leaving shared YAML intact.

```bash
cd /workspace
GOMAXPROCS=4 go test -p 4 ./tests/integration -run 'TestDoctor_JSON_PluginMissingCLIIsWarning/second-machine-import' -count=1 -v
```

**Expected**:
- Exit code 0.
- `--- PASS: TestDoctor_JSON_PluginMissingCLIIsWarning/second-machine-import`

## Two-machine pass criteria

- Receipts, pending work and native IDs stay machine-local.
- An import into an existing source declaration does not convert shared intent.
- Shared removal cleans up only the receiving machine's proven installation.
- Before a real first v2 edit, every writer is upgraded and **each machine** has
  run `skillshare plugin list` while the shared YAML is still legacy.
- Review and commit pinning are per machine. Branch refs resolve at that machine's
  install/update time, including non-interactive sync. A commit SHA in `source_ref`
  is required to pin all machines to the same revision.
