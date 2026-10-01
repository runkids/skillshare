# Codex Plugin Update Runbook

## Scope

Verify global managed and imported Codex local/Git updates, disabled enablement,
unchanged shared-marketplace caches, native policy rejection and recovery after
marketplace refresh. The native fixtures never request model output.

## Environment

Run inside the repository devcontainer with Codex CLI `0.159.3` and Git on PATH.
Create a fresh `ssenv` for this runbook. Each Go case additionally creates its own
temporary HOME and CODEX_HOME; it uses no live native configuration or credentials.

## Steps

### 1. Verify the native CLI

```bash
codex --version
```

**Expected**

- `codex-cli 0.159.3`

### 2. Run the native update contracts

```bash
cd /workspace
SKILLSHARE_CODEX_UPDATE_E2E=1 go test ./internal/plugin -run '^TestCodexUpdateNativeE2E$' -count=1 -v
```

**Expected**

- `--- PASS: TestCodexUpdateNativeE2E`
- `--- PASS: TestCodexUpdateNativeE2E/local`
- `--- PASS: TestCodexUpdateNativeE2E/git`
- `--- PASS: TestCodexUpdateNativeE2E/managed`
- `--- PASS: TestCodexUpdateNativeE2E/git-rollback`
- `--- PASS: TestCodexUpdateNativeE2E/managed-policy`
- `--- PASS: TestCodexUpdateNativeE2E/git-ref-race`

### 3. Run stale-evidence and recovery regressions

```bash
cd /workspace
go test ./internal/plugin -run 'CodexUpdateTransaction|CodexUpdatePreview|CodexConfig' -count=1 -v
```

**Expected**

- `--- PASS: TestCodexUpdateTransaction`
- `--- PASS: TestCodexUpdatePreviewRejectsNativeInlineRewrite`
- `--- PASS: TestCodexConfigRejectsUnknownEnabled`
- `--- PASS: TestCodexConfigPreservesTrailingArrayTable`
- `--- PASS: FuzzCodexConfigPreservesBytes`
- `--- PASS: FuzzCodexConfigRejectsInline`

## Pass Criteria

All steps exit successfully. The native cases verify the installed version and
disabled state, exact original config bytes, and unrelated cache contents. Failure
cases verify the old working install is restored and conflicting external edits
survive. Completion verifies native installation, not Agent runtime activation.
