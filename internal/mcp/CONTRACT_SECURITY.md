# MCP contract review security

This note describes contract review and optional externally anchored signing in
`check_contract.go`, `check_contract_diff.go`, `check_pins.go`,
`check_pins_identity.go`, `check_pins_signature.go` and `check_live.go`.

## Threat model and enforcement boundary

A configured MCP server can alter any advertised tool field, return incomplete
or ambiguous inventories, embed recognized credentials, or change its claimed
identity. Configuration and saved approval state can change while a probe is
running. The review mechanism detects advertised-contract and connection
identity drift at probe time, and prevents incomplete or stale observations
from becoming approved baselines.

Skillshare is not a runtime proxy. It does not authorize individual calls,
validate call arguments or results, quarantine native-client connections,
attest executable bytes or prove implementation safety. A malicious server can
keep its advertised contracts unchanged. The server's protocol and serverInfo
are observations, not authenticated claims. Ambient environment changes and
changes to the values behind environment references are not attested.

The unsigned store cannot authenticate a syntactically valid local edit by an
attacker who can write it. Hash consistency validation detects malformed or
inconsistent state, not deliberate recomputation by that attacker. File mode
0600 protects ordinary local access where supported; it is not a signature or
an external trust anchor.

## Native profile: skillshare-contract-v2

Descriptors must be complete JSON objects with a valid name and required
object `inputSchema`. Malformed typed fields, duplicate decoded keys, invalid
UTF-8, unpaired surrogate escapes and excessive nesting are rejected before
capture. `outputSchema`, when present, is an object. An explicit schema type,
when present at the input/output root, must be `object`.

Canonicalization normalizes object key ordering and JSON whitespace. It
preserves exact numeric lexemes, including large integers, decimal/exponent
spellings and negative zero, and preserves **every array's order**, including
schema sets, enum/default/example values and extension data. A harmless array
reorder or numeric respelling can therefore require an extra review. No
Unicode normalization or full RFC 8785/PIN-SPEC interoperability is claimed.

Missing description becomes `""`. Omitted known annotation hints use
conservative defaults: readOnlyHint=false, destructiveHint=true,
idempotentHint=false, openWorldHint=true. Explicit false is not confused with
true. Other annotation values and unknown top-level fields remain covered.
The stored snapshot is the canonical descriptor after these defaults.

Each tool retains SHA-256 hashes named `hash` (whole descriptor), `inputHash`
(input schema), `proseHash` (description/title), `outputHash` (output schema),
and `metadataHash` (all remaining top-level fields, including name,
annotations, icons, `_meta` and unknown fields), plus its full `definition`.
The API projects the latter as an exact `definitionText` string; interpreting
it as a JavaScript number-bearing object would round reviewed values.

## Directional changes and audit

Changes identify the tool, dimension, JSON pointer and a conservative grade:

| Grade | Review meaning |
|---|---|
| BREAKING | Removed tool or proven narrowing of supported accepted input |
| LOOSENED | Proven weakening of supported constraints |
| EXPANDED | Added tool or proven expansion of supported input options |
| DESCRIPTION | Model-visible description/title changed |
| NOTATION | Supported schema prose changed |
| METADATA | Annotation or other metadata changed |
| UNKNOWN | Structural change without a supported directional proof |

The supported schema fragment includes selected types, properties, required,
enum and bounds, with exact numeric comparison and integer as a subset of
number. Context can invalidate a local direction: adding an optional property
in an open object can narrow accepted input. Unsupported applicators,
references, uncertain mixed edits and changed output schemas do not gain a
safe/cosmetic label. This is not a general schema implication solver.
DESCRIPTION and NOTATION still require review; no grade auto-approves drift.
At most 200 change summaries are emitted, with `omitted` counting further
details; the complete bounded current snapshots remain available.

The existing audit engine scans decoded descriptor keys and string values,
including nested schema descriptions, without Markdown tutorial suppression.
Findings carry tool, path, rule ID and severity, not raw payload snippets.
The scanner caps work during scanning. An audit rule-load failure or exhausted
analysis budget is incomplete and blocks the review revision and acceptance.
Completed audit findings are warnings for operator review; a completed scan
with no findings proves neither safety nor absence of prompt injection.

## Complete probes and privacy

Stdio uses a resolved executable and a literal argv list with an explicit
working directory. There is no implicit shell concatenation; a configured
shell is still executable server code. HTTP redirects are refused. JSON-RPC
version, matching response ID and result/error shape are checked. Duplicate
keys, ambiguous results and duplicate tool names are rejected. Discovery
fallback applies only to supported legacy cases, not malformed inventories.
No metadata, schema reference, icon or resource-metadata URL is fetched.

| Resource | Bound |
|---|---|
| Pages | 50; a cursor remaining at the cap or cycling is incomplete |
| Tools | 4,096 across the inventory |
| Descriptor inventory | 4 MiB raw aggregate and 4 MiB canonical aggregate |
| One descriptor | 256 KiB raw and canonical |
| Descriptor JSON nesting | 64 levels |
| Baseline store | 16 MiB |
| Audit decoded strings/keys | 4,096 |
| Audit decoded text | 1 MiB |
| Audit visual lines | 8,192 |
| Audit findings | 200; an incomplete scan cannot be accepted |

A server probe defaults to 10 seconds and at most four probes run concurrently.
These are completeness bounds: no successful prefix can reach acceptance.

Recognized configured credentials are checked in decoded keys/values and
identity-adjacent material before snapshots, hashes or revisions are exposed.
This includes short nonempty credentials. The selected stored snapshot is also
checked against current recognized values before comparison, so credential
rotation cannot expose a newly sensitive old descriptor. Diagnostics redact
recognized credentials. Failed stdio probes omit the server's stderr payload;
when diagnostic text exists, they report `server diagnostic output omitted`.
These checks do not detect arbitrary unconfigured
secrets, authenticate metadata or provide a general data-loss-prevention tool.

All environment/header declarations, including ordinary non-secret literals,
and a configured bearer token must use `fromEnv` for baseline trust. Ordinary
configuration with literals remains valid for management and ordinary probing;
it has no approvable revision. Resolved credential values are not identity
inputs. Full snapshots are nevertheless potentially sensitive text and are
stored privately, not suitable as a public manifest.

## V2 store and identity

State remains at `<StateDir>/mcp/tool-pins.json`, written atomically with mode
0600. The envelope contains `version: 2`, `servers` (untouched legacy hash
evidence), and `records`, plus an optional `signature` in V2. Unknown/missing
envelope or record fields, invalid
snapshots/hashes, unsupported versions, oversized files, symlinks and
nonregular files are refused. Unreadable or newer stores are preserved.
Inspect the file or restore a valid backup; deleting/moving trust to bypass an
integrity failure is not a recovery procedure.

`records` is keyed by a digest of canonical configuration/source identity,
effective root and configured server name. Each record contains `config`,
`root`, `name`, `policy`, `identity` and complete `tools`. `config` derives from
canonical filesystem paths of the configuration and selected source, not a
public content hash of secret-bearing source bytes. Project configurations
use their own effective project root; global and `mcp.projects` scopes remain
distinct even for identical server names.

Canonical configuration/source paths and actual stdio launch paths must be
valid UTF-8 before identity serialization; unsupported Unix byte paths are
refused rather than collapsed by JSON replacement. Valid Unicode, including
U+FFFD, remains supported. Known credential matches in canonical source paths
also prevent an identity/revision. CLI source-path display is separately
redacted without changing the operational paths or hashing redacted text.

Connection `identity` binds the policy, complete server declaration (transport,
endpoint or command/arguments, environment/header reference declarations,
tools and native options), actual stdio executable/working directory and
observed protocol/serverInfo. HTTP omits unused executable/cwd observations.
Changing connection identity and tool contracts can be reported together.
Neither a path nor an identity hash attests implementation bytes.

V1 `servers` entries are evidence only and never silently become reviewed V2
records. Explicit acceptance writes one V2 record and retains legacy evidence
and other records. Reaccepting an identical approved record is a verified
no-op that preserves the store bytes.

## Externally anchored signed stores

`--verify-key FILE` accepts an Ed25519 PKIX PEM `PUBLIC KEY`. Only explicit
`--accept-tools --revision ID` may load `--signing-key FILE`, an Ed25519 PKCS8
PEM `PRIVATE KEY` matching that verifier. All signing options require `--live`.
Keys are never provisioned automatically or persisted, including their paths,
in config/state. The external verifier and required-use policy must be trusted
independently; a key committed beside mutable state is not a trust bootstrap.

Key reads are limited to 64 KiB, one unencrypted PEM block without headers or
extra content, and regular files with link/replacement checks. Unix private
keys with any group/other permissions are refused. Windows skips that Unix
mode check; operator-managed ACL protection remains necessary. Fixed generic
errors exclude paths, key material and raw OS/parser diagnostics. Private key
bytes are not forwarded to probes, API/UI, reports or logs.

V2 has one optional `signature: {algorithm: "Ed25519", value: BASE64}` inside
its atomic JSON envelope, with no sidecar. The signing preimage is the literal
`skillshare-mcp-pins-v2\n` domain prefix (one terminating newline) followed by
deterministic compact JSON of the entire unsigned envelope, excluding the
signature and trailing serialization newline. This is the native store
serialization, not descriptor-default expansion or a PIN-SPEC/RFC 8785 claim.
Verification parses and verifies the same immutable bounded read before
trusting any scope, and repeats against the locked reread before writing.
Duplicate/unknown fields, unsupported versions/algorithms and malformed
signatures fail closed. An unchanged verified acceptance preserves every byte,
including the signature.

| State | Required behavior |
|---|---|
| Missing store + verifier | Untrusted read-only preview; matching signer and reviewed revision for first signed acceptance |
| Unsigned store, no verifier | Existing unsigned workflow; no deliberate local-tampering protection |
| Unsigned store + verifier | Error except explicit `--sign-unsigned` migration preview; matching signer and migration revision also required for acceptance |
| Valid signed store | Correct external verifier on every check; matching signer also required for acceptance |
| Signed store without verifier | CLI verification required; no comparison/approval revision or API/UI trust action |
| Bad signature, unknown algorithm, malformed store | Error, preservation, no migration bypass |

`--sign-unsigned` only accepts existing unsigned stores; missing or signed
stores are refused, even when the signature is valid. Every migration preview
is `legacy` with identity review required, including a matching selected
unsigned record; strict mode fails until signed acceptance. It proposes a
whole-store migration: accept the selected reviewed record and downgrade ALL
unselected V2 records to policy `legacy-unsigned`. Their complete evidence
survives, but requires independent review and reacceptance to regain trust.
Legacy V1 evidence is retained. The revision preimage binds migration mode and
the entire proposed transformation, so a normal unsigned review token cannot
be replayed to authorize signing. Migration is never an identical-record
no-op; ordinary no-op applies only once the intended protection mode is met.

The fixed refusals below are errors even without strict mode and before live
probing; the report prefixes them with `tool contracts could not be compared:`.
They do not expose key paths or raw parser/OS diagnostics.

| Generic refusal | Required review action |
|---|---|
| `MCP baseline key is invalid or unreadable; use an Ed25519 PEM key with safe file permissions` | Check the supplied key format, regular-file bounds and private permissions |
| `signed MCP tool baseline requires CLI verification with --verify-key` | Use CLI verification with the independently trusted public key |
| `MCP tool baseline signature could not be verified; supply the correct external verification key` | Check the external anchor and investigate state integrity; preserve the failed store |
| `unsigned MCP tool baseline requires explicit --sign-unsigned review before signing` | Independently review the explicit migration, including all unselected-record downgrades |
| `accepting signed MCP tool baselines requires a matching --signing-key and --verify-key` | Supply matching Ed25519 keys only for explicit reviewed acceptance |
| `--sign-unsigned requires an existing unsigned MCP tool baseline` | Omit migration for missing or signed stores; it is not an integrity bypass |

Signatures authenticate payload integrity relative to the supplied anchor.
They provide no executable attestation, call-time enforcement or rollback
freshness. Stripping a signature AND omitting the external verification policy
can appear unsigned; protected callers must always supply their verifier.

## Revision-bound authority

First observation is `untrusted` and writes nothing. Matching V1 evidence is
`legacy`; a matching approved record is `unchanged`; contract, policy or
connection identity changes are `drifted`. Ordinary checks, add, sync, API GET
and editor draft probes cannot establish or replace trust.

`--root` omitted observes all scopes; empty selects this configuration's own
server list; nonempty must exactly match a configured `mcp.projects` root.
`-p` selects project configuration. Approval requires `--live`, one explicit
name, one resolved server/scope and `--revision <contract.revision>`.

The opaque revision hashes the proposed full record, exact pre-probe store
byte digest and existence state. Acceptance re-probes and must match that
review revision. Under the existing operation lock it rechecks source
unchangedness, canonical configuration identity, store existence and exact
bytes before saving. Source/store changes or a new inventory require a new
review. Another record's acceptance also stales the old store revision.
An inventory with probe, audit, validation or persistence errors cannot be
reported accepted. The typed `toolsAcceptance` outcome is `accepted`, `stale`
or `error`; only persisted success or verified no-op yields `accepted`.
Explicit CLI/API approval attempts write an `operations.log` outcome entry and a
separate `mcp contract review` entry in `audit.log`. The shared
`internal/mcpchecklog.WriteAudit` writer records only `servers`, `tools`,
`findings` counts and `accepted`, with standard timestamp/status/duration
metadata. It excludes server names, tool/contract text, revisions, diagnostic
payloads and credential values. `accepted` is true only for a typed persisted
success with no check error; other outcomes use status error and accepted=false.
Ordinary read-only checks do not call the audit writer. Logging is best effort
and cannot undo an already completed approval. Inspect the separate audit stream
with `skillshare log --audit`.

Normal untrusted/legacy/drift findings warn. `--strict-tools` with `--live`
promotes those to errors. Incomplete/audit failures, invalid stores, stale or
refused approval and failed writes are errors regardless of strict mode.
Strict mode supplies a command/CI gate, not runtime enforcement.

## Local API and dashboard

`GET /api/mcp/check?live=1&root=` is a read-only observation. Root omission,
empty root and exact nonempty root have the CLI semantics. Local-protected
`POST /api/mcp/check` takes exactly `{name, root, revision}` and delegates to
the same approval transaction. Duplicate/unknown fields, trailing or oversized
JSON and missing inputs are rejected. HTTP 200 with `accepted: true` requires
a typed persisted outcome; stale review returns 409, other refusal 422,
and malformed requests 400. Snapshots use `definitionText`, with the split
hashes unchanged; raw `definition` stays in CLI/store output.

The API/dashboard carry no key inputs. Signed-store live checks require CLI
verification and cannot expose a trust action; use the CLI with the external
verifier and, for approval, the matching signer.

Static **Check** stays static. **Review live tools** only opens a confirmation;
**Probe live tools** explicitly connects. Expandable full snapshots, grades
and audit findings render as inert text with formatting controls visible,
without activating metadata URLs or icons. **Trust this inventory** posts the
exact root/name/revision. A stale, failed or completed approval requires a new
probe before another approval. **Load tools** remains a draft-only probe.

## Prior art and provenance

The primary conceptual reference is
[Paraphern/RugSnare at db0bb6a7021c17f0dd9b26457f65b686a87d24db](https://github.com/Paraphern/rugsnare/tree/db0bb6a7021c17f0dd9b26457f65b686a87d24db).
Its inspected root [LICENSE](https://github.com/Paraphern/rugsnare/blob/db0bb6a7021c17f0dd9b26457f65b686a87d24db/LICENSE)
is Apache-2.0, copyright 2026 Paraphern; no NOTICE file was present in the
inspected tree. Split contract dimensions, readable drift, identity/policy
binding, externally anchored signatures and explicit review informed
independently authored Go code and inert test fixtures. No RugSnare source, translated source, fixture or golden
vector was copied. The native number/array/default rules, hash dimensions,
identity and store/revision format intentionally differ. The
[PIN-SPEC](https://github.com/Paraphern/rugsnare/blob/db0bb6a7021c17f0dd9b26457f65b686a87d24db/docs/PIN-SPEC.md)
is prior art, not a compatibility claim or an assertion that its runtime
protection is present in Skillshare.

[Tirith at c98b16a6ab394df03513994b0757d359e43e8c10](https://github.com/sheeki03/tirith/tree/c98b16a6ab394df03513994b0757d359e43e8c10)
is concepts-only prior art for safe inspection, poisoning signals, bounded
analysis and honest enforcement limits. Its MCP code is AGPL-3.0-only under
its [workspace declaration](https://github.com/sheeki03/tirith/blob/c98b16a6ab394df03513994b0757d359e43e8c10/Cargo.toml)
and [LICENSE-AGPL](https://github.com/sheeki03/tirith/blob/c98b16a6ab394df03513994b0757d359e43e8c10/LICENSE-AGPL).
No Tirith source, translated implementation, rules, data tables or fixtures
were reused. Its separate third-party notices do not license its MCP engine
for incorporation into this MIT repository.

Verification commands and fixture-only scenarios are in
[the contract-review runbook](../../ai_docs/tests/mcp-contract-review_runbook.md)
and [the signing runbook](../../ai_docs/tests/mcp-signing_runbook.md).
Authored tests and source inspection are not runtime evidence; execution is
pending until the supported devcontainer or remote CI runs them.
