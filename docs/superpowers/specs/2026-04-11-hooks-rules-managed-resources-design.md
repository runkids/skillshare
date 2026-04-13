# Hooks And Rules Managed Resources Design

## Goal

Make `Hooks` and `Rules` in `/resources` behave like `Skills` and `Agents` in `v0.19.0`:

- `/resources` is a managed inventory surface
- tabs share the same top-level interaction model
- resources support real per-item target assignment
- discovery/import is not mixed into the managed inventory UI

## Decision

`/resources` will show only managed hooks and managed rules.

Discovered hooks and discovered rules remain part of discovery and collect flows. They do not appear as primary inventory rows inside `/resources`.

This matches the maintainer's resource model more closely:

- managed inventory lives in `/resources`
- unmanaged/discovered state is an input to import/collect
- synced target files are compiled output, not canonical source-of-truth state

## Why

The current hooks/rules implementation mixes two concepts in one page:

1. managed resources that Skillshare owns
2. discovered diagnostics from existing target files

Skills and agents do not mix those concepts in `/resources`. Their UI feels coherent because every row is the same kind of thing: a managed resource with local state and actions.

If hooks/rules keep discovered rows in the same inventory page, parity is only visual. The page would look like skills/agents while behaving like a diagnostics browser.

## Product Shape

### `/resources`

Tabs:

- `Skills`
- `Agents`
- `Hooks`
- `Rules`

All four tabs share:

- search
- sort
- view toggle
- source filter row
- right-click context menu pattern

Hooks and rules will not show a `Managed / Discovered` toggle.

### Discovery / import

Discovered hooks and rules move out of `/resources`.

They should appear where the user is deciding what to import into managed storage:

- collect/import pages
- collect previews
- possible detail/import surfaces tied to discovery workflows

The user experience becomes:

1. discover existing hooks/rules from target config files
2. collect selected items into managed storage
3. manage those collected items in `/resources`

## Data Model

### Managed rules

Managed rule records gain metadata required for parity with skills:

- `targets?: string[]`
- `sourceType?: "local" | "github" | "tracked"`
- tracked repo provenance when applicable

Semantics:

- missing / empty / `["*"]` means all compatible targets
- explicit list means only those targets receive compiled output
- `sourceType` backs the `All / Tracked / GitHub / Local` row in `/resources`

### Managed hooks

Managed hook records gain the same optional target metadata:

- `targets?: string[]`
- `sourceType?: "local" | "github" | "tracked"`
- tracked repo provenance when applicable

Semantics match managed rules.

### Compatibility

Existing managed rules/hooks without targets continue to behave as global resources synced to all compatible targets.

No existing managed content should require migration before it can load.

## Sync And Compile Behavior

### Rules

Managed rule sync must only compile a rule for a target when:

- the rule is compatible with that target family
- the target is included by the rule's `targets` metadata

### Hooks

Managed hook sync must only compile a hook for a target when:

- the hook is compatible with that target family
- the target is included by the hook's `targets` metadata

### Preview behavior

Managed rule/hook previews should reflect the same target filtering logic used by sync.

The UI must not offer a target assignment that preview/sync later ignores.

## UI Design

### Resources shell

Hooks and rules should mirror skills/agents, not approximate them.

That means:

- no separate `Managed / Discovered` row
- no second `All / codex` filter row
- a single source filter row directly under search, using the same visual position as skills/agents
- view toggle available in the same header row

### Filters

Because `/resources` becomes managed-inventory-only for hooks/rules, source filters should match the same conceptual categories used elsewhere.

For hooks/rules, these filters must be backed by persisted managed-record metadata, not fake counts.

Collect/create/import flows must assign provenance metadata when managed records are created:

- rules/hooks created directly in the UI default to `local`
- rules/hooks collected from tracked repos become `tracked`
- rules/hooks collected from non-tracked GitHub installs become `github`

### Right-click menu parity

Managed hooks and rules should support the same menu shape as skills:

- `Available in...`
- `View Detail`
- `Disable` / `Enable` if supported
- `Uninstall`

If hooks/rules do not currently support `disabled`, the menu should either:

- add real disabled state, or
- omit only that item while preserving the rest of the pattern

`Available in...` must be real target assignment backed by managed record metadata.

### Detail behavior

Token display should match upstream `v0.19.0` behavior:

- token stats belong on the resource detail page
- file viewer modal should not gain custom token UI beyond what upstream does

## Architecture Constraints

This work should follow `v0.19.0` architecture decisions instead of introducing a second resource model.

That means:

- preserve `/rules` and `/hooks` as compatibility aliases if needed
- keep `/resources` as the shared managed shell
- do not keep a special-case hooks/rules toolbar model
- do not encode discovery state as if it were managed inventory state

## Implementation Outline

1. Extend managed rule and managed hook record schemas to store `targets`
2. Load and save that metadata through CLI and server APIs
3. Update compile/sync/preview logic to honor per-item targets
4. Add target assignment UI and right-click menu parity for managed hooks/rules
5. Remove `Managed / Discovered` and discovery-only filter rows from `/resources`
6. Restrict `/resources` hooks/rules tabs to managed inventory only
7. Move discovered hooks/rules access to collect/import-oriented flows
8. Restore `ResourceDetailPage` token behavior to match `v0.19.0`

## Non-Goals

- redesigning the maintainer's `v0.19.0` resources architecture
- keeping discovered hooks/rules as first-class inventory rows inside `/resources`
- inventing fake source-filter counts for hooks/rules
- adding markdown editing features from the separate branch

## Risks

### Record format drift

Adding targets to managed records changes persisted schema. This must remain backward-compatible and be covered by round-trip tests.

### Sync regressions

Per-item target filtering affects compile output. Rules/hooks may silently stop syncing if filtering logic is wrong. Preview and sync must share the same targeting rules.

### UI parity drift

If `/resources` retains discovery-specific controls for hooks/rules, the surface will remain inconsistent and harder to maintain.

## Testing

### Backend

- managed rule store round-trip with and without `targets`
- managed hook store round-trip with and without `targets`
- compile/sync respects per-item targets
- preview respects per-item targets
- collect/create paths preserve provenance and targets defaults

### UI

- `/resources` tab order and chrome parity
- hooks/rules tabs show no `Managed / Discovered` toggle
- hooks/rules right-click menu exposes real `Available in...`
- target assignment updates the managed record and affects preview/sync state
- token display matches upstream detail-page behavior

## Recommendation

Proceed with managed-only hooks/rules in `/resources` and add true per-item target assignment.

This is the cleanest way to make hooks and rules behave like skills and agents without diverging from the maintainer's resource model.
