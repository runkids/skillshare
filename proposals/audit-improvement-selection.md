# Feature Proposal: Select audit and supply-chain improvements

## Problem

I opened a lot of separate proposal PRs and issues for audit and install hardening. That's a lot to review one at a time, and most of them only need a yes, no or later. This file replaces them with one list. Every earlier proposal PR and issue is closed and linked from its item below, so nothing is lost.

## Proposed Solution

Each numbered item is independent. Pick the numbers you want and I'll open one focused implementation PR per item, scoped to the decision you give. Anything not picked is declined or deferred, and merging this file doesn't approve any of it.

Each item has a one-line summary and the main decision it needs. The full scope, defaults, open questions and pinned NVIDIA SkillSpector references (commit `6d0b7c849949cab9181e57798247f89e48ce5d6a`) are under **Full proposal**. Thresholds and defaults there are proposals, not measurements.

The code PRs #686, #691 and #711 are separate and stay open. Earlier related design: [tracked update audit #646](https://github.com/runkids/skillshare/pull/646) (merged).

### Index

| # | Idea | Size | Decision needed |
|---|---|---|---|
| 1 | [Bound remote installation](#1-bound-remote-installation) | Medium | Which limits and Git transfer guarantees should be supported? |
| 2 | [Report audit completeness](#2-report-audit-completeness) | Large | What incomplete-scan policy should apply to strict and install flows? |
| 3 | [Bound audit resource use](#3-bound-audit-resource-use) | Large | Which budgets, override controls and cancellation guarantees should be defaults? |
| 4 | [Test resource ceilings](#4-test-resource-ceilings) | Medium | Which expensive scenarios belong in a separate performance lane? |
| 5 | [Inspect bundled MCP configurations](#5-inspect-bundled-mcp-configurations) | Large | Which launcher versions, transports and environment forwarding policies should be supported? |
| 6 | [Inspect bundled hooks and settings](#6-inspect-bundled-hooks-and-settings) | Medium | Which bundled declaration forms, severities and activation assumptions are acceptable? |
| 7 | [Identify disguised executables and bytecode](#7-identify-disguised-executables-and-bytecode) | Medium | Which formats and uncertain headers should be reported? |
| 8 | [Inspect archive members](#8-inspect-archive-members) | Large | Which containers, member limits and incomplete-coverage policies should apply? |
| 9 | [Inspect dependency source redirection](#9-inspect-dependency-source-redirection) | Large | How should private registries, unknown sources and credential evidence be handled? |
| 10 | [Detect sensitive destinations and unsafe deserialization](#10-detect-sensitive-destinations-and-unsafe-deserialization) | Medium | Which language applicability and severities should apply to destination and deserialization rules? |
| 11 | [Inspect offline dependency manifests](#11-inspect-offline-dependency-manifests) | Large | How should package datasets be curated and trusted-container bypasses classified? |
| 12 | [Compare declared and observed capabilities](#12-compare-declared-and-observed-capabilities) | Large | What constitutes sufficient observed evidence, and how should unknowns and wildcard permissions be scored? |
| 13 | [Check bundled references](#13-check-bundled-references) | Medium | Which references are intentional external exceptions, and should escapes be findings? |
| 14 | [Normalize Unicode for matching](#14-normalize-unicode-for-matching) | Large | Which normalization and mixed-script heuristics should be enabled? |
| 15 | [Detect abusive triggers](#15-detect-abusive-triggers) | Large | Which trigger inventories and similarity thresholds should be used? |
| 16 | [Inspect metadata by field](#16-inspect-metadata-by-field) | Large | Which metadata schemas, base64 thresholds and severities should be supported? |
| 17 | [Detect whitespace padding](#17-detect-whitespace-padding) | Medium | Which whitespace thresholds, exemptions and severities should be used? |
| 18 | [Use Python syntax context](#18-use-python-syntax-context) | Large | Which Python parsing strategy and conservative fallback should be adopted? |
| 19 | [Support reasoned finding baselines](#19-support-reasoned-finding-baselines) | Large | How should baseline composition, migration and raw/effective scoring work? |
| 20 | [Handle direct prohibitions](#20-handle-direct-prohibitions) | Medium | Which prose rules may accept direct prohibitions, and how should ambiguity be handled? |
| 21 | [Identify evaluation datasets](#21-identify-evaluation-datasets) | Medium | Should dataset exclusion be opt-in or default, and which formats and strict policy apply? |
| 22 | [Measure detection accuracy](#22-measure-detection-accuracy) | Large | How should labels, per-rule metrics and metric-policy changes be reviewed? |
| 23 | [Cover rules with benign twins](#23-cover-rules-with-benign-twins) | Large | How should built-in rule coverage and explicit positive/negative twins be enforced? |
| 24 | [Explain findings and remediation](#24-explain-findings-and-remediation) | Large | How should overrides distinguish missing from empty values, and should guidance be localized? |
| 25 | [Filter displayed severity](#25-filter-displayed-severity) | Medium | Should JSON ignore the display floor or add hidden-count metadata? |
| 26 | [Discover and rank audits](#26-discover-and-rank-audits) | Large | Which discovery boundaries, ranking and timeout controls should be exposed? |
| 27 | [Expose catalog-scoped MCP audit tools](#27-expose-catalog-scoped-mcp-audit-tools) | Medium | Which catalog-scoped tools and network permissions should be exposed? |
| 28 | [Inventory and traverse external references](#28-inventory-and-traverse-external-references) | Large | Should transitive traversal be enabled separately from local inventory, with which boundary policy? |
| 29 | [Offer opt-in dependency vulnerability checks](#29-offer-opt-in-dependency-vulnerability-checks) | Large | Which exact pins, destination policy, cache TTL and incomplete states should be supported? |
| 30 | [Offer an opt-in semantic audit](#30-offer-an-opt-in-semantic-audit) | Large | Should the semantic stage be offered, and which provider, isolation and required-pass controls are acceptable? |
| 31 | [Confirm expanded update capabilities](#31-confirm-expanded-update-capabilities) | Large | Which capability changes require renewed consent, including unattended and batch updates? |
| 32 | [Audit managed hooks and rules before mutation](#32-audit-managed-hooks-and-rules-before-mutation) | Large | Which managed mutation points need audit gates and explicit overrides? |
| 33 | [Describe release trust and provenance](#33-describe-release-trust-and-provenance) | Large | Which release identity, signing, SBOM and provenance guarantees should maintainers adopt? |

## Install and audit limits

### 1. Bound remote installation

Cap how much a remote install can download (bytes, entries, depth, path length, time) and clean up only its own partial staging on failure.

**Decision:** Which limits and Git transfer guarantees should be supported?<br>
**Size:** Medium<br>
**Earlier discussion:** #684, issue #683 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Shared HTTP byte/entry/depth/path/deadline admission; proposed 100 MiB/10k/depth64/4096-byte paths/60s; typed errors, operation-owned cleanup, CLI/dashboard. Git working-tree admission is explicitly weaker than transport/storage bounds. Keep per-request timeout and metadata-byte policy question.

**Detailed proposal:**

Add one ingestion budget shared by the requests and files belonging to an
installation. Reuse the existing download clients, name validation, staging
and cleanup paths rather than adding another downloader.

- Start with proposed defaults of 100 MiB of file content, 10,000 entries,
  depth 64, path length 4,096 bytes and 60 seconds overall. Keep the existing
  per-request timeout. Expose limits through an `install` configuration section.
- Enforce API-response and file limits while reading, including bodies without
  a trustworthy Content-Length. Count directories and files before descending
  or creating more entries, with one shared budget across recursive requests.
- Apply the elapsed deadline to supported remote fetch operations. Inspect a
  cloned working tree before copying or publishing it, without following links,
  and reject content that exceeds the admitted tree limits.
- Return typed limit errors identifying the exhausted budget. Remove only the
  operation's own partial staging content and leave existing installations intact.
- Propagate the same configuration and failures through CLI and dashboard
  installation paths. Document the distinction between transfer and tree limits.

Relevant source is `internal/install/github_download.go`,
`internal/install/gitea_download.go`, the Git staging paths in `internal/install`,
and `internal/github/client.go`. Symlink policy is a separate concern; this
proposal reuses existing path checks and does not define a new link policy.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines), potentially larger with all consumers
- [ ] Large (10+ files, 500+ lines)

Implementation would include configuration/schema documentation, shared budget
accounting, forge/Git integration and fixture-only tests. Tests should cover
exact boundaries, a streaming body, recursive aggregate exhaustion, excessive
depth, deadline cancellation, typed errors and cleanup that preserves an
existing installation. No implementation or release change is included here.

**Additional open questions**

- Are the proposed defaults suitable for large legitimate skill repositories?
- Should configuration permit disabling a limit, or only raising its ceiling?
- For Git, is an admitted working-tree cap sufficient? That is not a hard cap
  on packfile transfer bytes or clone-time disk use. A strict transport/storage
  guarantee needs an additional design before being advertised.
- Should GitHub and Gitea metadata response bytes share the content allowance
  or have a separate, smaller ceiling?

**NVIDIA relationship:** Direct deadline, streamed byte bounds and partial cleanup tests; Git transfer guarantees remain distinct.

[src/skillspector/input_handler.py:899–912](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/input_handler.py#L899-L912) (`_deadline`, `_check_deadline`) · [src/skillspector/input_handler.py:1388–1452](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/input_handler.py#L1388-L1452) (`_download_file`) · [tests/unit/test_input_handler_bounds.py:271–346](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/unit/test_input_handler_bounds.py#L271-L346) (`test_streamed_body_overflow_rejected_when_header_missing`, `body_iter`)

</details>

### 2. Report audit completeness

Record which files were scanned, partially scanned or skipped and why, so a report can say an audit was incomplete instead of looking clean.

**Decision:** What incomplete-scan policy should apply to strict and install flows?<br>
**Size:** Large<br>
**Earlier discussion:** #690, #742, issue #689 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Per-file scanned/partial/skipped reasons, analyzers actually run, intentional scope vs failure, all report consumers/SARIF notifications, strict/install incomplete policy, bounded overlapping oversized-text scans with real source offsets and deduplication. Fixed overlap cannot guarantee arbitrary parser/regex completeness.

**Detailed proposal:**

Add a shared coverage contract to audit results and use it consistently in
CLI reports, installation policy and the dashboard.

- Record relative file paths with scanned, partial or skipped dispositions and
  stable reasons such as size, depth, binary, encoding, unreadable content,
  resource limits and analyzer failure. Record the analyzers that ran.
- Distinguish an intentionally excluded policy scope from a scan that could
  not complete. Show explicit warnings when analyzer selection narrows scope.
- Emit coverage and incomplete state in JSON, text and Markdown, and surface
  failures through SARIF execution notifications. Keep findings already found.
- Add `audit --fail-on-incomplete` and matching policy configuration. Agree how
  strict install policy handles incomplete inspection, keeping actual audit
  execution failures fail-closed rather than treating them as accepted findings.
- Scan oversized text in bounded overlapping windows where useful. Preserve
  source offsets and deduplicate overlap findings. Do not claim full analysis
  when window boundaries, encoding or parser context leave a coverage gap.

The main integration points are `internal/audit/audit_scan_skill.go`, the
parallel runner, `audit.Result`, the report exporters and existing CLI/install
policy consumers. Keep one definition of completeness across these consumers.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

This is a result/policy contract spanning collection, analyzers and consumers.
Fixture-only tests should cover each disposition, selected-analyzer runs,
unreadable content, analyzer errors, oversized-window boundaries, stable source
locations, partial findings and the incomplete-policy gate in both modes.
No implementation or change to current scan/exit policy is included here.

**Additional open questions**

- Which intentional exclusions affect completeness, and which define the
  declared inspection scope? Binary artifacts need an explicit policy.
- Should strict profiles enable the incomplete gate by default, or should it
  remain separately configured?
- What error/exit representation should incomplete-policy failures use, keeping
  them distinct from threshold findings where the CLI supports that distinction?
- Which analyzers can preserve meaningful context across text windows? Fixed
  overlap alone cannot guarantee arbitrary regex or parser-context coverage.
- How should the contract accommodate later aggregate scan budgets without
  requiring another incompatible result format?

**NVIDIA relationship:** Direct event and final completeness accounting; install gating and windowed scans differ.

[src/skillspector/inspection_ledger.py:23–70](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/inspection_ledger.py#L23-L70) (`LedgerOutcome`, `COMPLETED`) · [src/skillspector/inspection_ledger.py:305–396](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/inspection_ledger.py#L305-L396) (`PlannedWorkTarget`, `InspectionLedgerEvent`) · [src/skillspector/inspection_ledger.py:807–843](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/inspection_ledger.py#L807-L843) (`finalize_ledger`, `accounting_error`)

</details>

### 3. Bound audit resource use

Add per-invocation byte, file, time and finding budgets with real cancellation, keeping partial findings when a budget runs out.

**Decision:** Which budgets, override controls and cancellation guarantees should be defaults?<br>
**Size:** Large<br>
**Earlier discussion:** #717, #742, issue #713 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Invocation-wide 64 MiB/10k files/30s proposal with shared concurrent accounting, per-skill 1000 findings and per-line64 KiB, positive config and --timeout, actual cooperative cancellation, retained partial findings/never-started targets, integrity accounting and 100k-file performance lane.

**Detailed proposal:**

Add one explicit resource policy and cooperative cancellation through collection,
content analysis, skill analyzers and parallel scheduling.

- Propose `audit.max_total_bytes`, `max_files`, `timeout`,
  `max_findings_per_skill` and `max_line_bytes` configuration, plus an audit
  `--timeout` override. Resolve policy consistently in global/project modes,
  CLI, dashboard and install/update audit gates.
- Start discussion with 64 MiB aggregate bytes, 10,000 files, 30 seconds,
  1,000 findings per skill and 64 KiB per regex line. Retain existing file/depth
  ceilings initially; validate finite positive overrides and document units.
- Share aggregate bytes, files and elapsed budget across one audit invocation,
  including concurrent workers. Keep finding limits per skill and line limits
  per inspected line. Define counts for skipped files and integrity reads.
- Check cancellation before expensive stages and during reads/loops. Stop
  scheduling new skills and propagate context to supported operations; timing
  out a caller while its scan continues is not a resource guarantee.
- Preserve findings already produced. Add explicit Coverage status and reasons
  such as `runtime_limit`, with the exhausted budget and inspected/skipped
  counts. Keep unfinished and never-started skills visible in batch output.
- Bound line processing and report omitted content rather than silently treating
  a truncated prefix as a fully scanned line. Emit one informational
  `findings-truncated` notice when the finding allowance is exhausted.
- Propose that incomplete scans block automatic install/update approval unless
  an explicit separate policy permits them. A clean partial scan is not clean.

Relevant paths include `internal/config/config.go`,
`internal/audit/audit_scan_skill.go`, `internal/audit/audit_scan_content.go` and
`internal/audit/audit_result.go`. Coverage needs a shared completeness contract,
provided by coordinated work or delivered jointly here. Reuse of an existing
contract is preferred; a separate merged feature is not inherently required.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimated implementation: 500–850 production lines plus boundary/cancellation
fixtures and generated large-tree benchmarks. Include 100,000-file benchmark
coverage outside the default fast suite. No limits or tests are implemented here.

**Additional open questions**

- Approve the proposed defaults and invocation-wide aggregation policy?
- Which incomplete-scan overrides should be allowed for install/update gates?
- How should integrity verification share a budget without losing its coverage?
- What deadline overshoot is acceptable for non-interruptible library calls?

**NVIDIA relationship:** Direct analyzer ceilings; Skillshare config and cancellation differ.

[src/skillspector/nodes/analyzers/static_runner.py:180–204](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/static_runner.py#L180-L204) (`MAX_FINDINGS_PER_ARTIFACT`, `MAX_FINDINGS_PER_ANALYZER`)

</details>

### 4. Test resource ceilings

Add generated hostile fixtures (file floods, deep trees, huge lines, endless HTTP bodies) that check each limit at exactly the boundary and one over.

**Decision:** Which expensive scenarios belong in a separate performance lane?<br>
**Size:** Medium<br>
**Earlier discussion:** #724, issue #719 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Hostile generated file floods/depth100/50 MiB line/near-matches and endless or slow fixture HTTP. Exact-limit/one-over counters, actual termination, partial coverage, existing-install preservation and cleanup. Existing-limit tests can start now; future-limit tests require the resource contracts. Preserve original 50k-tiny-file profile explicitly in performance lane.

**Detailed proposal:**

Add generated, reproducible hostile-input tests tied to a documented table of
ceilings, keeping large performance experiments outside the routine fast suite.

- Generate file floods, 100-level nesting, oversized files and a 50 MiB line in
  temporary directories. Use bounded generators so test setup has its own
  resource ceiling; do not commit huge fixtures.
- Exercise each regex-heavy rule with large near-match input within admitted
  file limits, checking termination and useful diagnostics. Go regex behavior
  does not justify claims that every analyzer has a hard wall-clock guarantee.
- Use fixture HTTP servers and synthetic bodies for never-ending/slow remote
  downloads. Verify bounded reads, cancellation and typed limit errors; close
  every server/body and preserve an existing installation on failure.
- Check exact limits and one-over cases for aggregate bytes, files, findings,
  line length and elapsed deadlines. Assert retained findings, partial Coverage
  reasons and that cancellation stops the work rather than only the waiter.
- Keep baseline file/depth tests runnable before new budget features land.
  Record current behavior honestly; do not assert future limits against code
  that has no implementation of them.
- Add larger flood and per-rule timing/allocation benchmarks in an explicit
  performance lane. Use generous termination guards for CI variability and
  deterministic counters for the actual policy boundary.

Use `internal/audit/audit.go`, `internal/audit/parallel.go`,
`internal/install/github_download.go` and `internal/install/gitea_download.go`
as entry points. Documentation should list each ceiling, its enforcement stage,
fixture, units and expected exhaustion result.

Tests of new install ceilings genuinely require the remote-install budget
implementation. Tests of new audit ceilings require configurable scan budgets;
Coverage assertions also require the completeness contract. Existing-limit
fixtures can start now, but do not complete the full requested guarantee suite.
Archive bomb/forged-count tests belong with archive inspection if that feature
lands; archive parsing is not a prerequisite for the baseline audit/install suite.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines), plus generated data and documentation
- [ ] Large (10+ files, 500+ lines)

Estimate 250–450 test lines initially, with additional cases as limit contracts
settle. This proposal supplies no tests or measurements; it defines the full
suite and its prerequisites, not a claim that fixture-only checks finish it.

**Additional open questions**

- Which benchmarks belong in scheduled CI versus an explicit local lane?
- What termination margin is acceptable without making timing the policy oracle?
- Can the audit/install limit contracts expose deterministic accounting for tests?
- Should archive-bound fixtures become a separate optional extension of this suite?

**NVIDIA relationship:** Direct adversarial archive preflight tests; tree, regex and HTTP tests differ.

[tests/nodes/test_nested_artifacts.py:799–870](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/test_nested_artifacts.py#L799-L870) (`test_archive_member_budget_is_shared_across_outer_archives`, `forbidden_zipfile`)

</details>

## New things to inspect

### 5. Inspect bundled MCP configurations

Scan `.mcp.json` and similar bundled MCP configs for unpinned launchers, remote servers and broad env passthrough, with credentials redacted.

**Decision:** Which launcher versions, transports and environment forwarding policies should be supported?<br>
**Size:** Large<br>
**Earlier discussion:** #688, issue #687 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Explicit hidden MCP basenames; structured pinned launchers including scoped packages, npx/bunx/pnpm/uvx/pipx/Docker, transport/host policy, credential references vs broad env passthrough, locations, redaction before all reports/logs, controls, malformed/unsupported accounting. Include generic pip/npm latest and range forms; policy remains subject to approval.

**Detailed proposal:**

Add a structured MCP configuration analyzer for explicitly supported bundled
files, such as `.mcp.json`, `mcp.json` and `claude_desktop_config.json`.
Reuse compatible parsing from `internal/mcp` where the package boundaries
allow it, and register the findings through the existing audit engine.

- Inspect launcher command/argument arrays for unpinned `npx`/`bunx`/`pnpm dlx`,
  `uvx`/`pipx run` and Docker images. Parse scoped package names and versions
  rather than relying on unsupported regex lookahead or a blanket `@` test.
- Report remote server URLs according to an explicit transport/host policy.
  Do not classify every remote MCP connection as malicious or make network
  requests while auditing configuration.
- Inspect environment forwarding and permissions. Distinguish intentionally
  declared credential references from broad or unexplained passthrough.
- Include server name and JSON location in findings. Redact credentials and
  URL userinfo from snippets, arguments and diagnostic messages before they
  reach any report format or audit log.
- Support rule listing, disabling and severity overrides consistently with
  existing analyzers. Report malformed or unsupported configuration explicitly
  instead of presenting it as an inspected, clean server definition.

The analyzer would use bounded reads and explicit file discovery, including
the named hidden files. It would not read the user's real MCP configuration,
launch servers, install packages, resolve credentials or contact remote hosts.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines), including complete regression coverage

The complete change includes parser integration, bounded collection, findings,
redaction, policy and documentation consumers. Fixture-only tests should pair
pinned/unpinned launchers, legitimate remote servers, declared credentials and
unexplained passthrough, malformed files and redaction cases. No analyzer is
implemented by this proposal.

**Additional open questions**

- Which bundled file names and configuration versions should be supported?
- Which launcher/version forms count as sufficiently pinned, and at what severity?
- Should remote-host allowlists be optional policy, and how should trusted private
  HTTP connections relate to Skillshare's existing MCP transport options?
- Which environment-forwarding cases warrant findings without flagging normal
  intentional credential references as inherently unsafe?
- How should parse failures be represented until a general scan-completeness
  contract is available?

**NVIDIA relationship:** Direct unpinned launcher and permissions analysis; remote URL, env and redaction policy are separate.

[src/skillspector/nodes/analyzers/mcp_rug_pull.py:212–275](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/mcp_rug_pull.py#L212-L275) (`_operand_has_version_pin`, `_iter_config_commands`) · [src/skillspector/nodes/analyzers/mcp_least_privilege.py:285–330](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/mcp_least_privilege.py#L285-L330) (`_detect_capabilities`, `_permission_category`)

</details>

### 6. Inspect bundled hooks and settings

Scan bundled `hooks.json` and `.claude/settings*.json` for auto-run commands, network calls, broad permissions and command-valued helpers.

**Decision:** Which bundled declaration forms, severities and activation assumptions are acceptable?<br>
**Size:** Medium<br>
**Earlier discussion:** #721, issue #714 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** hooks.json/hooks/hooks.json/.claude settings and local settings; schema/event commands, network/POST distinction, permission wildcards/bypass, apiKeyHelper/statusLine, JSON evidence and redaction, explicit parse gaps. Proposed MEDIUM hooks/HIGH network/helper policy remains a question. No live settings read or execution.

**Detailed proposal:**

Add a skill-scope analyzer for an explicit list of bundled configuration paths:
`hooks.json`, `hooks/hooks.json`, `.claude/settings.json` and
`.claude/settings.local.json`. Inspect skill content, not live user settings.

- Collect these paths explicitly with bounded reads and existing source-root/link
  protections. Hidden-directory support must not turn into an unrestricted crawl
  of `.git`, unrelated hidden trees or files outside the audited skill.
- Parse supported hook/event structures and report lifecycle auto-run commands.
  Propose MEDIUM for an executable hook, escalating to HIGH for network-capable
  commands or non-loopback destinations; distinguish a capability from proven
  exfiltration and show the command evidence.
- Recognize remote POST behavior separately from merely mentioning a URL.
  Inspect broad permission entries such as Bash wildcards and bypass modes.
- Report command-valued helper fields such as apiKeyHelper and statusLine at
  proposed HIGH severity, only where the supported schema gives them that meaning.
- Attach file, event/helper identity and JSON path to findings. Redact credentials
  before snippets, diagnostics, logs or any report formatter receives them.
- Emit an explicit informational parse/unsupported-shape diagnostic for malformed
  or unsupported files. Such a diagnostic must not imply the surface was clean.
- Register rule IDs for listing, disabling and severity overrides, and display
  findings through normal CLI, API and dashboard audit reports.

Integration points are `internal/audit/audit_scan_skill.go` and
`internal/audit/analyzer_registry.go`. Managed-hook save/sync auditing is separate;
this bundled-content analyzer has no dependency on that proposal.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines)
- [ ] Large (10+ files, 500+ lines)

Estimate may expand for collection, redaction and schema fixtures. Cover all
named locations, event variants, remote POST, benign local hooks, permissions,
helper fields, malformed JSON, size limits and credential-safe reports.
No hook is executed and no analyzer is implemented by this proposal.

**Additional open questions**

- Which hook/settings schema versions and event forms should be supported first?
- Should all lifecycle hooks be MEDIUM, or only events with automatic execution?
- Is HIGH appropriate for every network-capable command, including benign fetches?
- How should informational parse failures affect gating under stricter policy?

**NVIDIA relationship:** Direct hook declarations and settings tests; severity and invalid input policy differ.

[src/skillspector/nodes/analyzers/bundled_execution_surface.py:669–741](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/bundled_execution_surface.py#L669-L741) (`_hook_declaration`, `_hook_group_declarations`) · [tests/nodes/analyzers/test_bundled_execution_surface.py:1113–1175](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/analyzers/test_bundled_execution_surface.py#L1113-L1175) (`test_bh3_closed_permission_table`)

</details>

### 7. Identify disguised executables and bytecode

Read a small header from each file to catch executables and Python bytecode hidden under document, image or hidden names.

**Decision:** Which formats and uncertain headers should be reported?<br>
**Size:** Medium<br>
**Earlier discussion:** #726, issue #722 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Bounded 512-byte header classification before extension/size filters; ELF/Mach-O/validated PE/Python magic/WASM/ZIP/JAR and disguised shebangs; deliberate hidden/cache traversal; maintained Python format table, uncertain headers and separate skipped-binary counts. Header classification is not runtime analysis.

**Detailed proposal:**

Inspect a bounded header before ordinary text size/extension filtering, with
explicit hidden-file and `__pycache__` coverage inside the audited skill.

- Read at most 512 bytes per eligible file and classify ELF, Mach-O, PE, Python
  bytecode, ZIP/JAR and WASM signatures. Validate enough structure to distinguish
  PE from a file that merely starts with MZ; report uncertain/truncated headers.
- Recognize shebang scripts under document, image, JSON or hidden names. Classify
  extension/signature mismatches without treating every executable bit as proof.
- Propose HIGH for executables disguised as documents/images or hidden payloads,
  MEDIUM for other executable artifacts and HIGH for Python bytecode. Treat
  ZIP/JAR as archive classification rather than proof of executable behavior.
- Use a pinned, maintained Python magic-number table with documented supported
  versions. Report unknown bytecode-like headers without claiming a full decode.
- Traverse the required hidden content and cache paths deliberately, retaining
  `.git`, internal-metadata, depth and source-boundary exclusions. Do not follow
  new external links just to inspect headers.
- Keep binary/skipped counts separate from auditable text bytes and show them in
  CLI, JSON and dashboard summaries. A classified binary has not been analyzed
  for its runtime behavior, and an oversized file has only had its header read.

The collection and current NUL check are in `internal/audit/audit_scan_skill.go`;
`internal/audit/audit_result.go` defines current analyzability reporting.
No prerequisite feature is required. Shared collector changes are merge risks.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines)
- [ ] Large (10+ files, 500+ lines)

Estimate includes header classification, deliberate hidden/cache traversal,
summary consumers, Python data maintenance and synthetic byte fixtures. Cover
renamed binaries, legitimate archives, shebang mismatches, large files, truncated
headers and known/unknown bytecode. No executable scanner is implemented here.

**Additional open questions**

- Which document/image extensions and hidden executable cases should count as disguise?
- Which Python versions should be recognized, and how are magic updates reviewed?
- Should ordinary bundled native executables be MEDIUM by default or informational?
- What total entry/header-read budget should complement existing depth limits?

**NVIDIA relationship:** Direct executable classification and bytecode/document tests; sniff limits differ.

[src/skillspector/nested_artifacts.py:496–525](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nested_artifacts.py#L496-L525) (`has_binary_executable_magic`, `is_executable_content`) · [tests/nodes/test_nested_artifacts.py:206–215](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/test_nested_artifacts.py#L206-L215) (`test_bytecode_formats_are_executable_content`) · [tests/nodes/test_nested_artifacts.py:668–710](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/test_nested_artifacts.py#L668-L710) (`test_hidden_disguised_document_inventories_nested_executable_locally`)

</details>

### 8. Inspect archive members

Look inside ZIP, wheel and Office files (no extraction to disk) with member, size, ratio and depth limits.

**Decision:** Which containers, member limits and incomplete-coverage policies should apply?<br>
**Size:** Large<br>
**Earlier discussion:** #707, issue #702 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** ZIP/wheel/Office signature+structure admission, virtual member paths/nesting, no disk extraction; depth3/1000 members/25 MiB expanded+compressed/1 MiB member/100:1/5s proposals counted on actual bytes; traversal/symlink rejection, encrypted/corrupt/unsupported/limit coverage, concealed executable severity. Own limits can ship jointly; global budgets are optional reuse.

**Detailed proposal:**

Add bounded ZIP-container inspection with virtual member paths, without
extracting member files to disk or executing their contents.

- Discover eligible containers by ZIP signatures within an explicit bounded
  read policy, including `.zip`, `.whl`, `.docx`, `.xlsx` and `.pptx` inputs.
  Do not rely on the current text-extension filter or filename alone.
- Use Go's ZIP reader and validate the archive's structure. Magic bytes select
  a parser; they do not prove a valid ZIP or an Office document.
- Identify supported Office packages from their structure. Scan text members
  through the existing content rules and attribute findings to virtual paths
  such as `outer.zip!/scripts/setup.sh`, including nested containers.
- Propose shared per-skill ceilings of depth 3, 1,000 members, 25 MiB of
  expanded content, 1 MiB per member, expansion ratio 100:1 and 5 seconds.
  Also cap admitted compressed input at 25 MiB; define exact units in policy.
- Account against actual bytes read across nested archives, not only declared
  sizes/counts. Stop before further allocation or decompression exceeds a cap.
  Apply cooperative deadline checks to parsing and member reads.
- Reject absolute/traversal member names and symlink members. Report encrypted,
  corrupt, unsupported or over-budget content as uninspected with a reason.
- Propose a HIGH concealed-executable finding for executable/script content in
  document containers. Ordinary archive packaging alone is not malicious;
  Office macros and arbitrary binary code need an explicit supported policy.

Integrate through `internal/audit/audit_scan_skill.go`,
`internal/audit/audit_scan_content.go` and `internal/audit/audit_result.go`.
The archive feature must own its limits and inspection status. A future global
scan budget can coordinate accounting, but is not a prerequisite.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines), including collection and result consumers

Estimated implementation: 450–800 production lines plus generated fixtures for
nested archives, Office packages, forged metadata, encrypted/corrupt members,
unsafe names and expansion limits. Test actual accounting and partial results.
This proposal includes no parser implementation or claim of complete binary analysis.

**Additional open questions**

- Approve the proposed ceilings and their configuration/override policy?
- Which document member types justify HIGH severity, including macros?
- Should partially inspected archives block installation by default?
- How should self-extracting ZIPs and unsupported compression be reported?

**NVIDIA relationship:** Direct bounded nested inspection and preflight tests; Skillshare limits and reporting differ.

[src/skillspector/nested_artifacts.py:46–51](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nested_artifacts.py#L46-L51) (`ARCHIVE_MAX_DEPTH`, `ARCHIVE_MAX_MEMBERS`) · [src/skillspector/nested_artifacts.py:1195–1228](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nested_artifacts.py#L1195-L1228) (`inspect_nested_artifacts`) · [tests/nodes/test_nested_artifacts.py:799–845](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/test_nested_artifacts.py#L799-L845) (`test_archive_member_budget_is_shared_across_outer_archives`, `forbidden_zipfile`)

</details>

### 9. Inspect dependency source redirection

Flag commands and shipped config that point npm, pip, Poetry, Yarn, Cargo or Maven at a different registry.

**Decision:** How should private registries, unknown sources and credential evidence be handled?<br>
**Size:** Large<br>
**Earlier discussion:** #731, issue #727 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Complete npm/pip/Poetry/Yarn/Cargo/Maven command, env, heredoc and shipped-config redirection family including hidden .cargo/.npmrc and XML/.conf; default/private-registry policy, bounded literal resolution, unknown destinations, credential-safe all-format evidence and prose negatives. credentials.go is not a general redactor.

Examples to evaluate include `--registry`, `--index-url`, `--extra-index-url`, package-manager configuration commands and shipped configuration files; whether a destination is trusted remains policy-dependent.

**Detailed proposal:**

Add a dependency-source-redirection family covering command text and explicitly
supported bundled configuration, with proposed HIGH severity for unexpected sources.

- Cover npm config/registry flags, pip index and extra-index flags/config, Poetry
  sources, Yarn configuration and direct git+http dependency sources.
- Recognize PIP_INDEX_URL, NPM_CONFIG_REGISTRY and Cargo registry-index environment
  settings. Resolve only simple literal variable assignments in a bounded local
  context; report unresolved destinations as unknown rather than inventing values.
- Inspect redirection and heredoc writes to `.npmrc`, `pip.conf`, `.yarnrc.yml`,
  `.cargo/config.toml` and `settings.xml`; include declared sources in
  `pyproject.toml`. Recognize the destination setting rather than any URL in a file.
- Explicitly collect these basenames and hidden locations under size/depth and
  source-boundary limits. `.conf`, XML and hidden `.cargo` cannot rely solely
  on current extension-based collection.
- Maintain package-manager default destinations and allow explicit trusted-source
  overrides. A private registry can be intentional; findings should explain the
  changed source and never label every non-default registry malicious.
- Redact URL userinfo, tokens and sensitive query/config values before snippets,
  errors, logs or report output. `internal/audit/credentials.go` generates
  credential-access patterns; it is not a reusable general redactor.
- Expose normal rule disabling, severity control and CLI/API/dashboard findings.
  Pair executable/config examples with prose and comment negatives.

Relevant source is `internal/audit/rules.yaml`,
`internal/audit/audit_scan_skill.go` and `internal/audit/credentials.go`.
No prerequisite proposal is required; bounded collection/redaction may be reused.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate includes the complete command/config family, collection, destination
policy, bounded literal resolution, redaction, reporting, documentation and
fixtures. Cover defaults, private sources, comments, unresolved variables,
heredocs and secrets in every output format. No implementation is included.

**Additional open questions**

- Which package-manager versions and configuration shapes should be supported first?
- Should trusted-source policy be global, project-local or both, and how do they combine?
- Should unresolved command destinations be MEDIUM while concrete unexpected sources are HIGH?
- How should direct VCS dependencies differ from persistent registry configuration changes?

**NVIDIA relationship:** Direct SC10 and config/command tests; Skillshare inputs and redaction differ.

[src/skillspector/nodes/analyzers/static_patterns_supply_chain.py:16–26](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/static_patterns_supply_chain.py#L16-L26) · [tests/nodes/analyzers/test_dependency_sources.py:58–134](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/analyzers/test_dependency_sources.py#L58-L134) (`test_supported_direct_configuration_surfaces_cover_all_ecosystems`) · [tests/nodes/analyzers/test_dependency_sources.py:375–425](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/analyzers/test_dependency_sources.py#L375-L425) (`test_generated_configs_support_pip_poetry_maven_and_cargo`)

</details>

### 10. Detect sensitive destinations and unsafe deserialization

Flag requests to cloud metadata and internal addresses, and unsafe deserializers like `pickle.load` or `yaml.load`, with safe forms as negatives.

**Decision:** Which language applicability and severities should apply to destination and deserialization rules?<br>
**Size:** Medium<br>
**Earlier discussion:** issue #706 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Validated per-rule files glob applicability through load/override/list/raw/skill paths, PHP collection, cloud metadata/internal/dynamic target evidence, Python/PHP/Ruby/JS unsafe deserializers with call-local safe YAML/torch negatives. No runtime SSRF guarantee. Retain metadata-host matching in any admitted text file, independently of language filters.

**Detailed proposal:**

Extend existing static rules with validated file applicability, then add
network-target and deserializer families with explicit safe-use negatives.

- Add a proposed `files` glob field with documented relative-path semantics and
  validation. Carry it through rule loading, overrides, listing, raw-content
  scans and skill scans; reject malformed applicability rather than ignoring it.
- Collect PHP scripts under existing bounded scan rules. Limit language-specific
  findings to the intended extensions instead of matching ordinary prose as code.
- Propose CRITICAL findings for recognized cloud metadata destinations and MEDIUM
  for loopback/private/internal request targets. Include dynamic request-target
  heuristics with lower confidence where the destination cannot be established.
- Cover Python pickle, marshal, dill, unsafe YAML loading and torch loading
  without the approved safe option; PHP unserialize; Ruby Marshal.load; and
  JavaScript node-serialize. Propose HIGH for unsafe deserialization evidence.
- Treat safe_load, an explicit SafeLoader and the approved weights-only form as
  negatives only for the matching call. A safe call elsewhere must not suppress
  an unsafe one on the same line or file.
- Use Go-compatible matching and narrow call/argument inspection where needed.
  Do not promise regex can resolve aliases, data provenance, runtime safety or
  arbitrary multiline syntax. Report evidence and uncertainty explicitly.

Relevant source is `internal/audit/patterns.go`, `internal/audit/rules.yaml`,
`internal/audit/audit_scan_content.go` and `internal/audit/audit_scan_skill.go`.
No other proposal is required; concurrent rule-schema work is a merge risk.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines)
- [ ] Large (10+ files, 500+ lines)

Estimate is provisional and may grow beyond 500 lines across consumers and
fixtures. Include applicability validation/listing, network target cases,
dynamic targets, every listed language and paired safe/unsafe calls. No network
requests, deserialization execution or implementation is included here.

**Additional open questions**

- Which metadata host aliases and IPv4/IPv6 forms belong in the initial policy?
- Should metadata URLs in documentation retain CRITICAL or use contextual severity?
- Which dynamic-target evidence is strong enough to report without noisy URL variables?
- Which exact YAML and torch safe forms should be recognized and documented?

**NVIDIA relationship:** Direct pattern and language dispatch inspiration; globs and severities differ.

[src/skillspector/nodes/analyzers/static_patterns_ssrf.py:33–70](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/static_patterns_ssrf.py#L33-L70) (`ANALYZER_ID`, `SSRF1_ENDPOINT_PATTERNS`) · [src/skillspector/nodes/analyzers/static_patterns_deserialization.py:115–148](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/static_patterns_deserialization.py#L115-L148) (`_language_for`, `analyze`)

</details>

### 11. Inspect offline dependency manifests

Parse requirements, package and Docker manifests offline for unpinned versions, near-miss package names and disabled container trust.

**Decision:** How should package datasets be curated and trusted-container bypasses classified?<br>
**Size:** Large<br>
**Earlier discussion:** #740, issue #737 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Offline requirements/pyproject/package+lock/Dockerfile/compose parsing; ecosystem-specific exact pins LOW, pinned popular-package dataset and distance<=1 typosquat MEDIUM with namespace/benign guards, explicit container trust bypass HIGH; named collectors, rules/locations/UI and dataset maintenance/license questions.

Examples of container trust bypass to assess include `--disable-content-trust` and `--insecure-registry`; supported forms and severities require approval.

**Detailed proposal:**

Add an offline manifest analyzer with explicit supported file families.

- Parse requirements*.txt, supported pyproject sections, package.json/lockfiles
  and Dockerfile/compose forms; document which ecosystems and version syntax
  are understood and report unsupported content without a clean-coverage claim.
- Report unpinned dependencies at proposed LOW severity using ecosystem-specific
  exact-pin semantics, including relevant install command forms.
- Compare package names against a pinned, provenance-documented popular-package
  dataset with proposed Damerau-Levenshtein distance<=1. Respect namespaces,
  normalization and legitimate near-name packages; propose MEDIUM, not proof.
- Detect explicit disabled container trust/content verification options at
  proposed HIGH severity, distinguishing them from ordinary unsigned references
  or prose mentioning a flag.
- Collect named manifests/config basenames deliberately under existing bounds;
  do not rely only on currently scannable text extensions.
- Register rule IDs and policy overrides, attach package/file/line evidence and
  surface findings consistently through CLI, API and dashboard reports.
- Keep all checks offline; dataset generation/update review is a maintenance
  process, not a runtime fetch. Parser sharing with OSV lookup is optional reuse.

Relevant source: `internal/audit/rules.yaml`, `internal/audit/audit_scan_skill.go`.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate 400–750 production and 300–550 test lines plus versioned package
data and its update process; no prerequisite feature.
Test exact/range pins, multiple ecosystem forms, namespaces, legitimate similar
names, trust flags, prose negatives, unsupported formats and stable locations.
No implementation, test result or maintainer approval is claimed.

**Additional open questions**

- Which ecosystems and manifest versions should ship first?
- Who maintains/licenses the popular-package data and approves updates?
- Which explicit container trust bypasses warrant HIGH severity?

**NVIDIA relationship:** Direct conceptual SC1, typosquat and container verification bypass; exact parsers differ.

[src/skillspector/nodes/analyzers/static_patterns_supply_chain.py:16–25](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/static_patterns_supply_chain.py#L16-L25) · [src/skillspector/nodes/analyzers/static_patterns_supply_chain.py:425–440](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/static_patterns_supply_chain.py#L425-L440) (`SC7_PATTERNS`) · [src/skillspector/nodes/analyzers/static_patterns_supply_chain.py:525–540](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/static_patterns_supply_chain.py#L525-L540)

</details>

### 12. Compare declared and observed capabilities

Compare a skill's declared `allowed-tools` with what its scripts actually do, under the strict profile only.

**Decision:** What constitutes sufficient observed evidence, and how should unknowns and wildcard permissions be scored?<br>
**Size:** Large<br>
**Earlier discussion:** #736, issue #732 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Strict-only allowed-tools declaration parsing/target capability mapping vs observed non-test script evidence; undeclared HIGH/MEDIUM, wildcard MEDIUM, unused LOW, missing/malformed declarations, confidence/unknowns, interaction-based noise suppression and complete-coverage question. Optional shared extraction with update consent does not make it a prerequisite.

**Detailed proposal:**

Add a skill-scope capability comparison, initially active only under the
strict audit profile. Treat results as static evidence rather than enforcement
of what a target agent can actually execute.

- Parse SKILL.md `allowed-tools` through the general frontmatter parser. Define
  accepted scalar/list forms, target-specific names and malformed declarations.
- Publish an explicit mapping from supported tool declarations and scoped tool
  arguments to shell, network, file and environment capabilities. Do not assume
  that a command tier or one target's tool name grants the same permission elsewhere.
- Extract used capabilities from non-test bundled scripts using compatible tier
  and dataflow evidence, extended where they lack the required categories.
  Retain file/line evidence and confidence; unknown dynamic use stays unknown.
- Emit undeclared-capability findings at HIGH for strong evidence and MEDIUM for
  uncertain evidence, wildcard permission at MEDIUM and unused declarations at LOW.
  Include a distinct missing-declaration diagnostic rather than treating absence
  as an explicit empty permission list.
- Define interaction-based suppression: once a missing declaration is reported,
  suppress duplicate per-category undeclared noise; broad wildcard grants cover
  their mapped capabilities but retain the wildcard finding. Do not suppress an
  unrelated strong undeclared capability because another heuristic matched.
- Exclude documented test/fixture paths and do not call absence of observed use
  proof that a permission is unnecessary. Display findings consistently across
  CLI, API and dashboard, with normal rule controls and strict-profile gating.

Useful foundations are `internal/utils/frontmatter.go`, `internal/audit/tiers.go`
and `internal/audit/policy.go`. Current tiers are optional evidence reuse, not a
complete capability mapper. This proposal has no prerequisite feature.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate includes declaration/target semantics, extraction, interaction policy,
strict activation, reports, docs and fixtures. Cover clean, underdeclared,
overdeclared, wildcard, missing and malformed cases, dynamic uncertainty,
test exclusion and duplicate suppression. No capability analyzer is implemented here.

**Additional open questions**

- Which target tool names and scoped argument forms should the first mapping support?
- What severity should a missing declaration have under strict policy?
- Which evidence justifies HIGH rather than MEDIUM for undeclared capability?
- Should unused-permission findings require complete script coverage before emission?

**NVIDIA relationship:** Direct capability and permission mapping; Skillshare tiers and profile differ.

[src/skillspector/nodes/analyzers/mcp_least_privilege.py:246–310](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/mcp_least_privilege.py#L246-L310) (`_normalize_allowed_tools`, `_detect_capabilities`) · [src/skillspector/nodes/analyzers/mcp_least_privilege.py:330–398](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/mcp_least_privilege.py#L330-L398) (`_map_permissions_to_categories`, `_map_allowed_tools_to_categories`)

</details>

### 13. Check bundled references

Extend the dangling-link check to more path forms and flag references that escape the skill directory.

**Decision:** Which references are intentional external exceptions, and should escapes be findings?<br>
**Size:** Medium

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Extend existing dangling links to links/images/backticked/fenced path forms, bounded lstat containment incl parent/absolute/symlink components, broken LOW/escape HIGH proposal, explicit intentional-external exceptions and doctor parity. Avoid following paths outside the declared root.

**Detailed proposal:**

Extend the existing local-reference checker and share its result contract.

- Recognize relative Markdown links/images plus explicitly defined backticked
  or fenced path forms; exclude URLs, anchors and ordinary identifiers.
- Inspect local paths with lstat-based component handling, preserving original
  source evidence and avoiding following external targets merely to inspect them.
- Define root containment for parent traversal, absolute paths and symlinked
  components. Report proposed HIGH reference-escapes-root and LOW broken-reference
  findings with stable rule identity and rule controls.
- Distinguish a deliberately documented external path from an instruction that
  requires an out-of-bundle resource; agree explicit exceptions and severity
  before treating every absolute example as a security failure.
- Reuse the same checker in doctor and audit, with each caller's normal output,
  errors and configuration. Preserve current dangling-link behavior compatibly.
- Bound path parsing/resolution and report unsupported or unreadable checks
  honestly. Do not read referenced personal files or execute referenced code.

Relevant source: `internal/audit/markdown_links.go`, `internal/audit/analyzer_structure.go`.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines)
- [ ] Large (10+ files, 500+ lines)

Estimate 130–230 production plus 140–240 test lines across parser, checker,
doctor integration and documentation; full scope exceeds a minimal link fix.
Use synthetic in-root/out-of-root link trees only; cover missing members,
symlinked parents, absolute examples, backticks, images, controls and doctor parity.
No implementation, test result or maintainer approval is claimed.

**Additional open questions**

- Which backticked/fenced forms unambiguously denote required local paths?
- How should intentional external references be distinguished or exempted?
- Should audit and doctor use identical severity and error treatment?

**NVIDIA relationship:** Direct normalization and no-follow existence check; escape findings and doctor integration differ.

[src/skillspector/references.py:429–475](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/references.py#L429-L475) (`_normalize_candidate`, `_target_exists_without_following_symlinks`)

</details>

## Fewer false negatives and false positives

### 14. Normalize Unicode for matching

Match rules against a Unicode-normalized view too, so confusables and invisible characters can't dodge them, with locations mapped back.

**Decision:** Which normalization and mixed-script heuristics should be enabled?<br>
**Size:** Large<br>
**Earlier discussion:** #705, issue #699 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Raw plus NFKC/UTS39/default-ignorable view, versioned licensed data and original-offset mapping through expansion/removal; same-span dedup/context/overrides, ASCII fast path, separately disableable mixed-script heuristic. Retain multilingual negatives and <15% benchmark acceptance target, not a measured claim.

**Detailed proposal:**

Scan raw text and a separate security view for injection, exfiltration,
destructive-command and shell patterns. Never rewrite installed content.

- Apply NFKC, a versioned UTS #39 confusable skeleton and an explicit
  default-ignorable policy. NFKC alone is not confusable detection.
- Preserve an offset map through expansion, contraction and removal so snippets
  and locations refer to the original bytes and lines, including combining marks.
- Deduplicate raw/normalized matches for the same rule and source span, retaining
  evidence that the normalized view matched. Keep existing Markdown context and
  rule overrides consistent across both views.
- Skip view construction for pure ASCII where equivalent. Document the Unicode
  version, table provenance, generation process and update/review procedure.
- Add a separately disableable mixed-script identifier heuristic. Use explicit
  script exceptions and conservative severity rather than flagging multilingual
  prose as an attack merely because it contains several scripts.
- Validate Cyrillic-confusable instruction text, soft-hyphen splits, compatibility
  forms, legitimate multilingual text and stable locations. Target less than 15%
  scan slowdown on a documented representative benchmark; this is an acceptance
  target to measure, not a demonstrated performance claim.

Integration points are `internal/audit/audit_scan_content.go`,
`internal/audit/audit_scan_skill.go` and `internal/audit/dedupe.go`.
`go.mod` already declares `golang.org/x/text`; generated confusable data is new.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate includes matching consumers, mapped locations, generated data,
mixed-script policy, documentation and benchmarks. Handwritten logic may be
smaller, but maintained Unicode data and coverage are part of the feature.
No normalization implementation or benchmark result is included here.

**Additional open questions**

- Which Unicode version and source/license should the generated table pin?
- Which default-ignorable characters should be removed in which script contexts?
- Should normalized-only matches inherit severity or start at reduced confidence?
- Which mixed-script identifiers warrant findings without overwhelming normal skills?

**NVIDIA relationship:** Direct normalized view; rule selection and source offsets differ.

[src/skillspector/artifacts.py:1882–1912](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/artifacts.py#L1882-L1912) (`normalized_security_view`)

</details>

### 15. Detect abusive triggers

Flag overly broad, keyword-stuffed or shadowing trigger descriptions, including near-duplicates of installed skills.

**Decision:** Which trigger inventories and similarity thresholds should be used?<br>
**Size:** Large<br>
**Earlier discussion:** #720, issue #709 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Complete multiline frontmatter, broad-trigger/stuffing/target command inventory and deterministic installed-description similarity index, self-update replacement, absent-context disclosure, controls and CLI/API/UI parity. Inventory maintenance/similarity thresholds stay approval questions.

**Detailed proposal:**

Extend metadata analysis with broad-trigger and keyword-stuffing checks, plus
command and installed-skill shadow comparisons. Propose MEDIUM findings with
stable rule IDs, individual disable support and CLI/dashboard visibility.

- Parse complete name/description frontmatter, including multiline descriptions,
  before applying trigger heuristics. Do not infer intent from length alone.
- Report explicit all-request triggers and repeated unrelated trigger terms;
  pair each heuristic with ordinary broad utility and detailed-description negatives.
- Define a versioned built-in command inventory tied to supported target tools.
  Match invocation-like names in their target context rather than every common word.
- Compare descriptions/names against other installed skills using a deterministic
  local index and documented similarity threshold. Exclude self-comparisons and
  identify the competing skill and evidence without choosing an installation winner.
- For install/update, compare candidate content to the existing installed set,
  replacing its own previous entry. Offline path-only audits should report when
  this comparison context is absent rather than imply a complete shadow check.
- Keep cross-skill findings consistent across CLI, API and dashboard audit paths;
  allow rule disabling and severity overrides through existing audit controls.

`internal/audit/analyzer_metadata.go` handles current metadata checks.
`internal/audit/analyzer_crossskill.go` provides bundle analysis scope, but its
current result-based analysis is not an installed-description index.
Reuse that scope where useful; no prerequisite analyzer is required.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate includes frontmatter handling, target command inventory, installed-set
context, candidate comparison, reports, configuration and fixtures. Test benign
utility descriptions, stuffing, command matches, near matches, self-updates and
missing comparison context. The design extends existing metadata analysis; implementation follows separately.

**Additional open questions**

- Which target command inventories should ship first, and who maintains them?
- What similarity and stuffing thresholds keep false positives acceptable?
- Should a missing installed-set context produce an informational diagnostic?
- Should explicitly related skill variants be exemptible through normal rule overrides?

**NVIDIA relationship:** Direct trigger family; cross-skill comparison and thresholds are new.

[`_analyze_triggers`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/static_patterns_supply_chain.py#L3368-L3609)

</details>

### 16. Inspect metadata by field

Scan metadata field by field (names, descriptions, parameters, defaults) for hidden text, encoded payloads and oversized fields.

**Decision:** Which metadata schemas, base64 thresholds and severities should be supported?<br>
**Size:** Large<br>
**Earlier discussion:** #733, issue #728 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Field-aware identifier/description/trigger/parameter/default scanning with supported JSON/YAML schema paths; confusables/invisibles, bounded candidate base64 decode, hidden comments, >500-character parameter proposal and nonlocal/executable defaults; original locations, redaction and unsupported coverage. The proposed 50-character base64 floor and HIGH override-phrase severity remain maintainer decisions.

**Detailed proposal:**

Extend metadata analysis using the general frontmatter parser and an explicit
set of supported adjacent tool-schema locations.

- Check skill/tool identifiers for invalid forms, invisibles, mixed scripts and
  confusables using documented conservative script exceptions.
- Inspect name, description, trigger text and parameter descriptions in their
  field context; detect override language and hidden-comment instructions.
- Decode candidate base64 text only above a defined threshold and within strict
  input/output/depth limits; inspect valid UTF-8 results, preserve evidence and
  never execute decoded content. Long benign encodings need negative fixtures.
- Flag overly long parameter descriptions (proposed 500 characters) and defaults
  containing nonlocal destinations or executable curl/wget/bash-c commands,
  only where the supported schema actually gives them default-value meaning.
- Register stable IDs with normal listing, disable and severity controls;
  attach field/JSON path and original source locations through all reports.
- Distinguish unsupported/malformed schema coverage from an inspected clean
  field; redact secrets before snippets or parser diagnostics escape.
- Reuse a maintained confusable helper if available, but this feature is not
  dependent on a separate normalized full-content scan.

Relevant source: `internal/audit/analyzer_metadata.go`, `internal/utils/frontmatter.go`.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate 300–550 production and 250–450 test lines plus maintained Unicode
data if required; supported schemas and collection are part of the scope.
Cover clean multilingual identifiers, invisible/confusable cases, bounded
decoding, field severity, defaults, unsupported schemas and report redaction.
No implementation, test result or maintainer approval is claimed.

**Additional open questions**

- Which tool-schema paths and versions should be recognized initially?
- Which field-specific thresholds and severities avoid routine metadata noise?
- Should schema parse gaps affect strict completeness policy?

**NVIDIA relationship:** Direct field-aware MCP metadata checks; Skillshare schema and policies differ.

[src/skillspector/nodes/analyzers/mcp_tool_poisoning.py:195–230](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/mcp_tool_poisoning.py#L195-L230) (`_extract_metadata_texts`) · [src/skillspector/nodes/analyzers/mcp_tool_poisoning.py:540–565](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/mcp_tool_poisoning.py#L540-L565) (`_check_tp2`) · [src/skillspector/nodes/analyzers/mcp_tool_poisoning.py:738–760](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/mcp_tool_poisoning.py#L738-L760) (`_check_tp3`)

</details>

### 17. Detect whitespace padding

Flag long runs of blank lines or whitespace used to push instructions out of view.

**Decision:** Which whitespace thresholds, exemptions and severities should be used?<br>
**Size:** Medium<br>
**Earlier discussion:** issue #741 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** All padding forms: >=20 blanks MEDIUM/>=40 HIGH, >=80 horizontal whitespace then text, >4 KiB at >=95% whitespace, >=100 repeated lines then text; fences/table exemptions, non-LF Unicode/VT/FF and CRLF location mapping; optimized walker invocation, disable/severity overlays, translated catalogs and threshold/negative tests. No product implementation completed.

**Detailed proposal:**

Add one registered whitespace-padding detector, preserving all detection forms.

- Propose MEDIUM at>=20 consecutive blank lines and HIGH at>=40; report the
  relevant run/location rather than a finding for every blank line.
- Detect>=80 horizontal whitespace characters followed by meaningful text.
- Propose a >4KiB eligible-content density check at>=95% whitespace, and a
  heavy-repetition check at>=100 consecutive identical nonblank lines followed
  by text. These last thresholds need maintainer agreement and measurement.
- Exempt fenced code and Markdown table rows from padding accounting; keep
  ordinary spacing, poetry, tables and long code examples as benign fixtures.
- Treat U+0085/U+2028/U+2029, vertical tab and form feed as line breaks while
  retaining locations in original source. Define CRLF and Unicode whitespace.
- Wire the detector into supported content scans and the optimized skill walker;
  registry registration alone does not make every path invoke it.
- Register stable rule identity, listing, disable and global/project severity
  overrides. Existing synthetic checks primarily use disabled-ID controls,
  while regex rules carry severity overlays, so that plumbing must be explicit.
- Update rule catalogs and reports consistently; explain that a padding warning
  indicates hidden content risk, not proof of malicious intent.

Relevant source: `internal/audit/analyzer_registry.go`, `internal/audit/markdown_context.go`, `internal/audit/audit_scan_skill.go`, `internal/audit/patterns.go`.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines)
- [ ] Large (10+ files, 500+ lines)

Fresh implementation-source assessment estimates 300–450 meaningful code/test
lines plus existing translated catalogs; no code was written.
Cover all thresholds at/one-over, Unicode/CRLF locations, fences/tables, benign
twins, real skill-path invocation, configuration disabling and severity overrides.
No implementation, test result or maintainer approval is claimed.

**Additional open questions**

- Approve density/repetition thresholds and their proposed severity?
- Should density count full source bytes or only nonexempt content?
- How should original-source locations represent non-LF line separators?

**NVIDIA relationship:** Direct thresholds and detector; Skillshare registry and severity differ.

[src/skillspector/nodes/analyzers/whitespace_padding.py:58–65](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/whitespace_padding.py#L58-L65) (`VERTICAL_BLANK_LINES`, `VERTICAL_HIGH_SEVERITY_LINES`) · [src/skillspector/nodes/analyzers/whitespace_padding.py:497–525](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/whitespace_padding.py#L497-L525) (`detect_whitespace_padding`) · [tests/nodes/analyzers/test_whitespace_padding.py:100–125](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/analyzers/test_whitespace_padding.py#L100-L125) (`TestVerticalSignal`)

</details>

### 18. Use Python syntax context

Use Python syntax context so comments and inert examples stop triggering, while real `shell=True` calls still do.

**Decision:** Which Python parsing strategy and conservative fallback should be adopted?<br>
**Size:** Large<br>
**Earlier discussion:** issue #734 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Bounded Python syntax/token strategy retaining executable string arguments; inert comments/examples, proven literal argv without shell, unknown shell=True remains finding, SQL binding evidence; visible parser gaps/conservative fallback and original locations. Alias/getattr/reflection via optional pinned parser remains later scope, not deleted.

**Detailed proposal:**

Settle a bounded syntax strategy before changing suppression behavior.

- First phase: identify comments, string literals and call/argument boundaries
  without executing Python or importing a bundle's dependencies.
- Ignore inert comment/example matches while retaining calls whose arguments
  are strings; blanket removal of every string is explicitly insufficient.
- Suppress subprocess warnings only for a proven literal argv form with shell
  disabled, respecting positional/keyword arguments and unknown dynamic values.
- Preserve findings for shell=True with variable content and unsupported or
  ambiguous constructs; constant shell strings need their own explicit policy.
- Handle bound SQL parameters only when the selected check can establish the
  binding form; never infer data provenance merely from a placeholder string.
- Keep parser failures/unsupported syntax visible and apply conservative fallback
  matching rather than calling an unparsed file safe.
- Optional second phase: evaluate a pinned Python parser such as tree-sitter
  for import-alias and getattr/reflection resolution, with its dependency and
  platform costs stated separately. It is not implied by phase-one tokenization.
- Keep original byte/line locations and existing rule controls/report shapes;
  constrain input, allocation and parser time under the selected scan policy.

Relevant source: `internal/audit/audit_scan_skill.go`, `internal/audit/audit_scan_content.go`.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Feasibility requires a parser/tokenizer decision first. Initial phase estimate
450–900 production and 300–600 test lines; optional phase needs a new estimate.
Use fixtures for comments, multiline strings, literal argv, shell=True variables,
aliases, reflection, SQL bindings, parse errors and stable source locations.
No implementation, test result or maintainer approval is claimed.

**Additional open questions**

- Which parser strategy and supported Python syntax versions are maintainable?
- What exactly proves literal-argv safety and when must a warning remain?
- Should optional alias/reflection analysis justify its new parser dependency?

**NVIDIA relationship:** Direct AST source and deserialization analysis; parser and safe argv design differ.

[src/skillspector/nodes/analyzers/behavioral_ast.py:294–324](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/behavioral_ast.py#L294-L324) (`_is_chain_sink`, `_contains_dangerous_source`) · [src/skillspector/nodes/analyzers/behavioral_ast.py:344–380](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/behavioral_ast.py#L344-L380) (`_deserialization_message`)

</details>

### 19. Support reasoned finding baselines

Let users accept known findings with a required reason, bound to the exact content, instead of blanket ignores.

**Decision:** How should baseline composition, migration and raw/effective scoring work?<br>
**Size:** Large<br>
**Earlier discussion:** #701, issue #695 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Versioned global/project baseline with required reasons for exact fingerprints and explicit glob rules; engine/content/path/rule/severity/location binding, generate/refresh/output/reason/explicit baseline/show-suppressed controls; acceptance before score/exit/all gates, raw findings and SARIF suppression reasons; malformed versions fail, legacy acknowledgements readable/migratable. Filename/composition and raw/effective score policy remain open.

Retain the proposed `audit baseline --refresh` and `--reason` workflows and `audit --baseline FILE --show-suppressed` controls, subject to the open interface and composition decisions.

**Detailed proposal:**

Add a versioned YAML baseline for global and project audit contexts. Propose
`audit-baseline.yaml` beside the active audit configuration, with explicit
file selection available as a proposed CLI option.

- Store generated exact fingerprints and hand-authored path/message glob rules;
  require a nonempty reason on every entry. Keep broad glob acceptance explicit.
- Bind exact entries to engine/rule version, relative file identity, file-content
  digest and finding identity, including severity and source location. Content
  or engine changes must make exact acceptance stale rather than silently carry it.
- Add proposed baseline generate/refresh operations, output-file and reason
  inputs, and audit options for an explicit baseline and suppressed visibility.
  Refresh must require reviewing new findings and supplying reasons.
- Apply acceptance before effective risk scoring, exit decisions and install,
  update and dashboard gates. Preserve raw findings separately for inspection.
- Show suppressed findings and reasons in CLI and dashboard reports; expose
  them in JSON and SARIF suppression records. Reject malformed baselines and
  unknown versions with a nonzero result rather than silently ignoring them.
- Keep existing acknowledgements readable during transition, distinguish their
  weaker identity in reports and offer explicit migration rather than deletion.

Relevant source is `internal/audit/accept.go`, `internal/audit/audit_result.go`,
`internal/install/audit_accept.go` and `internal/server/handler_audit.go`.
This is a new persistence and CLI contract with no prerequisite feature.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate includes format validation, generation, refresh/migration, all gating
and reporting consumers, documentation and fixture tests. Cover edited content,
engine changes, stale entries, glob reasons, bad versions and consistent scores
and exits. The proposal does not implement a baseline or change current acceptance.

**Additional open questions**

- Should an engine change invalidate all exact entries or only changed rule versions?
- Should glob entries require explicit renewal on engine changes as well?
- How should global and project baselines combine, and which file wins conflicts?
- Should effective and raw risk scores both be displayed during migration?

**NVIDIA relationship:** Direct content keys, fingerprints, required reasons and scanner-version binding. Per-entry rule-version binding, global/project composition and migration remain Skillshare design decisions.

[src/skillspector/suppression.py:182–190](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/suppression.py#L182-L190) (`source_content_key`) · [src/skillspector/suppression.py:218–260](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/suppression.py#L218-L260) (`finding_fingerprint`) · [required v2 reasons](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/suppression.py#L414-L445) · [src/skillspector/suppression.py:504–540](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/suppression.py#L504-L540) (`partition_findings`)

</details>

### 20. Handle direct prohibitions

Let selected prose rules recognize a direct prohibition like "do not run X" without suppressing a nearby real instruction.

**Decision:** Which prose rules may accept direct prohibitions, and how should ambiguity be handled?<br>
**Size:** Medium

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Opt-in allow_prohibition on selected prose rules; match-local prefix grammar ending exactly at match start; reject leading if/when/only and suffix unless/except/until/but/however/instead; preserve adjacent attacks and do-not-worry cases; never hidden comments/fences/nonprose; full schema/matching path parity and conservative ambiguity.

**Detailed proposal:**

Extend existing static matching with an explicitly opt-in prose rule property.

- Propose `allow_prohibition: true` for selected prompt-injection/exfiltration
  prose rules, preserving false as the default for custom and non-prose rules.
- Evaluate each match span against a tight negation grammar ending exactly at
  its start. Do not suppress another unsuppressed match on the same line.
- Reject suppression when conditions such as if/when/only govern the clause,
  or unless/except/until/but/however/instead introduce an exception or reversal.
- Keep referential actions and prefixes such as "Do not worry, ignore previous
  instructions" detectable. Never infer a blanket safety claim from negation.
- Apply only in ordinary Markdown prose, never hidden HTML comments, code
  fences or non-prose rules. Preserve existing tutorial/fence handling without
  adding a new exemption for SKILL.md.
- Carry the property through rule loading/overrides/listing and all supported
  matching paths, using existing Markdown context and original match locations.
- Retain findings whenever grammar or context is uncertain; favor a narrowly
  proven reduction of false positives over broad semantic interpretation.

Relevant source: `internal/audit/patterns.go`, `internal/audit/audit_scan_skill.go`, `internal/audit/audit_scan_content.go`.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines)
- [ ] Large (10+ files, 500+ lines)

Source estimates 70–120 production plus 90–150 test lines before all schema,
context and documentation consumers; the complete security-policy change is
borderline for the small-contribution guideline and needs scoped agreement.
Require adversarial table cases, multiple matches, leading conditions, suffix
exceptions, hidden comments/fences, custom rules and supported scan-path parity.
No implementation, test result or maintainer approval is claimed.

**Additional open questions**

- Which exact prose rules may opt in without suppressing concealment directives?
- Which negation grammar and clause boundaries are sufficiently narrow?
- How should ambiguous punctuation or multiline instructions retain findings?

**NVIDIA relationship:** Direct prohibition context and tests preserving later attacks; eligible rules and Markdown policy differ.

[src/skillspector/nodes/analyzers/prohibition_context.py:117–155](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/prohibition_context.py#L117-L155) (`is_directly_prohibited`) · [tests/nodes/analyzers/test_yara_prompt_prohibitions.py:34–80](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/analyzers/test_yara_prompt_prohibitions.py#L34-L80) (`test_direct_prompt_prohibitions_do_not_emit_builtin_yara_finding`)

</details>

### 21. Identify evaluation datasets

Recognize eval datasets so their example attacks don't flood reports, while still counting them as uninspected.

**Decision:** Should dataset exclusion be opt-in or default, and which formats and strict policy apply?<br>
**Size:** Medium<br>
**Earlier discussion:** #700, issue #696 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Exact proposed eval JSON/JSONL/YAML families, only static exemption with visible uninspected bytes/counts, explicit scan_eval_datasets reversal and integrity preservation; scripts/manifests keep scanning. Default exclusion vs opt-in and strict policy require approval; narrowed dataset.* formats are a deliberate safety choice to confirm.

**Detailed proposal:**

Recognize a narrow set of skill-relative dataset paths and propose excluding
only their static pattern analysis by default, with a visible disclosure.

- Start with `evals/evals.{json,jsonl,yaml,yml}` and
  `eval/dataset.{json,jsonl,yaml,yml}` as proposed exact path families.
  Do not exempt an entire directory or arbitrary `dataset.*` extensions.
- Keep manifests and scripts subject to their normal analysis even when they
  sit under an evaluation directory. A dataset label does not confer trust
  or permission to execute its contents.
- Add proposed `audit.scan_eval_datasets: true` policy to restore static
  inspection of these datasets, including the named JSONL format.
- Record each recognized dataset as data excluded from static analysis.
  Show counts and reasons in JSON and a concise human summary; do not count
  excluded bytes as inspected bytes or present exclusions as a clean verdict.
- Preserve integrity accounting and hash verification independently of the
  static exemption, using `internal/audit/analyzer_integrity.go`.
- Explain that dataset paths are author-controlled and that strict users can
  scan them. Retain other applicable analyzers rather than a blanket bypass.

Policy belongs with `internal/config/config.go`; collection and result changes
belong with `internal/audit/audit_scan_skill.go` and
`internal/audit/audit_result.go`. A general completeness contract could supply
shared dispositions if available, but is not a prerequisite for explicit output.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines), including policy and output consumers
- [ ] Large (10+ files, 500+ lines)

Cover matching/nonmatching paths, case handling, JSONL policy reversal,
identical hostile text in a dataset and a script, truthful byte accounting and
integrity changes. No implementation or change to default coverage is included.

**Additional open questions**

- Approve default static exclusion, or require users to opt in?
- Are these exact path families sufficient, and should matching be case-sensitive?
- Should strict profiles always scan recognized datasets despite the default?
- Which non-static analyzers should continue to inspect dataset content?

**NVIDIA relationship:** Documentation-to-source conflict at the pinned revision: [EVAL_DATASETS.md](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/docs/EVAL_DATASETS.md#L3-L21) says the named dataset paths are skipped by static patterns, but [static-pattern regression](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/analyzers/test_static_patterns.py#L881-L906) asserts findings for both eval paths and [ledger regression](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/test_security_remediation.py#L1773-L1782) asserts completed static inspection. Treat the documented exclusion as historical or uncertain, not current behavior; no tests were run. Skillshare default exclusion, wildcard scope, restoration policy and coverage reporting remain independent approval decisions.

</details>

## Rule quality

### 22. Measure detection accuracy

Add a labeled corpus of malicious and benign skills and track per-rule precision and recall in `make test`.

**Decision:** How should labels, per-rule metrics and metric-policy changes be reviewed?<br>
**Size:** Large<br>
**Earlier discussion:** #729, issue #725 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Complete representative labeled malicious/benign/mixed trees, required/forbidden rule labels, per-rule TP/FP/FN and zero-denominator policy, isolated normal make-test gate, versioned metric policy and reviewed labels. Later two-revision comparison/JSON+Markdown deltas/override approvals/private external corpus/PR-comment automation remain deliverables.

**Detailed proposal:**

Build the complete initial corpus gate, with labels describing actual expected
behavior rather than snapshots that automatically bless current scanner output.

- Add benign and malicious skill trees under the proposed corpus location in
  `internal/audit/testdata`, with expected and forbidden rule IDs per case.
  Include mixed-content skills, ordinary documentation, scripts and manifests,
  not only isolated strings that exactly reproduce existing unit tests.
- Define the unit of measurement as a case/rule label. Count true positives,
  false positives and false negatives per rule, aggregate them consistently,
  and report precision/recall with explicit handling of zero denominators.
- Require reviewed labels and explain ambiguous cases. Define whether unrelated
  unexpected findings fail a case; do not silently count unlabeled results as
  correct or treat every benign-looking example as universal ground truth.
- Add a deterministic test to the normal `make test` path in `Makefile`.
  Fail on forbidden findings, missing required findings and approved metric
  regressions, with named cases and per-rule counts in the failure output.
- Run with isolated configuration and stable built-in rules. Version the corpus,
  metric policy and baseline together; adding cases must not invisibly reset
  thresholds or conceal a newly introduced false positive.
- Represent the supported detection families with a documented corpus inventory.
  A small seed is useful during development but does not complete the proposed
  systematic gate. Published metrics describe this corpus, not real-world rates.

Keep explicitly later scope visible: compare two builds/revisions with per-rule
JSON/Markdown deltas, an approved regression-policy/override file, optional
external private corpus paths, and PR comment automation in
`.github/workflows/test.yaml`. Those are follow-on deliverables, not implied
by the initial test gate. External corpus contents must never enter public reports.

`internal/audit/rules.yaml`, `internal/audit/credentials.go` and
`internal/audit/patterns.go` define relevant built-in rule sources. No separate
feature is required. A benign-twin fixture project could share labels/harnesses;
that is optional reuse and a merge-coordination concern, not a prerequisite.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines), counting the complete corpus and policy

Estimate 250–500 harness/test lines initially plus representative labeled data;
later comparison/automation work needs its own estimate. This proposal includes
no corpus, implementation, accuracy measurement or claim of complete coverage.

**Additional open questions**

- Which detection families and minimum case diversity make the initial gate useful?
- Should the first policy allow any regression, or require reviewed baseline edits?
- How should multi-rule cases and unknown labels affect precision/recall?
- Which later comparison and private-corpus features are worth maintaining?

**NVIDIA relationship:** Direct corpus identity and bounded runner; Skillshare precision/recall gate differs.

[scripts/compare_scan_accuracy.py:304–339](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/scripts/compare_scan_accuracy.py#L304-L339) (`_resolve_case_path`, `_corpus_identity`) · [scripts/compare_scan_accuracy.py:348–390](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/scripts/compare_scan_accuracy.py#L348-L390) (`_terminate_process_tree`, `_run_bounded`)

</details>

### 23. Cover rules with benign twins

Require every built-in rule to have a positive fixture and a benign twin that must not match.

**Decision:** How should built-in rule coverage and explicit positive/negative twins be enforced?<br>
**Size:** Large<br>
**Earlier discussion:** #735, issue #730 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Full-skill expected.json positive/absent pairs for every reachable built-in rule, generated credential rules included, true matching file context, stale-ID/coverage gate and meaningful explicit twins; normal test gate with full inventory, no tiny seed completion claim. Optional corpus harness reuse only.

**Detailed proposal:**

Add skill-tree fixtures with expected and absent rule IDs, then enforce paired
coverage against the complete built-in rule inventory.

- Store small skill trees under a proposed fixtures subdirectory of
  `internal/audit/testdata`. Each case gets `expected.json` declaring required
  rule IDs and rule IDs that must be absent, with a short rationale for labels.
- Scan each tree through the real skill scanner with isolated configuration.
  Compare findings by stable rule ID rather than fragile message text, while
  keeping existing targeted unit tests for their useful local detail.
- Require every built-in rule to have a reachable positive fixture and a paired
  benign twin asserting that same rule is absent. List uncovered or stale IDs
  in failures and reject labels for rules that no longer exist.
- Enumerate built-ins through the rule source used by the scanner, including
  generated credential rules from `internal/audit/credentials.go`.
  `internal/audit/rules.yaml` alone is not the complete inventory;
  `internal/audit/patterns.go` appends generated credential rules.
- Include benign variants of credential access, legitimate Git/CLI use,
  documentation quoting risky commands and evaluation dataset content. Labels
  must follow the intended policy; quoted commands are not automatically safe.
- Cover each rule's applicable file context. Avoid a passing negative caused
  only by an unrelated file filter, disabled rule or unvisited fixture path.
  Keep pair relationships explicit so one generic clean file cannot stand in
  for meaningful benign coverage of the entire rule set.
- Keep the all-rule check in the normal test gate. Deliver the complete fixture
  inventory with it; a handful of seeded pairs is an intermediate milestone.

The harness belongs with existing scanner tests and `internal/audit/testdata`.
No new runtime analyzer or default audit policy is proposed. Dataset-exclusion
or MCP analyzer work may later add fixtures for its own behavior; neither is
needed to establish coverage for today's built-in rules.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines), including the complete paired fixture data

Estimate 150–250 harness lines plus 700–1,500 lines of fixture data, potentially
more after inventorying contexts. No prerequisite feature exists; coordination
with corpus work is optional reuse. This proposal includes no tests or coverage
claim and does not treat a small initial seed as completion.

**Additional open questions**

- Is one meaningful pair per rule sufficient, or do some contexts need more?
- Should unexpected unrelated findings fail fixtures, or only declared assertions?
- How should policy changes update labels without rubber-stamping regressions?
- Should non-regex analyzer findings get an analogous coverage inventory later?

**NVIDIA relationship:** Direct benign/malicious test inspiration; automatic rule coverage is new.

[tests/nodes/test_nested_artifacts.py:668–710](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/test_nested_artifacts.py#L668-L710) (`test_hidden_disguised_document_inventories_nested_executable_locally`) · [tests/nodes/analyzers/test_yara_prompt_prohibitions.py:34–80](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/analyzers/test_yara_prompt_prohibitions.py#L34-L80) (`test_direct_prompt_prohibitions_do_not_emit_builtin_yara_finding`)

</details>

## Reports and day-to-day use

### 24. Explain findings and remediation

Attach a short explanation and fix to each rule, shown in text, Markdown, SARIF help and the dashboard.

**Decision:** How should overrides distinguish missing from empty values, and should guidance be localized?<br>
**Size:** Large<br>
**Earlier discussion:** #716, issue #710 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Explanation/remediation metadata through custom rules and override merging; populate ALL built-ins incl generated credentials, listing/Markdown/SARIF help/dashboard with escaping, nonempty-explanation inventory gate. Missing-vs-empty override and localized UI policy remain questions.

**Detailed proposal:**

Extend existing rule metadata and populate the full built-in inventory.

- Add optional explanation/remediation fields to YAML rules, validate them and
  preserve them through global/project merging and custom rule loading.
- Populate useful guidance for every built-in rule, including generated
  credential rules; a schema-only seed is not completion.
- Expose help in `audit rules`, Markdown reports and the dashboard’s expandable
  audit results, with stable rule identity and escaping appropriate to each format.
- Emit SARIF rule help with the available explanation and remediation, retaining
  current result locations, severity and finding messages.
- Let custom rules provide their own text without executing links or templates.
  Define absent/empty override semantics so guidance is not silently lost.
- Keep guidance evidence-based: explain what was detected, common legitimate
  cases and a concrete review/remediation action, without promising safety.
- Add an inventory gate requiring nonempty explanation for every built-in rule,
  plus formatter/schema/override regression tests.

Relevant source: `internal/audit/patterns.go`, `internal/audit/rules_list.go`, `internal/audit/sarif.go`, `ui/src/pages/AuditPage.tsx`.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate 250–450 plumbing lines plus 400–800 guidance-content lines and
150–250 test lines; all consumers and populated data are part of one feature.
Test generated-rule coverage, custom help, override behavior, escaping and
consistent SARIF/Markdown/listing/dashboard presentation.
No implementation, test result or maintainer approval is claimed.

**Additional open questions**

- Which guidance structure should custom rules and overrides share?
- Should remediation be mandatory for built-ins as well as explanation?
- How should localized UI display English rule help initially?

**NVIDIA relationship:** Direct explanation and remediation lookup; custom YAML, SARIF and UI are adaptations.

[src/skillspector/nodes/analyzers/pattern_defaults.py:474–488](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/pattern_defaults.py#L474-L488) (`get_explanation`, `get_remediation`)

</details>

### 25. Filter displayed severity

Add a display-only minimum severity to the terminal, Markdown and UI output, without changing scores or exit codes.

**Decision:** Should JSON ignore the display floor or add hidden-count metadata?<br>
**Size:** Medium

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Inclusive display-only severity floor in terminal/Markdown/UI with full findings/scores/gates/errors/completeness preserved and visible/hidden counts. Reuse summary.maxSeverity. Decision needed: byte-identical JSON vs additive hiddenFindingsCount; require explicit maintainer choice rather than claiming both.

**Detailed proposal:**

Agree the reporting contract before extending the existing CLI/UI controls.

- Propose `audit --min-severity <level>` as display-only, with validated levels
  and a consistent inclusive floor across terminal, Markdown and dashboard.
- Preserve the complete findings, risk score, threshold evaluation and exit
  status. Filtering must not affect install/update policy or accepted findings.
- Show visible and hidden finding counts in human reports, including the case
  where all findings are hidden. Keep errors/incomplete status visible.
- Preserve full machine-readable findings; define whether JSON ignores the
  display flag or adds display metadata without removing underlying data.
- Existing summary.maxSeverity should be reused. Decide whether a new top-level
  alias is justified rather than introducing a second inconsistent value.
- The original request asks both identical JSON with/without the flag and new
  hiddenFindingsCount metadata. Those cannot both mean byte-identical output
  when the floor changes; prefer unchanged findings/gating with explicitly
  documented additive display metadata, subject to maintainer agreement.
- Extend current UI filtering rather than a parallel control, preserving
  existing severity selection and low/info-collapse behavior where useful.

Relevant source: `cmd/skillshare/audit.go`, `internal/audit/markdown.go`, `ui/src/pages/AuditPage.tsx`.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines)
- [ ] Large (10+ files, 500+ lines)

Estimate 100–180 production plus 80–140 test lines across CLI, formatters and
UI; the complete multi-consumer contract is borderline for a small change.
Test every floor, all-hidden/no-findings/error states, count accuracy, unchanged
score/exit/finding data, Markdown escaping and existing UI filter interaction.
No implementation, test result or maintainer approval is claimed.

**Additional open questions**

- Should JSON ignore the floor or include additive display metadata?
- Is existing summary.maxSeverity sufficient without a top-level duplicate?
- How should the floor combine with current exact-severity UI selection?

**NVIDIA relationship:** Indirect maximum-severity computation; display-only filter and hidden count are new.

[src/skillspector/nodes/report.py:533–560](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/report.py#L533-L560) (`_max_issue_severity`)

</details>

### 26. Discover and rank audits

Find and audit every skill under an arbitrary directory, with jobs, per-skill timeouts and ranked output.

**Decision:** Which discovery boundaries, ranking and timeout controls should be exposed?<br>
**Size:** Large<br>
**Earlier discussion:** #718, issue #704 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Bounded deterministic SKILL.md discovery, --jobs/--skill-timeout/--top, nesting/hidden/symlink policy, cancellation inside actual scanner, per-skill failed/timed-out/never-started statuses, retained findings and stable severity/count/path ordering. top-N only limits display; all machine findings/exit decisions retained.

**Detailed proposal:**

Extend the existing audit command and parallel runner rather than a new CLI.

- Propose `--discover <dir>`, `--jobs N`, `--skill-timeout 30s` and `--top N`.
  Validate positive limits and bound discovery depth and entry count.
- Discover SKILL.md parent directories deterministically, skipping `.git` and
  `node_modules`; define nested-skill, hidden-path and symlink policies.
- Reuse parallel scheduling with actual cooperative scan cancellation.
  A timeout wrapper that abandons a running scan is insufficient.
- Record malformed, unreadable, timed-out and never-started skills explicitly;
  continue independent targets and retain findings already produced.
- Sort by maximum severity, then finding count with stable path tie-breaking.
  Top-N limits human display, not underlying findings, failures or exit gating.
- Emit one structured report with full per-skill statuses and aggregate counts,
  while keeping single-path and managed-source modes compatible.
- Use a shared completeness contract for partial work; do not represent a
  timeout or failed scan as a zero-finding clean skill.

Add cooperative cancellation to scanning and ParallelScan as part of this
feature. Reuse an agreed resource/completeness contract if one is available;
discovery itself has no prerequisite feature.

Relevant source: `cmd/skillshare/audit.go`, `internal/audit/parallel.go`.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate 300–550 production and 250–450 test lines across discovery, CLI,
scheduling, reporting and documentation.
Cover nested trees, ignored paths, malformed skills, cancellation, failures,
ordering, top-N visibility and unchanged exit decisions with synthetic fixtures.
No implementation, test result or maintainer approval is claimed.

**Additional open questions**

- Approve discovery/symlink defaults and the aggregation/exit contract?
- Should a nested skill be audited independently from its parent?
- What deadline overshoot is acceptable for non-interruptible operations?

**NVIDIA relationship:** Direct discovery, bounded worker and sorted reporting; Skillshare jobs and ranking differ.

[contrib/batch_scan/discovery.py:27–39](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/contrib/batch_scan/discovery.py#L27-L39) (`discover_skills`) · [contrib/batch_scan/batch_scan.py:208–250](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/contrib/batch_scan/batch_scan.py#L208-L250) (`_scan_skill_bounded`) · [contrib/batch_scan/reports.py:36–44](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/contrib/batch_scan/reports.py#L36-L44) (`sorted_results`)

</details>

## Integrations and opt-in online checks

### 27. Expose catalog-scoped MCP audit tools

Add `audit_skill` and `check_updates` tools to the existing MCP server, limited to installed skills.

**Decision:** Which catalog-scoped tools and network permissions should be exposed?<br>
**Size:** Medium<br>
**Earlier discussion:** #712, issue #708 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Extend existing skillserve MCP list/read server with catalog-scoped audit_skill/check_updates; resolved policy/findings/partial states, no arbitrary path/URL, outbound checks only selected installed sources and HTTP opt-in, existing authenticated HTTP retained, native Skills capability must not hide tools. No duplicate command/server/SDK migration; safe_to_install wording is limited policy verdict.

The earlier proposal described `--transport stdio|http` and `--read-only` controls. Evaluate those needs against the existing server before adding any new command or transport.

**Detailed proposal:**

Extend the existing server with read-only audit and update-check tools while
keeping its current transport, authentication and optional editing behavior.

- Add proposed `audit_skill(target, threshold, profile)` and `check_updates`
  tools. Resolve targets from the server-selected catalog or installed skills,
  respecting the selected global/project context and source-follow policy.
- Reuse `internal/audit` and `internal/check/resolve.go` for domain behavior.
  Return findings, resolved threshold/profile, blocking state and update statuses
  as structured results. Distinguish errors, unknown state and partial inspection.
- If exposing `safe_to_install`, define it only as a policy verdict under the
  reported scan scope. Prefer `passes_audit_policy`; no finding is not a
  guarantee that arbitrary skill behavior is safe.
- Reject arbitrary local paths and caller-supplied remote URLs. Do not turn an
  authenticated HTTP tool call into access to the server's entire filesystem
  or a general network-fetch interface.
- Keep audit local to selected content. Update checking may contact only the
  selected installed sources, subject to explicit server policy; propose it
  disabled for HTTP unless the operator enables outbound checks.
- Preserve authenticated non-loopback serving and existing HTTP protections.
  Apply new restrictions to these tools without making all serving loopback-only.
- Make the new tools available to clients using the native Skills extension;
  its current hiding of duplicate list/read tools should not hide audit/check.

Document tool schemas, policy resolution, transport-specific target restrictions
and existing server registration. No new `skillshare mcp` command, SDK migration
or replacement list/read interface is proposed.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines), potentially larger with documentation
- [ ] Large (10+ files, 500+ lines)

Estimated implementation: 230–400 production lines plus fixture-backed tool,
policy, transport and client-capability tests. No prerequisite feature or new
server is required. This proposal includes no implementation or network checks.

**Additional open questions**

- Approve the catalog-only target boundary and outbound-check defaults?
- Should callers be allowed to weaken server audit policy, or only strengthen it?
- Which structured field best expresses a limited audit verdict?
- Should audit/check tools be opt-in independently of existing edit tools?

**NVIDIA relationship:** Direct scan_skill tool; Skillshare list/check and target policy differ.

[src/skillspector/mcp_server.py:247–295](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/mcp_server.py#L247-L295) (`build_server`, `scan_skill`)

</details>

### 28. Inventory and traverse external references

List every external URL or repo a skill tells the agent to fetch, with optional bounded transitive auditing later.

**Decision:** Should transitive traversal be enabled separately from local inventory, with which boundary policy?<br>
**Size:** Large<br>
**Earlier discussion:** #738, issue #715 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Local canonical external-reference inventory with source locations/spelling/revision distinctions/self-ref exclusion; later explicit transitive stage with canonical prefix/redirect/destination policy, depth/count/time/byte admission, disposable cleanup/cycles/visited edges/truncated frontier. Inventory independent; traversal genuinely needs bounded ingestion and completeness/cancellation.

The earlier proposal included `--external-refs`, `--transitive`, `--depth`, `--allow-prefix` and `--deny-prefix`; these remain candidate controls subject to the stated traversal and policy decisions.

**Detailed proposal:**

Keep a useful local inventory and explicitly later transitive traversal visible.

- Propose `audit <skill> --external-refs` to collect references with source
  locations, canonical identities and original spelling preserved as evidence.
- Normalize supported GitHub blob/tree/raw forms, deduplicate locations and
  exclude verifiable self-references without collapsing distinct revisions.
- Emit an `external_references` array in JSON and a human inventory. Listing a
  reference does not fetch it, trust it or imply it is installable.
- Propose opt-in `--transitive`, depth/count/time caps and allow/deny prefixes
  for a later traversal stage. Define exact canonical-prefix comparison and
  redirect/destination policy before any request.
- Fetch to disposable locations through bounded ingestion, audit each admitted
  source and clean up after failure or cancellation; never execute fetched code.
- Report visited nodes, failures and a truncated frontier with reasons, retaining
  source edges and deterministic ordering. Cycles and duplicate refs share work.
- Inventory has no prerequisite. Transitive fetches genuinely require admission,
  byte/path/time limits equivalent to the
  [bounded remote-install proposal](https://github.com/runkids/skillshare/pull/684);
  cancellation and completeness contracts must be agreed, not assumed.

Relevant source: `internal/audit/markdown_links.go`, `internal/install/github_download.go`, `internal/install/gitea_download.go`.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Inventory estimate 180–300 production and 150–250 test lines; complete
traversal 650–1,100 production and 450–750 test lines.
Cover canonical forms/revisions, self-refs, duplicates, cycles, redirects,
prefix boundaries, time/count exhaustion and cleanup with fixture servers.
No implementation, test result or maintainer approval is claimed.

**Additional open questions**

- Which reference forms/revision semantics should canonicalization support?
- What allow/deny and redirect policy should transitive requests obey?
- Which truncated-frontier states must affect exit or installation policy?

**NVIDIA relationship:** Direct bounded references and frontier; Skillshare allow/deny and output differ.

[src/skillspector/transitive.py:58–72](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/transitive.py#L58-L72) (`MAX_EXTERNAL_REFERENCE_SOURCES`, `MAX_EXTERNAL_REFERENCE_SOURCE_BYTES`) · [src/skillspector/transitive.py:183–230](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/transitive.py#L183-L230) (`BoundedTransitiveFrontier`, `__post_init__`) · [src/skillspector/transitive.py:377–400](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/transitive.py#L377-L400) (`extract_external_refs_with_metadata`)

</details>

### 29. Offer opt-in dependency vulnerability checks

Opt-in OSV lookups for pinned dependencies, with caching and an explicit incomplete state when lookups fail.

**Decision:** Which exact pins, destination policy, cache TTL and incomplete states should be supported?<br>
**Size:** Large<br>
**Earlier discussion:** #739, issue #723 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Off-by-default OSV batch lookups for exact requirements/package+lock/go.mod/Cargo.lock pins, bounded destination/redirect client, severity unknowns, validated TTL cache, explicit failed/skipped incomplete state. Static fallback only with pinned provenance/narrower coverage. Optional shared manifest parsing, no implicit online ordinary audit.

**Detailed proposal:**

Add an off-by-default dependency analyzer with explicit network policy.

- Propose `--online-deps` or equivalent resolved policy, never an implicit
  network call during ordinary offline audits.
- Parse exact pins from requirements.txt, package/lock JSON, go.mod and
  Cargo.lock; distinguish ranges, missing pins and unsupported constructs.
- Query OSV's batch API through a destination-restricted, timeout/byte/count
  bounded client with redirect policy and no caller-supplied arbitrary endpoint.
- Map package/ecosystem identity and reported severities consistently; take the
  worst applicable severity without inventing scores for unknown advisories.
- Cache results on disk with validated schema, TTL and package/version/query
  identity; distinguish a fresh empty result from no result or a stale entry.
- If an offline fallback catalog is supported, pin its provenance/version and
  disclose its narrower coverage. Do not call stale/static data a live lookup.
- Emit explicit skipped/failed lookup status and an informational diagnostic,
  feeding incomplete-analysis state rather than silently reporting clean.
- Define a completeness contract jointly with scanning, informed by the
  [scan-completeness proposal](https://github.com/runkids/skillshare/pull/690).
  Dependency parser sharing with offline supply-chain analysis is optional reuse,
  not a hard feature dependency.

Relevant source: `internal/audit/analyzers.go`, `internal/audit/policy.go`, `internal/config/config.go`.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate 600–1,000 production and 400–700 test lines, including parsers,
client, cache, reporting and policy.
Use httptest and synthetic manifests only: batching, timeouts, malformed output,
cache expiry/corruption, unknown scores, offline behavior and incomplete gates.
No implementation, test result or maintainer approval is claimed.

**Additional open questions**

- Approve opt-in defaults, destination policy and cache retention?
- Which exact pin formats and severity sources should be supported first?
- Should stale/static fallback be allowed, and how should its coverage be shown?

**NVIDIA relationship:** Direct batch query and in-process cache; persistent TTL and incomplete handling differ.

[src/skillspector/nodes/analyzers/osv_client.py:213–235](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/osv_client.py#L213-L235) (`_cache_key`, `_get_cached`) · [src/skillspector/nodes/analyzers/osv_client.py:564–600](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/osv_client.py#L564-L600) (`query_batch`)

</details>

### 30. Offer an opt-in semantic audit

Opt-in semantic review using an installed agent CLI, isolated and on top of static findings rather than replacing them.

**Decision:** Should the semantic stage be offered, and which provider, isolation and required-pass controls are acceptable?<br>
**Size:** Large<br>
**Earlier discussion:** #703, issue #698 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** One opt-in semantic subsystem with all six nested requirements: isolation, usage/provenance, required-pass accounting, costs/cache, static triage and offline mocks/replay. Exact provider/flag/schema controls need explicit disposition; preserve static findings and no silent HIGH/CRITICAL dismissal. Details below.

**Detailed proposal:**

**Entry points and findings**

Keep semantic review off by default. Propose `skillshare audit <path> --llm [--llm-provider claude]` and an explicitly requested `install --llm` path. Installation already runs static audit by default; the new flag would opt into semantic review within that gate. Never run semantic review automatically during sync. Define an `internal/audit/llm` provider contract along the lines of `Name`, `Available` and `Complete`; availability must include the verified capability boundary, not just a binary or login probe. A missing, unsupported, unverified or unauthenticated provider is reported as unavailable with a clear skip reason; never silently switch providers or fall back to broader credentials.

Treat skill files and model output as untrusted. Split input into bounded, overlapping chunks with `L<n>:` line prefixes. Request structured JSON with at least `rule_id`, `severity`, `start_line`, `confidence` and `explanation`, validate IDs, severity, numeric values and source locations against the submitted file and range, and reject malformed or ambiguous results. Emit `LLM-` rule IDs with `analyzer="llm"` in the existing finding shape and JSON/SARIF. Dedupe overlap without losing evidence or source locations. Preserve static findings and normal behavior when semantic review is not requested.

**Capability and authentication boundary**

Define a per-CLI specification table with exact argv, parser, authentication probe and a verified-policy flag. Use `exec.CommandContext` with no shell, validated model labels, prompts on stdin, an allowlisted child environment that excludes API keys and unrelated secrets, a replacement system prompt, a fresh temporary working directory removed after use, bounded stdout/stderr, timeout and process termination on overflow or cancellation. Verify that tools, MCP, hooks, settings discovery, slash commands and persistent sessions cannot run or load. Preserve only authentication access the chosen CLI actually requires and document that boundary, including the provider's destination and any unavoidable local credential access. A CLI that cannot satisfy and demonstrate the boundary remains unavailable. Do not mistake a read-only mode for proof of denied tool execution.

**Usage and provenance**

Propose an optional `audit.llm` configuration for provider, model, per-slot model overrides, temperature and timeout, with explicit environment overrides. Specify config/environment precedence, model source, and requested versus resolved provider/model for every slot. Record requested versus honored controls, including temperature, seed and effort; mark unsupported or unavailable controls explicitly. Report actual provider-supplied token counters when present, with missing counters unknown rather than zero. Add a versioned `llm` object to JSON and SARIF `run.properties`, and concise text and operation-log summaries. Never include prompts, environment values or credentials in reports or logs, or present inferred prices as measured usage. Exact output schema and control support need maintainer agreement.

**Completeness and required mode**

Represent `llm_status` as `not_requested`, `complete`, `incomplete` or `unavailable`, with limitations and per-file skip counts. Optional review still shows a warning and every unavailable, failed, skipped or budget-limited batch; absence of semantic findings must not imply the pass completed. Propose `--llm-required` to make a requested pass fail its gate unless all required work is complete, including missing provider, invalid output, skipped files or failed batches. Decide exit-code and install-policy interaction explicitly, including whether `--llm-required` implies `--llm`; no required mode silently succeeds on incomplete work.

**Costs, budgets and cache**

Propose `--llm-max-concurrency` with initial default **2**, `--llm-max-calls`, `--llm-max-file-bytes`, and `--llm-timeout`, with corresponding configuration and bounded chunk/output sizes. A first-use billing warning precedes calls; `audit --llm --plan` estimates work and exits without calling a provider. Limit exhaustion marks the pass incomplete. Any estimate must be labelled as such and must not be presented as actual usage or a guaranteed price.

A verdict cache can avoid repeating unchanged work. Key it to the skill/content hash, prompt version, model and provider, and engine/analysis and relevant configuration identity so a changed input or policy cannot reuse stale results. Provide `--llm-no-cache` and visible cache-hit provenance. Agree retention, permissions, invalidation, storage of sensitive content and any monetary ceiling before implementation. Caches cannot hide missing batches or provider failure as fresh complete results.

**Advisory triage of static findings**

An optional second pass receives redacted static findings grouped by file and limited context. Associate each response by stable finding ID and validate a schema containing `finding_id`, a confirm/dismiss/downgrade verdict and reason. Store the model verdict and reason separately, including in SARIF properties. Default to advisory: retain the original finding, severity, evidence, scoring and exit behavior; never silently remove HIGH or CRITICAL findings. A model disagreement is not proof that a rule is wrong.

The original idea also proposed `--llm-dismiss` to exclude dismissed non-CRITICAL findings from gating. That wording includes HIGH while the original safety requirement says not to silently drop HIGH/CRITICAL. Keep this as an explicit unresolved maintainer decision: whether dismissal can ever affect gating, whether HIGH requires explicit consent and visible disclosure or must remain gate-effective, and how originals, audit logs and SARIF remain available. Do not assume that a flag or model verdict resolves this conflict. Without such an approved opt-in, model triage has no gate effect.

Relevant source: `internal/audit/analyzer_registry.go`, `internal/audit/audit.go`, `internal/audit/sarif.go`.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate 1,800–3,500 production lines plus 1,200–2,500 test lines; provider
isolation feasibility must be settled before provider enablement.
Validate isolation, deadlines, failure accounting, provenance, cache invalidation
and unchanged static findings through offline mocks; no live model experiment here.
No implementation, test result or maintainer approval is claimed.

**Additional open questions**

- Which providers can satisfy a verifiable capability and authentication boundary?
- What content may leave the machine, and what user approval/disclosure is required?
- Which required-pass and advisory-triage semantics should be supported first?

**NVIDIA relationship:** Direct batching and semantic node; opt-in and Skillshare provider/SARIF contract differ; see six requirements below.

[src/skillspector/llm_analyzer_base.py:874–918](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/llm_analyzer_base.py#L874-L918) · [src/skillspector/nodes/analyzers/semantic_security_discovery.py:108–150](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/semantic_security_discovery.py#L108-L150) (`node`) · [tests/nodes/analyzers/test_semantic_security_discovery.py:291–355](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/analyzers/test_semantic_security_discovery.py#L291-L355) (`TestSemanticSecurityDiscoveryBatching`, `TestUseLlmGuard`)
**Related semantic requirements:**

- **Usage/provenance:** Partial: provider-reported usage and controls; Skillshare JSON/SARIF is new. [src/skillspector/llm_analyzer_base.py:1126–1143](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/llm_analyzer_base.py#L1126-L1143) (`inference_usage`)

- **Required/incomplete:** Partial: batch failure ledger and general incomplete gate; --llm-required semantics are separate. [src/skillspector/inspection_ledger.py:296–303](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/inspection_ledger.py#L296-L303) (`outcome_for_llm_batch_failure`) · [src/skillspector/cli.py:950–960](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/cli.py#L950-L960)

- **Cost guardrails/cache:** Partial: token and process caps do not prove monetary cap or verdict cache. [src/skillspector/llm_analyzer_base.py:874–918](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/llm_analyzer_base.py#L874-L918) · [src/skillspector/providers/_agent_cli.py:73–76](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/providers/_agent_cli.py#L73-L76) (`MAX_INPUT_BYTES`, `MAX_OUTPUT_BYTES`)

- **Mock/replay:** Related tests cover batching and guard; replay and SARIF assertions are new. [tests/nodes/analyzers/test_semantic_security_discovery.py:291–355](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/analyzers/test_semantic_security_discovery.py#L291-L355) (`TestSemanticSecurityDiscoveryBatching`, `TestUseLlmGuard`)

- **Static finding triage:** Indirect meta-pass test; confirm/dismiss/downgrade policy is separate. [tests/nodes/test_meta_analyzer_fallback.py:184–229](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/test_meta_analyzer_fallback.py#L184-L229) (`TestCombinedFallback`, `TestLLMFailurePassthrough`)

- **Subprocess isolation:** Partial: environment scrub and stdin, with Codex disabled pending no-tools policy; not proof of a universal OS sandbox. [src/skillspector/providers/_agent_cli.py:16–30](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/providers/_agent_cli.py#L16-L30) · [src/skillspector/providers/_agent_cli.py:130–144](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/providers/_agent_cli.py#L130-L144) (`_scrub_env`) · [src/skillspector/providers/_agent_cli.py:319–327](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/providers/_agent_cli.py#L319-L327) (`_build_codex_argv`)

</details>

## Update and mutation gates

### 31. Confirm expanded update capabilities

On update, diff the old and new capability surface and ask before accepting an expansion.

**Decision:** Which capability changes require renewed consent, including unattended and batch updates?<br>
**Size:** Large<br>
**Earlier discussion:** #697, issue #693 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Independent old-vs-candidate capability acceptance: tools/description/triggers/scripts+risk/hosts/MCP commands, additions/removals/changes and surface_delta, revision-bound central summary and unknown baseline handling, interactive/unattended/batch+dashboard consent, permission-prestaging heuristic and rejection preservation. Resource limits are not consent; --yes is distinct from --force/--skip-audit.

**Detailed proposal:**

Compare the installed revision with the candidate before publication, across
tracked and individual skill updates, global/project mode and dashboard updates.
Treat this as a review of declared and observed capabilities, not proof of all
runtime behavior.

- Extract allowed-tools, description and trigger text, script inventory and
  maximum risk tier, external hosts and bundled MCP server commands.
- Show additions, removals and changes, including new scripts and increased
  script risk. Add a proposed `surface_delta` field to update JSON output.
  Keep unchanged surfaces silent in normal human output.
- Require explicit confirmation for added tools, scripts, hosts, server commands
  or increased risk. Show description/trigger changes for review without
  treating every wording edit as a privilege increase.
- Flag manifest language announcing later permission requests as a heuristic
  warning, with the relevant text. It is not evidence permission was granted.
- Reject expansion in unattended mode unless an explicit approval option was
  supplied. Propose `--yes` for this gate; do not silently equate it with
  existing audit overrides, `--force` or `--skip-audit`.
- Persist a versioned surface summary tied to the accepted revision in central
  metadata. For older installs, derive the baseline from installed content;
  mark extraction failures as unknown and require review rather than no change.
- On rejection, preserve installed content and its accepted summary. Apply the
  same decision through CLI and dashboard, including batch updates.

Relevant paths are `internal/install/metadata.go`,
`internal/install/install_update.go`, `internal/update/tracked.go`,
`cmd/skillshare/update.go`, `cmd/skillshare/update_handlers.go` and
`internal/server/handler_update.go`. Shared extraction from another analyzer
would be optional reuse; no separate analyzer is a prerequisite.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimated implementation: 450–800 production lines plus regression fixtures,
metadata compatibility, CLI/dashboard output and documentation. Cover expansion,
unchanged content, missing baselines, extraction failures and declined batches.
This proposal includes no implementation.

**Additional open questions**

- Is confirmation for every new host/script appropriate, or should policy allow
  a narrower set of high-risk expansions? Which changes count as equivalent?
- Should description/trigger changes ever block automatically?
- Should the expansion approval be one batch decision or one decision per skill?
- How should incomplete extraction be presented without implying full coverage?

**NVIDIA relationship:** Direct inspiration: `_check_manifest_changes` compares supplied current and previous manifests, emits RP1 for added permissions and RP2 for added or removed triggers; `node` invokes it only when both manifests are present. Its tests cover added permissions and trigger additions/removals. `_check_rp2` separately flags manifest wording suggesting future permissions as a heuristic. Skillshare's broader tools/scripts/risk/hosts/MCP surface, revision-bound stored baseline, update-time consent and unattended/batch/dashboard policy remain distinct design decisions. These source spans do not establish an update approval workflow.

[Manifest comparison](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/mcp_rug_pull.py#L554-L613) (`_check_manifest_changes`) · [Conditional invocation](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/mcp_rug_pull.py#L713-L741) (`node`) · [Permission expansion test](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/analyzers/test_mcp_rug_pull.py#L130-L151) · [Trigger addition/removal tests](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/analyzers/test_mcp_rug_pull.py#L168-L202) · [Permission pre-staging heuristic](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/mcp_rug_pull.py#L449-L482) (`_check_rp2`)

</details>

### 32. Audit managed hooks and rules before mutation

Audit managed hooks and rules when they are saved or synced, before they reach agent configs.

**Decision:** Which managed mutation points need audit gates and explicit overrides?<br>
**Size:** Large<br>
**Earlier discussion:** #694, issue #692 (closed, replaced by this list)

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Independent operation-time audit gates for hook preview/save/source-only save and sync, referenced code, explicit rule-extra classification and transformed output; doctor/CLI/API/Hooks+Extras UI visibility; failed publication containment and explicit overrides separate from conflict --force. Mutable symlink sources cannot receive a continuous safety guarantee.

Test a dangerous `curl | sh` command and a normal `prettier --write` formatter as distinct cases. Whether an override resembles `--skip-audit` remains an explicit policy choice rather than permission to bypass the gate.

**Detailed proposal:**

Apply the active audit policy to managed hook commands, referenced code and
rule bodies before saving or publishing them to targets, in both modes.

- Define rule extras explicitly in configuration. Start with opted-in rule
  sources rather than treating every arbitrary extra as an instruction file.
- Audit previews and saves through the shared hook service, including source-only
  saves. Propose blocking saves at the active threshold even without sync;
  native output conflicts remain a separate decision from content findings.
- Re-audit current source and resolved references before sync. For transformed
  rule extras, inspect the content that will actually be published as well.
- Provide an explicit audit override or skip option with visible diagnostics.
  Do not silently give the existing extras conflict override audit semantics.
- Surface findings and skipped/failed inspections through doctor, CLI, API and
  the Hooks and Extras dashboard views, with file and hook/rule identity.
- Keep failed gated operations from publishing rejected content. Document that
  symlinked sources can change after sync, so this is an operation-time gate.

Relevant boundaries are `internal/hooks/apply.go`, `internal/hooks/plan.go`,
`internal/sync/extras.go` and `internal/server/handler_extras_edit.go`.
Reuse audit policy and matching; this needs no bundled-settings analyzer.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate includes both mutation systems, policy plumbing, reports, docs and
fixture coverage. Acceptance should include a rejected fetch-and-execute hook,
a normal formatter hook, rule-body findings, source-only saves, sync-time
reference changes and explicit overrides. This proposal includes no implementation.

**Additional open questions**

- Is explicit rule-source classification the right default for existing extras?
- Should rejected source-only drafts be saveable with an override, or only previewed?
- Which skip/override contract should CLI and API expose, and how is its use shown?
- Should rules supplied through directory symlinks require a copy mode for any
  stronger guarantee than inspection at save/sync time?

**NVIDIA relationship:** Indirect: bundled hook inspection; managed save/sync enforcement and UI are new.

[src/skillspector/nodes/analyzers/bundled_execution_surface.py:669–741](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/bundled_execution_surface.py#L669-L741) (`_hook_declaration`, `_hook_group_declarations`) · [tests/nodes/analyzers/test_bundled_execution_surface.py:176–200](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/analyzers/test_bundled_execution_surface.py#L176-L200) (`test_bh1_handler_and_reach_table`)

</details>

## Releases

### 33. Describe release trust and provenance

Sign release artifacts, publish SBOMs and provenance, and pin workflow actions.

**Decision:** Which release identity, signing, SBOM and provenance guarantees should maintainers adopt?<br>
**Size:** Large

<details>
<summary>Full proposal</summary>

**Proposed scope and acceptance:** Eligible separate design-only maintainer release policy: finalized checksums cosign keyless with constrained identity/issuer/ref, Syft SBOM formats+UI/container scope, archive/GHCR provenance digests, draft/manual publish separation, SHA actions+Dependabot, informational Scorecard, verification/recovery docs; optional upgrade signature checking later. No release/signing/OIDC/workflow actions authorized here.

**Detailed proposal:**

Ask maintainers to approve one release-trust policy before changing workflows,
OIDC permissions, signing identity or publication behavior.

- Propose keyless cosign signing of the final `checksums.txt`, after all assets
  and the formula are finalized. Bind verification to the approved repository,
  workflow identity, tag/ref and OIDC issuer; document those trust constraints.
- Generate explicit Syft SBOMs with GoReleaser for release artifacts. Define
  formats, asset coverage and retention, including how bundled UI content and
  container dependencies are represented.
- Add build-provenance attestations for release archives and GHCR images,
  bound to their immutable digests and the approved source revision. Preserve
  the current draft-build and manual-publish separation.
- Pin third-party actions to reviewed commit SHAs and configure Dependabot
  updates. Some actions are already SHA-pinned; this completes a consistent
  policy rather than replacing an entirely unpinned workflow set.
- Add OpenSSF Scorecard reporting, initially informational. Decide which later
  thresholds should gate releases instead of treating a score as proof of safety.
- Document checksum, cosign blob and provenance verification with constrained
  identities, and failure/recovery behavior for rebuilt draft assets.
- Keep signature verification in `skillshare upgrade` optional later work.
  Specify trust roots, compatibility and failure policy before enabling it.

Relevant paths are `.github/workflows/release.yaml`,
`.github/workflows/publish-release.yml`, `.github/workflows/docker-publish.yml`,
`scripts/release/release.py` and `.goreleaser.yaml`. Signing tools and services
would be new release dependencies. No audit feature is a prerequisite.

**Acceptance and additional decisions:**

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines), including all workflow/docs consumers

Core workflow work is estimated at 220–450 production lines plus validation;
complete documentation and later upgrade verification can exceed that estimate.
This is a maintainer policy proposal only. It performs no signing, release,
permission change, publishing or workflow execution.

**Additional open questions**

- Which exact workflow identity, issuer and artifact set should users trust?
- Must signing, SBOM and attestation failures block publication from day one?
- How should attestations survive the final checksum/formula preparation step?
- Which SBOM formats and Scorecard thresholds should be supported?
- Should upgrade verification remain opt-in, and how are older releases handled?

**NVIDIA relationship:** Indirect pinned actions; no cosign, SBOM or attestations established by these spans.

[.github/workflows/release.yml:30–40](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/.github/workflows/release.yml#L30-L40) · [.github/workflows/scorecard.yml:37–42](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/.github/workflows/scorecard.yml#L37-L42)

</details>

## Alternatives Considered

Keep the separate proposal PRs open. That keeps each discussion in its own thread, but leaves a long queue of near-identical review requests. Reopening any one of them is easy if a thread works better for a particular item.

## Scope

- [x] Large: selection list only. Each selected item gets its own scoped implementation PR.

## Open Questions

Which numbers should go ahead, and which should be declined or deferred?
