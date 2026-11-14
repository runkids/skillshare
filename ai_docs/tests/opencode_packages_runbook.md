# OpenCode directory package regression

## Scope

Verify issue #913 with synthetic packages and version probes. No OpenCode session,
plugin code, network installation, real user configuration or security setting is used.

## Environment

Use the devcontainer and this branch's mounted worktree. All tests create isolated
homes/state. Do not run these commands on the host. This runbook was authored but
not executed when the change was prepared because no devcontainer was available.

## Steps

### 1. Native adapter lifecycle

```bash
cd /workspace
go test ./internal/opencode ./internal/hooks ./internal/plugin -run 'Test(PackageLoaderInputs|OpenCode)' -count=1
```

**Expected**

- Exit code: 0
- Output contains: `ok`

### 2. Reporter-facing CLI file input

```bash
cd /workspace
go test ./tests/integration -run '^TestHooksOpenCodeV2FileInput$' -count=1
```

**Expected**

- Exit code: 0
- Output contains: `ok`

### 3. Dashboard file-map round trip

```bash
cd /workspace/ui
pnpm exec vitest run src/components/hooks/hooksView.test.ts
```

**Expected**

- Exit code: 0
- Output contains: `passed`

## Pass Criteria

All steps pass. Tests verify directory URLs and preserved manifests, no-op resync,
updates/removal, v1 compatibility, stale-version previews, JSONC preservation and
external-edit conflicts. The CLI fixture rejects any probe other than --version;
service tests reject native update/install commands. A passing registration test
is not evidence that a real OpenCode runtime loaded or executed a plugin.
