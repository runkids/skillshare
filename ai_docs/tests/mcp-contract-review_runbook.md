# MCP contract review security runbook

## Scope

Verify the unsigned `skillshare-contract-v2` slice with independently authored
inert fixtures: strict capture and complete pagination, directional changes,
bounded audit, credential refusal, exact configuration/root identity,
first observation without writes, revision-bound approval and stale/write
failure preservation. Cover CLI integration, the local-protected API and the
explicit dashboard confirmation/approval flow. Optional signing is covered by
the separate [ephemeral-key signing runbook](mcp-signing_runbook.md); this
runbook keeps the unsigned workflow coverage.

**Execution status: pending.** This runbook was authored from source only.
No step below has a recorded runtime result; expected assertions are test
criteria, not PASS claims. Website build is also pending.

The real CLI review/approve/change/strict/stale flow is authored in
`tests/integration/mcp_check_test.go: TestMCPCheckLiveToolPins` and the
self-contained [MCP check Step 15](mcp_check_runbook.md#step-15-first-review-writes-nothing-approval-binds-one-exact-revision).
That step additionally asserts that the first observation creates no state
file and that ordinary/strict/stale checks preserve approved bytes.
The package baseline tests check those no-write guarantees directly as well.

## Environment

Run product, build, test and frontend commands **only inside the supported
Linux devcontainer**, or the owner's existing authorized remote CI route.
Do not run them on the host. Do not install Docker, change shared mounts,
provision compute or use production MCP endpoints to execute this runbook.

Before execution, resolve the running container and verify that `/workspace`
contains the exact branch under review and that its `.git` resolves there.
If this worktree is not mounted, report that condition and use the authorized
remote route; never silently test a different checkout.

For manual steps, create a fresh ssenv without `--init`, enter it, and run
each Steps block in that isolated HOME. Go test fixtures create additional
`t.TempDir` sandboxes. Every block is self-contained: shell variables and
working directories do not persist between mdproof steps.

```sh
# Environment setup only; run via the owner's supported container route.
CONTAINER=$(docker compose -f .devcontainer/docker-compose.yml ps -q skillshare-devcontainer)
# Continue only when CONTAINER is nonempty and the worktree mount is verified.
docker exec "$CONTAINER" ssenv create mcp-contract-review
docker exec -it "$CONTAINER" ssenv enter mcp-contract-review
```

Use sequential focused checks and `GOMAXPROCS=2 GOFLAGS=-p=2`; no full suite
or broad gate is needed for this runbook. Go, the UI/website dependencies and
any test engine must already be available through the authorized environment.
Hegel is test-only in `tests/property`, outside the production module. Its
explicit latest-version command below may update that module's dependency
files; let the owner review those changes.

For mdproof, first inspect `ai_docs/tests/mdproof.json` and
`.mdproof/lessons-learned.md`. The project config builds and initializes
`/workspace` and has `isolation: per-runbook`; do not nest its automatic
isolation inside ssenv or use it against an unmounted alternate worktree.
Once that exact checkout/config is verified, use the already-installed mdproof
in the container directly. Use ssenv for manual debugging instead.

```sh
docker exec "$CONTAINER" env GOMAXPROCS=2 GOFLAGS=-p=2 \
  SKILLSHARE_DEV_ALLOW_WORKSPACE_PROJECT=1 \
  mdproof --report json /workspace/ai_docs/tests/mcp-contract-review_runbook.md
```

The project setup/teardown affects only its disposable HOME. The runbook
steps do not read native user data or contact external MCP servers. HTTP
fixtures use local `httptest` servers; stdio fixtures are authored inert
scripts, never reference-project attack fixtures. Hegel dependency retrieval
is package download, not an MCP probe.

## Steps

### Step 1: Strict capture, identity, bounded probes and audit

Run the owning MCP package and its related bounded-audit regressions. The MCP
suite covers number/array preservation, all hash dimensions, malformed JSON,
required input schema, legacy evidence, literal declaration refusal,
rotated credentials, exact root/config isolation, CAS/no-op behavior, cursor
cycles/page caps and redirects. Audit failures must not receive a revision.

```bash
set -eu
cd /workspace
env GOMAXPROCS=2 GOFLAGS=-p=2 go test -count=1 -timeout 5m ./internal/mcp
env GOMAXPROCS=2 GOFLAGS=-p=2 go test -count=1 -timeout 5m ./internal/audit -run '^TestScanContentBounded'
```

Expected:
- exit_code: 0
- regex: ok\s+skillshare/internal/mcp
- regex: ok\s+skillshare/internal/audit

### Step 2: CLI option refusal and real revision-bound acceptance

Build the reviewed CLI into an isolated fixture directory and point the
integration sandbox at that exact binary. This avoids a stale `/workspace/bin`
binary. The live fixture launches only the authored local stdio script;
unknown endpoints and native user configuration are not used.

The integration scenario reviews an untrusted inventory, approves its returned
revision, changes the description, observes drift, rejects strict drift and
an old revision, accepts the newly reviewed revision, then verifies unchanged
status and `operations.log`. Explicit CLI/API approval attempts also write a separate
`mcp contract review` entry to `audit.log` (`skillshare log --audit`), containing only
server/tool/audit-finding counts, accepted outcome and standard log metadata, without
names, contract text, revisions, diagnostics or credentials. The shared writer's
source tests are in `internal/mcpchecklog/audit_test.go`; their execution is pending.
Logging is best effort and cannot undo a persisted approval.
Package tests in Step 1 and MCP check Step 15
supply explicit no-write/byte-preservation assertions.

```bash
set -eu
cd /workspace
CASE=$(mktemp -d "$HOME/mcp-contract-cli.XXXXXX")
trap 'rm -rf "$CASE"' EXIT
env GOMAXPROCS=2 GOFLAGS=-p=2 go build -o "$CASE/skillshare" ./cmd/skillshare
env GOMAXPROCS=2 GOFLAGS=-p=2 go test -count=1 -timeout 5m ./cmd/skillshare \
  -run '^(TestMCPCheckApprovalRequiresReviewedRevisionBeforeProbing|TestLogMCPAcceptTools.*)$'
env GOMAXPROCS=2 GOFLAGS=-p=2 SKILLSHARE_TEST_BINARY="$CASE/skillshare" \
  go test -count=1 -timeout 5m ./tests/integration -run '^TestMCPCheckLiveToolPins$'
```

Expected:
- exit_code: 0
- regex: ok\s+skillshare/cmd/skillshare
- regex: ok\s+skillshare/tests/integration

### Step 3: Read-only API and typed exact-scope approval

Use in-process API tests; do not start a dashboard daemon or bind a production
port. Cover omitted versus empty versus exact root, strict request parsing,
local MCP protection, no locks held across probes, stale 409, refusal without
success, exact numeric `definitionText`, empty inventories, project mode and
cross-root revision refusal. HTTP 200/accepted must follow persisted state,
not a diagnostic string.

```bash
set -eu
cd /workspace
env GOMAXPROCS=2 GOFLAGS=-p=2 go test -count=1 -timeout 5m ./internal/server \
  -run '^TestMCP(Check|Accept|Probe)'
```

Expected:
- exit_code: 0
- regex: ok\s+skillshare/internal/server

### Step 4: Dashboard confirmation, snapshots and stale behavior

The API client and React tests mock all requests. Opening Review live tools
must not probe; Probe live tools must be explicit. Full snapshots stay inert,
formatting controls stay visible and metadata links/icons stay inactive.
Trust this inventory sends exact root/name/revision; stale or failed approval
and a completed approval require a new probe before another approval. Static
Check and editor Load tools retain their distinct behavior.

```bash
set -eu
cd /workspace/ui
npm run test -- src/api/mcpCheck.test.ts src/components/mcp/MCPContractReview.test.tsx
npm run build
```

Expected:
- exit_code: 0

### Step 5: Latest Hegel contract properties

The separate module exercises exported `Service.Check` against local inert
HTTP fixtures. It checks equivalent spelling/exact numeric distinctions and
stale approval preservation. Refresh the test-only dependency to latest;
never add its engine to the production Go module or infer that passing a
property proves all malicious server behavior safe.

```bash
set -eu
cd /workspace/tests/property
env GOMAXPROCS=2 GOFLAGS=-p=2 go get hegel.dev/go/hegel@latest
env GOMAXPROCS=2 GOFLAGS=-p=2 go test -mod=mod -count=1 -timeout 5m ./...
```

Expected:
- exit_code: 0
- regex: ok\s+skillshare/tests/property

### Step 6: Documentation links and website build

Validate the English command reference, four existing translated copies and
built-in MCP reference, including anchors and the revised first-trust flow.
This is the only website build step; its outcome remains pending until run.

```bash
set -eu
cd /workspace
python3 scripts/ai-context.py check
cd /workspace/website
npm run build
```

Expected:
- exit_code: 0

## Pass Criteria

- Record actual results for every step; pending/skipped/error is never PASS.
- First observations/API GETs never establish trust; legacy evidence needs
  explicit review. Approval requires one exact server/root and current revision.
- Strict untrusted/legacy/drift and invalid/incomplete/audit/store/write
  failures fail. Stale/config/store races cannot replace an approved baseline.
- Full contract fields, exact numbers and all array order remain covered.
  UNKNOWN and description/notation changes never become automatic approval.
- Configured credentials do not reach snapshots, hashes, revisions or logs;
  failed stdio stderr payloads are omitted. Explicit approval audit entries contain
  only counts/outcome and standard log metadata. No arbitrary-secret detection or
  runtime quarantine claim is made.
- Dashboard opens inertly, only explicit probing starts servers, and typed
  persisted approval is the only accepted outcome. Re-probe after stale/failure.
- Website build and locale consistency checks have actual results before
  claiming documentation build success. Signed-store coverage is in the separate
  signing runbook.

## Cleanup and report

The CLI step removes its own exact mktemp directory on exit; Go fixtures
clean their temporary directories and close their local fixture servers.
No persistent daemon should be started. After manual ssenv use, leave the
shell and delete only the named disposable environment, after confirming it
is no longer needed. Never delete a real baseline as a repair action.

Report the reviewed SHA, environment, exact commands, actual results,
missing prerequisites, and any Hegel dependency-file changes to the owner.
Do not wait for nonexistent CI runs. No execution result is recorded here.
