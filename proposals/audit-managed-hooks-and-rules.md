# Feature Proposal: Audit managed hooks and rule extras before save and sync

Issue: https://github.com/runkids/skillshare/issues/692

## Problem

Managed hook definitions can cause commands to run on target events. Rule
content shared through extras also influences agent behavior. The shared hook
mutation service validates and renders definitions, but neither it nor extras
sync applies an audit gate to this content.

## Proposed Solution

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

## Alternatives Considered

Hooks-only checks leave rule extras uncovered. Checking only CLI handlers
leaves API saves and later sync exposed. Treating all extras as rules would
change unrelated file synchronization without a useful content contract.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Estimate includes both mutation systems, policy plumbing, reports, docs and
fixture coverage. Acceptance should include a rejected fetch-and-execute hook,
a normal formatter hook, rule-body findings, source-only saves, sync-time
reference changes and explicit overrides. This proposal includes no implementation.

## Open Questions

- Is explicit rule-source classification the right default for existing extras?
- Should rejected source-only drafts be saveable with an override, or only previewed?
- Which skip/override contract should CLI and API expose, and how is its use shown?
- Should rules supplied through directory symlinks require a copy mode for any
  stronger guarantee than inspection at save/sync time?
