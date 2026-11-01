# Feature Proposal: Confirm capability expansion during updates

Issue: https://github.com/runkids/skillshare/issues/693

## Problem

An update can expand what a skill asks an agent to do without introducing a
finding that crosses the current audit threshold. Users need to see changes
in tools, scripts, destinations and server launch commands before accepting it.
Tracked updates already stage and audit candidates in `internal/update/tracked.go`;
that gate does not compare the old and new capability surface.

## Proposed Solution

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

## NVIDIA source and adaptation

The links below pin NVIDIA SkillSpector at commit
`6d0b7c849949cab9181e57798247f89e48ce5d6a`. They describe inspiration, not an
implementation of this proposal.

| Mechanism | Inspected source and tests | Relationship to this proposal |
|---|---|---|
| Manifest changes | [`_check_manifest_changes`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/mcp_rug_pull.py#L554-L613); [permission expansion test](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/analyzers/test_mcp_rug_pull.py#L130-L151) and [trigger addition/removal tests](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/tests/nodes/analyzers/test_mcp_rug_pull.py#L168-L202) | Direct inspiration: when given current and previous manifests, NVIDIA reports added permissions and added or removed trigger phrases. Comparing Skillshare's broader tools, script tiers, hosts and MCP commands is proposed here. |
| Permission pre-staging | [`_check_rp2`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/mcp_rug_pull.py#L449-L482) | Direct inspiration for a LOW-severity heuristic warning about manifest language suggesting future permissions. It does not establish that permissions were granted or code executed. |
| Surface extraction and update consent | [`_normalize_allowed_tools` and `_detect_capabilities`](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/mcp_least_privilege.py#L246-L308); [conditional manifest comparison](https://github.com/NVIDIA/SkillSpector/blob/6d0b7c849949cab9181e57798247f89e48ce5d6a/src/skillspector/nodes/analyzers/mcp_rug_pull.py#L713-L741) | Indirect extraction inspiration: NVIDIA normalizes allowed tools and detects capabilities, while comparison consumes a supplied previous manifest. Revision-bound central summaries, unknown baselines, pre-publication consent, unattended and batch policy, and dashboard parity are Skillshare design choices; the cited functions do not implement that update workflow. |

## Alternatives Considered

Rerunning the current audit remains useful but does not explain an expansion
relative to a previously accepted version. Comparing only manifest text misses
script and server changes. A new warning without a gate is easy to miss in batches.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimated implementation: 450–800 production lines plus regression fixtures,
metadata compatibility, CLI/dashboard output and documentation. Cover expansion,
unchanged content, missing baselines, extraction failures and declined batches.
This proposal includes no implementation. Maintainer approval of the linked issue
is required before implementation.

## Open Questions

- Is confirmation for every new host/script appropriate, or should policy allow
  a narrower set of high-risk expansions? Which changes count as equivalent?
- Should description/trigger changes ever block automatically?
- Should the expansion approval be one batch decision or one decision per skill?
- How should incomplete extraction be presented without implying full coverage?
