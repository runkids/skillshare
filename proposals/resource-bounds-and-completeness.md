---
last_edited: 2026-11-01
---

# Feature Proposal: Bound install and audit work, and report incomplete inspection

Status: design proposal; unapproved. This document includes no implementation,
measured resource guarantees, or change to current installation or exit policy.

Issues: [#683](https://github.com/runkids/skillshare/issues/683),
[#689](https://github.com/runkids/skillshare/issues/689),
[#713](https://github.com/runkids/skillshare/issues/713),
[#719](https://github.com/runkids/skillshare/issues/719).

## Problem

An installation or audit needs two answers: how much work may it consume, and
what was left uninspected when that allowance ran out? A sequence of individually
timed downloads can exceed any useful aggregate allowance. Fixed audit file/depth
limits and bounded worker counts do not bound total bytes, files or elapsed work.
A below-threshold report also needs to disclose skipped content and analyzers
that did not finish.

The installation client already has a 15-second per-request timeout. The audit
scanner has fixed per-file and depth ceilings. Preserve those protections while
adding shared accounting and explicit coverage. Resource exhaustion must retain
findings and explain the gap; it must not turn omitted work into a clean verdict.

This combines the contracts proposed in
[#684](https://github.com/runkids/skillshare/pull/684),
[#690](https://github.com/runkids/skillshare/pull/690),
[#717](https://github.com/runkids/skillshare/pull/717) and
[#724](https://github.com/runkids/skillshare/pull/724).
Their issues remain design discussions. Consolidation does not grant approval
or imply that the original PRs have been retired.

## Proposed Solution

Use separate ingestion and audit budgets with one shared coverage contract for
the audit consumers. They have different units and scopes; an installation
allowance must not silently become an audit allowance. Keep each mechanism and
its defaults independently discussable within this proposal.

### Remote installation admission

Add one ingestion budget per installation, shared across recursive requests and
files. Reuse the existing GitHub/Gitea clients, validation, staging and cleanup
paths rather than adding a second downloader.

| Proposed ceiling | Starting value | Enforcement boundary |
|---|---:|---|
| File-content bytes | 100 MiB | Aggregate streamed content, including responses with missing or misleading Content-Length |
| Entries | 10,000 | Count directories and files before creating entries or descending |
| Depth | 64 | Check before recursive descent |
| Path length | 4,096 bytes | Check admitted paths before materialization |
| Overall duration | 60 seconds | Shared elapsed deadline for supported remote operations |

Expose these limits under an `install` configuration section, retaining the
existing per-request timeout. Final key names and whether forge metadata uses
the content allowance or a separate smaller allowance require agreement.
Check both API-response and file reads while streaming; an up-front declared
length is only an early rejection check. Carry one budget through recursive
downloads, including concurrent work if supported.

Inspect a cloned working tree once before copying or publishing it, without
following symlinks, and reject trees exceeding the admitted limits. This is
working-tree admission: it does **not** cap Git pack transfer or clone-time disk
use. A strict Git transport/storage guarantee needs a separate design before it
can be advertised. Reuse existing link/path checks; this proposal does not
introduce a new symlink policy.

Return a typed `ErrIngestLimit`-style error identifying the exhausted allowance.
Remove only this operation's partial staging content. Existing installations
must survive rejection or cancellation. CLI and dashboard installation paths
must resolve the same configuration and expose the same failures.

Integration points include `internal/install/limits.go` (proposed),
`internal/install/github_download.go`, `internal/install/gitea_download.go`,
Git staging in `internal/install`, and `internal/github/client.go`.

### One audit coverage contract

Propose `audit.Result.Coverage` with `AnalyzersRun`, `Complete`, and per-file
relative paths, dispositions and stable reasons. Exact serialization is a design
decision; every consumer must use the same definition.

| Disposition or scope | Required meaning |
|---|---|
| `scanned` | The applicable declared work completed; selected-analyzer scope remains visible |
| `partial` | Some work completed but content or applicable analysis remains unfinished |
| `skipped` | The file received no applicable inspection; record its reason |
| Intentional exclusion | Record separately as declared scope, rather than silently treating it as completed work |

Retain reasons covering `oversize`, `depth`, `binary`, `unreadable`, `non_utf8`,
`budget` and `analyzer_error`. Map budget exhaustion to explicit `runtime_limit`
or the agreed stable equivalent, with the exhausted allowance and observed/
allowed counters. Settle the final reason vocabulary once; do not introduce
different meanings across exporters. Record analyzers that actually ran and
their failures, rather than just configured analyzer names. Warn when
`--analyzer` narrows inspection.

Define completeness against declared policy scope, separately from execution
success and threat severity. An intentional exclusion must remain visible;
whether each exclusion affects `Complete` needs an explicit policy, especially
for binary artifacts. Failed, partial and never-started work cannot disappear
from accounting. Batch results must identify unfinished and never-started
targets; do not invent per-file inspection records for paths never enumerated.
Retain already emitted findings even when inspection stops. A percentage alone
cannot establish completeness.

Emit coverage and explicit incomplete state in text, JSON and Markdown, show it
in the dashboard, and use SARIF `toolExecutionNotifications` for inspection
failures/limitations. Update audit documentation. The same result must reach
CLI and install/update policy consumers.

Propose `audit --fail-on-incomplete` and `audit.fail_on_incomplete`. Test both
gate settings. Agree whether strict installation profiles enable it by default.
The audit-budget proposal recommends blocking automatic install/update approval
for incomplete scans unless an explicit separate policy permits them; that
recommendation remains an open maintainer decision. Actual audit execution
failures remain fail-closed and cannot be accepted as ordinary findings.
When the agreed strict policy blocks, print the skipped/partial file list and
reasons so users can see which inspection gaps caused the decision.
Keep incomplete-policy failures distinguishable from threshold findings and
execution errors where the CLI supports those distinctions. This document
does not assign a new exit code or reinterpret the separate execution-error fix.

### Bounded inspection of oversized text

Keep bounded overlapping text windows as part of the full scope, rather than
stopping at a ledger-only implementation. Preserve original byte/line locations
and deduplicate findings in overlaps. Limit reads, windows and line processing
under the audit budget; disclose omitted bytes and context.

Only declare complete analysis where an analyzer can justify it. Fixed overlap
does not guarantee arbitrary regex, long-range matching or parser-context
coverage. Window boundaries, encoding failures and unavailable parser context
must produce the appropriate partial/skipped state. Choose supported analyzers,
window/overlap sizes and their completeness rules before implementation.

### Audit budgets and cancellation

| Proposed configuration | Starting value | Scope |
|---|---:|---|
| `audit.max_total_bytes` | 64 MiB | One invocation, shared across workers |
| `audit.max_files` | 10,000 | One invocation, shared across workers |
| `audit.timeout` | 30 seconds | Invocation deadline; proposed `--timeout` override |
| `audit.max_findings_per_skill` | 1,000 | Each skill |
| `audit.max_line_bytes` | 64 KiB | Each regex-processing line |

Retain existing per-file/depth ceilings initially. Validate finite positive
audit overrides, document units, and resolve policy consistently in global and
project modes, CLI, dashboard and install/update audit gates. Define how skipped
files and integrity reads consume the allowance without losing integrity
coverage. These counts must not be reset independently by concurrent workers.

Propagate context/deadlines through `ScanSkill`/`ParallelScan`, collection,
content analysis, skill analyzers and supported I/O. Check before expensive
stages and within reads/loops. Stop scheduling new skills on cancellation.
Timing out a waiting goroutine while scanning continues is insufficient.
Cooperative cancellation has limits: agree acceptable overshoot for
non-interruptible library calls rather than claiming a hard real-time bound.

Preserve partial findings and coverage. Bound long-line processing; a scanned
prefix is not a fully inspected line. When a skill reaches its finding cap,
emit one informational `findings-truncated` notice and record omitted analysis.
Agree whether that administrative notice is outside the finding allowance so
it remains observable at the exact boundary.

Integration points include `internal/config/config.go`,
`internal/audit/audit_scan_skill.go`, `audit_scan_content.go`, `audit_result.go`,
and `internal/audit/parallel.go`, plus report and policy consumers. Reuse the
coverage contract or deliver it jointly; a separately merged coverage feature
is not an inherent prerequisite.

### Hostile fixtures and acceptance criteria

Generate reproducible fixtures in temporary directories with bounded generators;
do not commit large input trees. Use fixture-only HTTP servers and synthetic
content, never a real provider account or user archive. Close every server/body
and clean up test-owned temporary data.

| Fixture or check | Required assertion and lane |
|---|---|
| Original 50,000 tiny-file flood | Explicit large performance lane; accounting, termination and partial coverage |
| 100-level nesting | Existing depth-ceiling baseline first; retained findings and explicit depth omissions once coverage exists |
| Oversized files and one 50 MiB line | Bounded reads/line processing; omitted content reported, no claim of full inspection |
| Original 1 MB near-match input per regex-heavy rule | Per-rule time/allocation checks within admitted file limits; document exact byte units before encoding the generator |
| 100,000-file generated audit benchmark | Explicit performance lane outside the default fast suite; shared counters and stopped scheduling |
| Never-ending and slow `httptest` HTTP bodies | Streaming enforcement without trusted Content-Length, cancellation, typed errors and closed resources |
| Recursive installation aggregate exhaustion | Shared bytes/entries/depth/path/deadline checks; operation-owned staging removed and existing install preserved |
| Every new byte/file/finding/line limit | Exact-limit and one-over deterministic accounting; retained findings, reason and observed/allowed values |
| Installation entry/depth/path limits | Exact and one-over boundaries, including the original 10,000-entry profile; reject before further descent/materialization |
| Elapsed deadlines | Controlled cancellation tests plus generous termination guards; prove worker termination, not just caller return |
| Every coverage disposition/reason and selected-analyzer run | All exporters, dashboard and policy consumers agree; both incomplete-gate modes tested |
| Oversized-window boundaries | Original source locations, overlap deduplication, encoding/parser gaps and preserved partial findings |
| Concurrent scans and integrity reads | Invocation counters remain shared; partial, timed-out and never-started targets visible |

The original near-match requirement says **1 MB**, while the scanner's existing
file cap is 1,000,000 bytes; the hostile long-line profile says **50 MiB** in the
published proposal. Preserve that distinction instead of silently converting
the near-match profile to 1 MiB, which would exceed that file cap. Adjusted
admission for a test must be explicit and must not change production defaults.

Document each ceiling, unit, enforcement stage, fixture and expected exhaustion
result. Baseline file/depth tests can start before new limits exist, but they
do not complete this suite. New ingestion-limit tests depend on ingestion
enforcement; new audit-limit tests depend on audit budgets; coverage assertions
depend on the shared contract. Archive-bomb/forged-count fixtures are a later
optional extension with archive inspection, not a prerequisite here.

Use deterministic counters as the policy oracle and generous timing margins
for CI variability. Benchmarks alone do not prove cleanup or coverage. Go regex
properties do not establish a wall-clock guarantee for every analyzer.

### NVIDIA inspiration and exact differences

All references below pin NVIDIA/SkillSpector commit
`6d0b7c849949cab9181e57798247f89e48ce5d6a`. These are concrete mechanisms to
adapt, not claims that Skillshare already has them or that the proposed defaults
match NVIDIA's. No source port is included.

| Mechanism | Inspected source/tests | What informs this proposal; what differs |
|---|---|---|
| Streamed ingestion and deadline | [`InputHandler._deadline`, `_check_deadline`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/input_handler.py#L899-L912); [`_download_file`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/input_handler.py#L1388-L1452) | Check elapsed work and actual streamed bytes before writing; remove a partial file on failure. Skillshare needs an aggregate recursive forge budget, proposed limits/configuration, CLI/dashboard integration and operation-owned cleanup. These spans establish no Git pack-transfer cap. |
| Ingestion failure fixtures | [`test_streamed_body_overflow_rejected_when_header_missing`, `test_streamed_overflow_leaves_no_partial_file_on_disk`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/unit/test_input_handler_bounds.py#L271-L346) | Generator-backed streamed overflow and cleanup assertions inform fixture HTTP tests. Endless/slow recursive forge cases and preservation of an existing Skillshare installation need additional tests. |
| Completeness model | [`LedgerOutcome`, `LedgerReason`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/inspection_ledger.py#L23-L70); [`PlannedWorkTarget`, `InspectionLedgerEvent`, `AnalysisCompleteness`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/inspection_ledger.py#L305-L396); [`finalize_ledger`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/inspection_ledger.py#L807-L843) | Separate terminal work, scope exclusions and analyzer status; finalize a canonical projection. Skillshare's `Result.Coverage`, stable reason mapping, incomplete install policy, windowed scans and report compatibility need their own design. The finalization span is the function entry/accounting setup, not its whole validation algorithm. |
| Coverage accounting tests | [`test_inspection_work_id_separates_analyzers_and_overlapping_chunks`, analyzer status and scope-boundary tests](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/test_inspection_ledger.py#L38-L98) | Distinct identities for overlapping work and visible scope boundaries inform accounting tests. These tests do not establish complete arbitrary regex/parser analysis across windows. |
| Analyzer ceilings | [`MAX_FINDINGS_PER_ARTIFACT`, `MAX_FINDINGS_PER_ANALYZER`, `_static_max_seconds_from_environment`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/static_runner.py#L180-L204) | Explicit finding/time ceilings and finite-positive validation are direct analogues. NVIDIA uses 10,000 finding ceilings and a 300-second artifact default here; Skillshare proposes 1,000 per skill and a shared 30-second invocation. This span does not prove invocation-wide cancellation or byte/file accounting. |
| Adversarial shared-budget/preflight tests | [`test_archive_member_budget_is_shared_across_outer_archives`, `test_preflight_rejects_actual_member_count_before_zipfile_allocation`, `test_preflight_rejects_large_central_directory_before_zipfile_allocation`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/test_nested_artifacts.py#L799-L870) | Assert shared accounting and reject before expensive allocation even when metadata lies. This testing pattern informs admission tests; those archive tests do not implement Skillshare's tree, regex or HTTP suite. Archive inspection remains optional separate work. |

## Alternatives Considered

- Per-request or per-file limits alone leave recursive and concurrent aggregate
  work unbounded. Rejecting after copying consumes the resources admission is
  supposed to protect.
- Counting only readable bytes cannot identify omitted artifacts. Converting
  every skip into a threat finding confuses inspection coverage with malicious
  behavior. A ledger alone leaves the oversized-text scanning requirement open.
- An outer goroutine/test timeout can leave work running. Hard process isolation
  would establish a different termination boundary and is outside this initial
  design; cooperative overshoot must remain explicit.
- Benchmarks without assertions miss cleanup and partial results. Real networks
  add variability without needed coverage. Use local synthetic HTTP instead.

## Scope

- [ ] Small (1–3 files, under 200 lines)
- [ ] Medium (3–10 files, 200–500 lines)
- [x] Large (10+ files, 500+ lines), delivered in agreed stages

The original ingestion estimate was medium (potentially larger across all
consumers); completeness was large; audit budgets estimated 500–850 production
lines; the initial fixture suite estimated 250–450 test lines plus generated
data/documentation. These are planning estimates, not limits to split around
or a new measured total.

1. Agree units, scope/completeness semantics, default values and policy decisions.
   Existing-limit fixture tests can be prepared independently.
2. Implement ingestion accounting, typed failures, cleanup and consumer parity
   with deterministic boundary tests. No dependency on an archive analyzer.
3. Deliver the shared audit result/coverage contract and exporter/policy parity;
   deliver budgets jointly or reuse that contract, with real cancellation and
   bounded text-window support. A ledger-only stage remains an interim stage.
4. Complete all new-limit hostile tests and the explicit performance lane once
   their corresponding enforcement exists. Publish the ceiling/fixture table.

Maintainer agreement and approved issues are required before implementation.
No new runtime dependency is selected here; use existing Go context/I/O/testing
mechanisms first and agree any additional analyzer/parser dependency separately.
Update-consent policy remains a separate proposal. Actual implementation PRs
remain separate; no code delivery or prior review certifies this combined design.

## Open Questions

- Are the proposed installation and audit defaults appropriate for legitimate
  large repositories? May install limits be disabled or only raised? Audit
  overrides are proposed to remain finite and positive.
- Are the invocation aggregation scope, skipped-file counts, integrity-read
  accounting and finding-cap notice treatment acceptable?
- Should forge metadata share the content budget or receive a smaller ceiling?
- Is post-clone tree admission sufficient for Git, or is a separate hard
  transport/storage design required?
- Which intentional exclusions define scope and which make a scan incomplete,
  especially binary content? How should later analyzers extend the contract?
- Should strict profiles enable incomplete gating by default? Which separate
  overrides, if any, may permit incomplete install/update approval?
- What exit/error representation preserves threshold, incomplete-policy and
  execution-failure distinctions without breaking existing callers?
- Which analyzers support useful bounded windows, with what overlap/context
  rules and accurate source locations?
- What deadline overshoot/termination margin is acceptable for library calls
  that cannot be interrupted? Can each enforcement stage expose deterministic
  counters for tests?
- Which performance benchmarks run in scheduled CI versus an explicit local
  lane? Confirm near-match byte units and whether archive-bound fixtures become
  a separate optional extension.
