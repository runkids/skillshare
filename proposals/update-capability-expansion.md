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
This proposal includes no implementation.

## Open Questions

- Is confirmation for every new host/script appropriate, or should policy allow
  a narrower set of high-risk expansions? Which changes count as equivalent?
- Should description/trigger changes ever block automatically?
- Should the expansion approval be one batch decision or one decision per skill?
- How should incomplete extraction be presented without implying full coverage?
