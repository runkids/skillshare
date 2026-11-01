# Feature proposal: opt-in semantic audit with isolated providers

Issue: https://github.com/runkids/skillshare/issues/698

## Status and problem

This is a proposal for discussion, not an implementation or approval. Static rules can miss natural-language instructions and mismatches between a skill's stated intent and its behavior. A model review could add a second source of findings, but it sends potentially sensitive and adversarial skill content to a provider, adds cost and nondeterminism, and runs a subprocess. No provider is assumed safe merely because its executable is present or authenticated. Maintainer approval of the feature issue is required before implementation.

## Proposed behavior

### Entry points and findings

Keep semantic review off by default. Propose `skillshare audit <path> --llm [--llm-provider claude]` and an explicitly requested `install --audit --llm` path. Never run it automatically during sync. Define an `internal/audit/llm` provider contract along the lines of `Name`, `Available` and `Complete`; availability must include the verified capability boundary, not just a binary or login probe. A missing, unsupported, unverified or unauthenticated provider is reported as unavailable with a clear skip reason; never silently switch providers or fall back to broader credentials.

Treat skill files and model output as untrusted. Split input into bounded, overlapping chunks with `L<n>:` line prefixes. Request structured JSON with at least `rule_id`, `severity`, `start_line`, `confidence` and `explanation`, validate IDs, severity, numeric values and source locations against the submitted file and range, and reject malformed or ambiguous results. Emit `LLM-` rule IDs with `analyzer="llm"` in the existing finding shape and JSON/SARIF. Dedupe overlap without losing evidence or source locations. Preserve static findings and normal behavior when semantic review is not requested.

### Capability and authentication boundary

Define a per-CLI specification table with exact argv, parser, authentication probe and a verified-policy flag. Use `exec.CommandContext` with no shell, validated model labels, prompts on stdin, an allowlisted child environment that excludes API keys and unrelated secrets, a replacement system prompt, a fresh temporary working directory removed after use, bounded stdout/stderr, timeout and process termination on overflow or cancellation. Verify that tools, MCP, hooks, settings discovery, slash commands and persistent sessions cannot run or load. Preserve only authentication access the chosen CLI actually requires and document that boundary, including the provider's destination and any unavoidable local credential access. A CLI that cannot satisfy and demonstrate the boundary remains unavailable. Do not mistake a read-only mode for proof of denied tool execution.

### Usage and provenance

Propose an optional `audit.llm` configuration for provider, model, per-slot model overrides, temperature and timeout, with explicit environment overrides. Specify config/environment precedence, model source, and requested versus resolved provider/model for every slot. Record requested versus honored controls, including temperature, seed and effort; mark unsupported or unavailable controls explicitly. Report actual provider-supplied token counters when present, with missing counters unknown rather than zero. Add a versioned `llm` object to JSON and SARIF `run.properties`, and concise text and operation-log summaries. Never include prompts, environment values or credentials in reports or logs, or present inferred prices as measured usage. Exact output schema and control support need maintainer agreement.

### Completeness and required mode

Represent `llm_status` as `not_requested`, `complete`, `incomplete` or `unavailable`, with limitations and per-file skip counts. Optional review still shows a warning and every unavailable, failed, skipped or budget-limited batch; absence of semantic findings must not imply the pass completed. Propose `--llm-required` to make a requested pass fail its gate unless all required work is complete, including missing provider, invalid output, skipped files or failed batches. Decide exit-code and install-policy interaction explicitly, including whether `--llm-required` implies `--llm`; no required mode silently succeeds on incomplete work.

### Costs, budgets and cache

Propose `--llm-max-concurrency` with initial default **2**, `--llm-max-calls`, `--llm-max-file-bytes`, and `--llm-timeout`, with corresponding configuration and bounded chunk/output sizes. A first-use billing warning precedes calls; `audit --llm --plan` estimates work and exits without calling a provider. Limit exhaustion marks the pass incomplete. Any estimate must be labelled as such and must not be presented as actual usage or a guaranteed price.

A verdict cache can avoid repeating unchanged work. Key it to the skill/content hash, prompt version, model and provider, and engine/analysis and relevant configuration identity so a changed input or policy cannot reuse stale results. Provide `--llm-no-cache` and visible cache-hit provenance. Agree retention, permissions, invalidation, storage of sensitive content and any monetary ceiling before implementation. Caches cannot hide missing batches or provider failure as fresh complete results.

### Advisory triage of static findings

An optional second pass receives redacted static findings grouped by file and limited context. Associate each response by stable finding ID and validate a schema containing `finding_id`, a confirm/dismiss/downgrade verdict and reason. Store the model verdict and reason separately, including in SARIF properties. Default to advisory: retain the original finding, severity, evidence, scoring and exit behavior; never silently remove HIGH or CRITICAL findings. A model disagreement is not proof that a rule is wrong.

The original idea also proposed `--llm-dismiss` to exclude dismissed non-CRITICAL findings from gating. That wording includes HIGH while the original safety requirement says not to silently drop HIGH/CRITICAL. Keep this as an explicit unresolved maintainer decision: whether dismissal can ever affect gating, whether HIGH requires explicit consent and visible disclosure or must remain gate-effective, and how originals, audit logs and SARIF remain available. Do not assume that a flag or model verdict resolves this conflict. Without such an approved opt-in, model triage has no gate effect.

## NVIDIA source and adaptation map

These links pin SkillSpector at `6d0b7c849949cab9181e57798247f89e48ce5d6a`. They identify inspiration, not an assertion that Skillshare has the same implementation or that the entire proposal is supplied upstream.

| Mechanism | Inspected NVIDIA source and tests | Relationship and Skillshare decisions |
|---|---|---|
| Semantic stage and input | [`semantic_security_discovery.node`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/semantic_security_discovery.py#L108-L150); [`chunk_file_by_lines`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/llm_analyzer_base.py#L872-L888); [batching and LLM guard tests](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/analyzers/test_semantic_security_discovery.py#L291-L355) | Direct inspiration for an optional analyzer and overlapping, line-aware batches. Skillshare CLI, provider contract and install opt-in are proposed here. |
| Structured results | [`LLMFinding`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/llm_analyzer_base.py#L632-L696) | Direct typed finding and conversion analogue. NVIDIA clamps some invalid numeric values; strict rejection, `LLM-` IDs and Skillshare SARIF are separate choices. |
| Provider isolation | [`_scrub_env`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/providers/_agent_cli.py#L125-L137); [Claude argv](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/providers/_agent_cli.py#L270-L286); [Codex disabled reason](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/providers/_agent_cli.py#L313-L327) | Direct examples of environment allowlisting and per-CLI restrictions. Codex is disabled upstream pending a verified deny-all-tools policy; these examples do not establish a universal OS sandbox or validate any Skillshare provider. |
| Usage and controls | [`inference_usage` and controls](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/llm_analyzer_base.py#L1124-L1143) | Direct partial analogue for provider-reported usage and controls. Per-slot config, output schema, text/oplog and unknown-value handling here remain Skillshare design. |
| Failed passes | [`outcome_for_llm_batch_failure`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/inspection_ledger.py#L296-L303); [CLI incomplete gate](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/cli.py#L950-L960) | Partial analogue: upstream accounts for failed work and has a general incomplete gate. The `--llm-required` flag and status enum are Skillshare-specific proposals. |
| Bounds and cache | [chunk limits](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/llm_analyzer_base.py#L872-L888); [CLI byte/time ceilings](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/providers/_agent_cli.py#L73-L76) | Direct inspiration for bounded calls. These spans do not demonstrate a monetary limit or verdict cache. Cache keys, no-cache flag, first-use warning and any cost policy require separate design. |
| Static-finding triage | [`MetaAnalyzer.apply_filter`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/meta_analyzer.py#L466-L482); [matching and annotations](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/meta_analyzer.py#L553-L577); [fallback tests](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/test_meta_analyzer_fallback.py#L184-L229) | Direct analogue for model assessment while retaining deterministic findings. NVIDIA explicitly preserves originals and does not downgrade them; a dismissal gate would be a distinct, unapproved Skillshare policy. |
| Offline verification | [semantic batching/guard tests](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/analyzers/test_semantic_security_discovery.py#L291-L355) | Related offline test inspiration; these lines do not establish Skillshare's replay format or precision/recall gate. |

## Alternatives and objections

More static rules remain useful and preserve deterministic offline operation, but cannot settle every intent question. An unrestricted interactive agent CLI would introduce model-driven tool execution. A cloud SDK has a different authentication and disclosure design and is not implied by local CLI support. Maintainers may decide the privacy, security, cost, maintenance and nondeterminism burden is too high for Skillshare; the dependent isolation, usage, cache, required-pass, triage and test ideas do not stand as independent builds without the semantic stage.

## Proposed verification and scope

This is a large design (prior estimate: 1,800–3,500 production lines plus 1,200–2,500 test lines, not measured implementation). Settle provider isolation feasibility before enabling any provider. Do not claim safety or detection performance from this proposal.

Acceptance requires offline tests for:

- Exact per-provider argv, auth probe and child env, no secret leakage, no shell, no tool/MCP/settings/hook/session access, model-label validation, fresh cwd cleanup, output overflow kill, deadline and cancellation. A missing or unverified CLI is unavailable with no fallback.
- Chunk overlap and line numbering, duplicate boundaries, validated finding fields and locations, `LLM-` identity, malformed/empty/truncated JSON, and conversion through existing JSON and SARIF, including a mock finding.
- Status enum, per-file limitations, optional warnings and `--llm-required` failure when one mocked batch fails or is skipped; unchanged static results, gating and exits with no semantic option.
- Per-slot config/environment precedence and model sources; supported/unsupported controls; missing counters remaining unknown; JSON/SARIF/text/oplog provenance with prompts, env and credentials absent.
- Concurrency default and call/file/time limits, first-use warning, no-call plan, limit-to-incomplete propagation, cache hits and invalidation for content, prompt, model, provider, engine and policy changes, and `--llm-no-cache`.
- Stable finding-ID triage, redaction and advisory preservation of originals including HIGH/CRITICAL; if dismissal is approved, explicit visibility and tests for the agreed HIGH/CRITICAL gate policy.
- Canned mock provider and prompt-hash-keyed record/replay fixtures (`SKILLSHARE_LLM_REPLAY_DIR` proposed); ordinary Go tests require no network or billing. Keep a small labeled corpus and an explicitly invoked evaluation lane, provisionally `make eval-llm`, reporting precision/recall; do not imply a measured result or that evaluation establishes security.

## Maintainer questions

1. Should Skillshare add this large, nondeterministic and potentially disclosing feature at all? Which supported CLIs can prove the capability and authentication boundary, including across CLI upgrades?
2. What content may leave the machine, to which providers, under what user disclosure or consent, and what cache retention and permissions are acceptable?
3. Which providers, model slots, config/environment precedence, output schema and controls are supported first? Which caps, cost warning and any monetary ceiling should be defaults?
4. Should `--llm-required` imply `--llm`, and what exit and install-policy behavior applies to incomplete or unavailable work?
5. Is advisory triage sufficient? If `--llm-dismiss` is allowed to change gating, how is the non-CRITICAL versus HIGH/CRITICAL ambiguity resolved and made explicit to users?
6. What is the staged implementation and offline evaluation scope after approval? No provider isolation feasibility test, live model run or implementation is included here.
