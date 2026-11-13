# MCP signed baseline runbook

## Scope

Verify externally anchored Ed25519 baseline integrity with an independently
authored inert stdio fixture. Cover first signed trust, correct/wrong/missing
verifiers, signer refusal, malformed/tampered state, byte-stable no-op,
revision-bound unsigned migration and independent re-review of all downgraded
records. This complements the [unsigned contract-review runbook](mcp-contract-review_runbook.md)
and [security contract](../../internal/mcp/CONTRACT_SECURITY.md).

**Execution status: unrun.** Source inspection and shell syntax checks are not
runtime evidence. Expected sections are acceptance criteria, not PASS claims.
Compile, runtime/API/dashboard checks and the website build remain pending.

## Environment

Run only in the supported Linux devcontainer or authorized remote test route,
with this exact reviewed branch mounted at `/workspace`. Use an already-built
`/workspace/bin/skillshare` from the reviewed source; record its commit/build
provenance before execution. Do not install infrastructure or test another
checkout. Prerequisites: Bash, ssenv, Python 3, OpenSSL with Ed25519 support,
and jq. No network endpoint, account, credential or real-user key is needed.

The single self-contained step creates ssenv without `--init` under a unique
temporary root, supplies synthetic HOME/XDG/config paths, removes inherited
native-account overrides in the consuming child and creates **EPHEMERAL TEST
KEYS ONLY inside that root**. It never signs with a real-user key. The fixture
only answers discovery and tool-list requests; it does not execute tools,
fetch metadata or open a listener. The CLI owns and closes each fixture child.

`ai_docs/tests/mdproof.json` has automatic build/setup/teardown commands and
per-runbook HOME isolation. Do not run that default pipeline as part of a
source-only review. For later manual execution, run this entire block once
inside the supported environment; it owns its ssenv isolation and cleanup.
Do not separately initialize or wrap it in another ssenv. A future mdproof
runner must avoid the default build/setup/teardown and select the same exact
reviewed binary. Shell variables do not persist between separate steps.

## Steps

### Step 1: Ephemeral keys, signed review and explicit unsigned migration

The script intentionally modifies only its disposable fixture store to check
integrity refusals, then restores its own known fixture bytes. Restoring a
fixture is not an instruction to delete, strip or replace real trust state.
Each refusal checks nonzero exit, JSON errors, absence of approval revisions
and byte preservation. No block below has been executed during authoring.

```bash
set -euo pipefail
MCP_SIGNING_BIN=/workspace/bin/skillshare
test -x "$MCP_SIGNING_BIN"
export MCP_SIGNING_BIN
SIGNING_TEST_ROOT=$(mktemp -d /tmp/mcp-signing.XXXXXX)
trap 'test -n "$SIGNING_TEST_ROOT" && test -d "$SIGNING_TEST_ROOT" && rm -rf -- "$SIGNING_TEST_ROOT"' EXIT
export SSENV_ROOT="$SIGNING_TEST_ROOT/ssenv"
ssenv create signing-fixture >/dev/null
ssenv enter signing-fixture -- env \
  -u SKILLSHARE_CONFIG -u CODEX_HOME -u CLAUDE_CONFIG_DIR \
  -u PI_CODING_AGENT_DIR -u OPENCODE_CONFIG_DIR -u OPENCODE_CONFIG \
  bash <<'SCENARIO'
set -euo pipefail
umask 077
cd "$HOME"
CASE=$(mktemp -d "$HOME/signing-case.XXXXXX")
export TMPDIR="$CASE/tmp"
mkdir -p "$TMPDIR" "$CASE/skills"
export SKILLSHARE_CONFIG="$CASE/config.yaml"
PINS="$XDG_STATE_HOME/skillshare/mcp/tool-pins.json"

# All generated keys are disposable test material in CASE, never user keys.
for pair in anchor other; do
  openssl genpkey -algorithm ED25519 -out "$CASE/$pair-private.pem" 2>/dev/null
  openssl pkey -in "$CASE/$pair-private.pem" -pubout -out "$CASE/$pair-public.pem" 2>/dev/null
done
PUB="$CASE/anchor-public.pem"
PRIV="$CASE/anchor-private.pem"

cat > "$CASE/server.py" <<'FIXTURE'
import json
import sys

for line in sys.stdin:
    request = json.loads(line)
    if "id" not in request:
        continue
    method = request.get("method")
    if method == "server/discover":
        result = {
            "resultType": "complete", "supportedVersions": ["2026-07-28"],
            "capabilities": {"tools": {}},
            "_meta": {"io.modelcontextprotocol/serverInfo": {
                "name": "signing-fixture", "version": "1.0.0"}},
        }
    elif method == "tools/list":
        result = {"resultType": "complete", "tools": [{
            "name": "inspect", "description": "Returns fixture metadata.",
            "inputSchema": {"type": "object"},
        }]}
    else:
        continue
    print(json.dumps({"jsonrpc": "2.0", "id": request["id"], "result": result}), flush=True)
FIXTURE
cat > "$SKILLSHARE_CONFIG" <<CONFIG
source: $CASE/skills
targets: {}
mcp:
  servers:
    alpha:
      command: python3
      args: [$CASE/server.py]
      targets: []
    beta:
      command: python3
      args: [$CASE/server.py]
      targets: []
CONFIG

check() {
  local name=$1
  shift
  "$MCP_SIGNING_BIN" mcp check "$name" --live --root "" --no-dns --json -g "$@"
}
revision() { jq -er '.servers[0].contract.revision | select(test("^[0-9a-f]{64}$"))' "$1"; }
refuse() {
  local label=$1 name=$2
  shift 2
  if check "$name" "$@" > "$CASE/$label.json"; then
    echo "expected refusal: $label" >&2
    exit 1
  fi
  jq -e '.summary.errors > 0 and .toolsAcceptance != "accepted" and
    all(.servers[]; (.contract.revision // "") == "")' "$CASE/$label.json" >/dev/null
}

# A missing store permits a protected preview but not unsigned migration.
check alpha --verify-key "$PUB" > "$CASE/first.json"
test ! -e "$PINS"
jq -e '.servers[0].contract.status == "untrusted"' "$CASE/first.json" >/dev/null
FIRST_REV=$(revision "$CASE/first.json")
refuse missing-migration alpha --verify-key "$PUB" --sign-unsigned
refuse missing-signer alpha --verify-key "$PUB" --accept-tools --revision "$FIRST_REV"
test ! -e "$PINS"
check alpha --verify-key "$PUB" --signing-key "$PRIV" \
  --accept-tools --revision "$FIRST_REV" > "$CASE/first-accepted.json"
jq -e '.toolsAcceptance == "accepted"' "$CASE/first-accepted.json" >/dev/null
jq -e '.version == 2 and .signature.algorithm == "Ed25519" and
  (.signature.value | type == "string")' "$PINS" >/dev/null
cp "$PINS" "$CASE/signed-baseline"

# Reconstruct this ASCII fixture's native unsigned envelope and verify the
# signature independently. Production callers must use the native serializer.
python3 - "$PINS" "$CASE" <<'PREIMAGE'
import base64
import json
import pathlib
import sys

store = json.loads(pathlib.Path(sys.argv[1]).read_text())
signature = store.pop("signature")
case = pathlib.Path(sys.argv[2])
payload = b"skillshare-mcp-pins-v2\n" + json.dumps(store, separators=(",", ":")).encode()
(case / "payload").write_bytes(payload)
(case / "signature").write_bytes(base64.b64decode(signature["value"], validate=True))
PREIMAGE
openssl pkeyutl -verify -rawin -pubin -inkey "$PUB" \
  -in "$CASE/payload" -sigfile "$CASE/signature" > "$CASE/external-verification.txt"

check alpha --verify-key "$PUB" --strict-tools > "$CASE/unchanged.json"
jq -e '.summary.errors == 0 and .servers[0].contract.status == "unchanged"' "$CASE/unchanged.json" >/dev/null
NOOP_REV=$(revision "$CASE/unchanged.json")
check alpha --verify-key "$PUB" --signing-key "$PRIV" \
  --accept-tools --revision "$NOOP_REV" > "$CASE/noop.json"
jq -e '.toolsAcceptance == "accepted"' "$CASE/noop.json" >/dev/null
cmp -s "$PINS" "$CASE/signed-baseline"

refuse missing-verifier alpha
refuse wrong-verifier alpha --verify-key "$CASE/other-public.pem"
refuse mismatched-signer alpha --verify-key "$PUB" --signing-key "$CASE/other-private.pem" \
  --accept-tools --revision "$NOOP_REV"
refuse signed-migration alpha --verify-key "$PUB" --sign-unsigned
cmp -s "$PINS" "$CASE/signed-baseline"
jq -e '.servers[0].findings | any(.message | contains("requires CLI verification with --verify-key"))' \
  "$CASE/missing-verifier.json" >/dev/null

# Fixed generic key errors; private Unix permissions must fail, even as root.
chmod 0644 "$PRIV"
refuse private-mode alpha --verify-key "$PUB" --signing-key "$PRIV" \
  --accept-tools --revision "$NOOP_REV"
chmod 0600 "$PRIV"
jq -e '.servers[0].findings | any(.message | contains("MCP baseline key is invalid or unreadable"))' \
  "$CASE/private-mode.json" >/dev/null
printf '%s\n' 'synthetic invalid PEM' > "$CASE/invalid.pem"
refuse invalid-key alpha --verify-key "$CASE/invalid.pem"
cmp -s "$PINS" "$CASE/signed-baseline"

# Tampering is limited to our fixture. Corruption never becomes migration.
for variant in tampered unknown-algorithm malformed unknown-field; do
  case "$variant" in
    tampered) jq -c '.records |= with_entries(.value.policy = "tampered")' "$CASE/signed-baseline" > "$PINS" ;;
    unknown-algorithm) jq -c '.signature.algorithm = "unknown"' "$CASE/signed-baseline" > "$PINS" ;;
    malformed) printf '%s\n' '{' > "$PINS" ;;
    unknown-field) jq -c '.unexpected = true' "$CASE/signed-baseline" > "$PINS" ;;
  esac
  cp "$PINS" "$CASE/corrupt-before"
  refuse "$variant" alpha --verify-key "$PUB"
  refuse "$variant-migration" alpha --verify-key "$PUB" --sign-unsigned
  cmp -s "$PINS" "$CASE/corrupt-before"
done
jq -e '.servers[0].findings | any(.message | contains("signature could not be verified"))' \
  "$CASE/tampered.json" >/dev/null
jq -c 'del(.signature)' "$CASE/signed-baseline" > "$PINS"
cp "$PINS" "$CASE/stripped-before"
refuse stripped alpha --verify-key "$PUB"
cmp -s "$PINS" "$CASE/stripped-before"
cp "$CASE/signed-baseline" "$PINS"

# Key paths/PEM bytes must not appear in state, reports or operation/audit logs.
python3 - "$CASE" "$XDG_STATE_HOME" "$SKILLSHARE_CONFIG" <<'PRIVACY'
import pathlib
import sys

case, state, config = map(pathlib.Path, sys.argv[1:])
keys = list(case.glob("*.pem"))
forbidden = [str(p).encode() for p in keys] + [p.read_bytes() for p in keys]
forbidden += [b"".join(p.read_bytes().splitlines()[1:-1]) for p in keys
              if p.read_bytes().startswith(b"-----BEGIN ")]
outputs = list(case.glob("*.json")) + [config]
outputs += [p for p in state.rglob("*") if p.is_file()]
outputs += list(case.rglob("*.log"))
for output in outputs:
    data = output.read_bytes()
    if any(value in data for value in forbidden):
        raise SystemExit("key path or material leaked to fixture output")
PRIVACY

# New state directory, same synthetic configuration: two unsigned approvals.
export XDG_STATE_HOME="$CASE/unsigned-state"
PINS="$XDG_STATE_HOME/skillshare/mcp/tool-pins.json"
mkdir -p "$XDG_STATE_HOME"
for name in alpha beta; do
  check "$name" > "$CASE/unsigned-$name.json"
  REV=$(revision "$CASE/unsigned-$name.json")
  check "$name" --accept-tools --revision "$REV" > "$CASE/unsigned-$name-accepted.json"
  jq -e '.toolsAcceptance == "accepted"' "$CASE/unsigned-$name-accepted.json" >/dev/null
done
cp "$PINS" "$CASE/unsigned-baseline"
check alpha > "$CASE/plain-review.json"
PLAIN_REV=$(revision "$CASE/plain-review.json")
refuse unsigned-protected alpha --verify-key "$PUB"
check alpha --verify-key "$PUB" --sign-unsigned > "$CASE/migration-review.json"
jq -e '.servers[0].contract.status == "legacy" and .servers[0].contract.identityChanged == true' \
  "$CASE/migration-review.json" >/dev/null
MIGRATION_REV=$(revision "$CASE/migration-review.json")
test "$MIGRATION_REV" != "$PLAIN_REV"
if check alpha --verify-key "$PUB" --sign-unsigned --strict-tools > "$CASE/migration-strict.json"; then
  echo 'migration preview must require review before signed acceptance' >&2
  exit 1
fi
jq -e '.summary.errors > 0 and .servers[0].contract.status == "legacy"' \
  "$CASE/migration-strict.json" >/dev/null
cmp -s "$PINS" "$CASE/unsigned-baseline"
refuse old-mode-revision alpha --verify-key "$PUB" --signing-key "$PRIV" \
  --sign-unsigned --accept-tools --revision "$PLAIN_REV"
jq -e '.toolsAcceptance == "stale"' "$CASE/old-mode-revision.json" >/dev/null
cmp -s "$PINS" "$CASE/unsigned-baseline"

# A fresh complete migration review authorizes only alpha as trusted.
check alpha --verify-key "$PUB" --signing-key "$PRIV" --sign-unsigned \
  --accept-tools --revision "$MIGRATION_REV" > "$CASE/migrated.json"
jq -e '.toolsAcceptance == "accepted"' "$CASE/migrated.json" >/dev/null
jq -e '.signature.algorithm == "Ed25519" and (.records | length == 2) and
  ([.records[] | select(.name == "alpha")][0].policy == "skillshare-contract-v2") and
  ([.records[] | select(.name == "beta")][0].policy == "legacy-unsigned")' "$PINS" >/dev/null
jq -e --slurpfile old "$CASE/unsigned-baseline" '
  .servers == $old[0].servers and
  ([.records[] | select(.name == "beta") | del(.policy)] ==
   [$old[0].records[] | select(.name == "beta") | del(.policy)])' "$PINS" >/dev/null
cp "$PINS" "$CASE/migrated-baseline"
check alpha --verify-key "$PUB" --strict-tools > "$CASE/migrated-alpha.json"
check beta --verify-key "$PUB" > "$CASE/beta-legacy.json"
jq -e '.servers[0].contract.status == "legacy"' "$CASE/beta-legacy.json" >/dev/null
if check beta --verify-key "$PUB" --strict-tools > "$CASE/beta-strict.json"; then
  echo 'downgraded evidence must fail strict review' >&2
  exit 1
fi
cmp -s "$PINS" "$CASE/migrated-baseline"
BETA_REV=$(revision "$CASE/beta-legacy.json")
check beta --verify-key "$PUB" --signing-key "$PRIV" \
  --accept-tools --revision "$BETA_REV" > "$CASE/beta-reaccepted.json"
jq -e '.toolsAcceptance == "accepted"' "$CASE/beta-reaccepted.json" >/dev/null
check beta --verify-key "$PUB" --strict-tools > "$CASE/beta-after.json"
jq -e '.summary.errors == 0 and .servers[0].contract.status == "unchanged"' "$CASE/beta-after.json" >/dev/null

# Only machine-readable summary goes to stdout if every prior assertion succeeds.
jq -n --slurpfile first "$CASE/first.json" --slurpfile noop "$CASE/noop.json" \
  --slurpfile migrated "$CASE/migrated.json" --slurpfile beta "$CASE/beta-after.json" \
  '{first: $first[0].servers[0].contract.status, noop: $noop[0].toolsAcceptance,
    migration: $migrated[0].toolsAcceptance, rereviewed: $beta[0].servers[0].contract.status}'
SCENARIO
```

Expected:
- exit_code: 0
- jq: .first == "untrusted" and .noop == "accepted"
- jq: .migration == "accepted" and .rereviewed == "unchanged"

## Pass Criteria

- Record actual binary provenance, environment, exit code and assertion results.
  An unrun, skipped, failed or missing-prerequisite check is never PASS.
- First protected observation writes nothing; signed approval requires matching
  keys and the reviewed revision. Invalid key/signature states never grant trust.
- Failed observations/approvals preserve fixture bytes; verified no-op preserves
  all bytes including signature. Malformed state cannot use migration as bypass.
- Migration binds mode and whole-store downgrade; all unselected V2 evidence
  remains untrusted until independent review and explicit signed reacceptance.
- No real key, user configuration or remote server is accessed. No key path or
  material appears in state/config/report/log output. Ephemeral keys are removed.

## Additional pending checks

The block above is not exhaustive. Keep these checks explicitly pending until
the owning source tests and supported integrations actually run:

- CLI flag misuse (without live, verifier or explicit approval), PEM type/size,
  symlink/replacement refusal, wrong PKIX/PKCS8 type and mismatched Ed25519 keys.
- Whole-store tampering in another root/config/name, duplicate/case-aliased
  fields, invalid signature length/base64, null signature, unsupported store
  versions, exact preimage/domain verification and legacy V1 preservation.
- Locked source/store changes, persistence failure and post-lock reread
  verification. Normal signed approval must preserve other reviewed records.
- Before probing, signed-store verifier/integrity errors must surface even for
  disabled, unreachable or authentication-required servers.
- API/dashboard have no key inputs; signed stores report CLI verification
  required and expose no approval revision/trust action. Typed HTTP success
  must reflect persisted success only. Use in-process/mock fixtures, no daemon.
- Windows key ACL behavior needs platform-specific verification. Unix mode
  rejection does not claim Windows ACL enforcement.
- Locale/command reference consistency, compile and website build remain unrun.

Signatures prove payload integrity relative to the supplied external anchor,
not unchanged implementation safety, runtime blocking or rollback freshness.
Stripping signature AND omitting required external verification can look
unsigned. Do not treat a committed key as implicit bootstrap or a successful
fixture as evidence of production trust provisioning.

## Cleanup and report

The EXIT trap removes only this block's unique `SIGNING_TEST_ROOT`, including
its ssenv, synthetic HOME/XDG state and ephemeral keys. No server daemon is
started. Report actual results, unrun checks and failures to the owner; never
remove real trust state to recover from an integrity failure.
