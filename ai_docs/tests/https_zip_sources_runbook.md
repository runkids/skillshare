# CLI E2E Runbook: HTTPS ZIP Sources

Validates install, byte-based check/update and local-edit protection using the
real CLI with a trusted local HTTPS fixture; no external source is accessed.

**Origin**: Issue #883 — HTTPS ZIP source support.

## Scope

- Global and project installation with a custom name and group
- Unchanged archive reported as up to date; changed bytes detected and installed
- Local edits preserved until explicit `update --force` restores upstream content

## Environment

Run inside the devcontainer through mdproof's per-runbook HOME isolation.
The integration test creates additional `testutil.Sandbox` homes, explicitly
trusts only its fixture certificate and closes its local server on every exit.
The project mdproof build hook supplies `/workspace/bin/skillshare`.

## Steps

### 1. Exercise the real CLI in both modes

```bash
cd /workspace
SKILLSHARE_TEST_BINARY=/workspace/bin/skillshare GOMAXPROCS=2 GOFLAGS=-p=2 \
  go test ./tests/integration -run '^TestArchiveCLIEndToEnd$' -v -count=1
```

Expected:
- exit_code: 0
- --- PASS: TestArchiveCLIEndToEnd/-g
- --- PASS: TestArchiveCLIEndToEnd/-p

## Pass Criteria

- Both global and project subtests pass against the current binary
- Temporary homes, fixture certificates and local HTTPS servers are cleaned up
