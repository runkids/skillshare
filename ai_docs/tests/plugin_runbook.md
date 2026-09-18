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
