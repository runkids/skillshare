# v0.19.0-Aligned Managed Resources Refactor

Date: 2026-04-11
Status: Approved for planning

## Summary

This design aligns the branch's managed `rules` and `hooks` work with the architectural direction established in Skillshare `v0.19.0`.

The refactor has two goals:

1. Preserve `v0.19.0`'s product and architecture decisions.
2. Remove the branch-local duplication introduced while porting managed `rules` and `hooks` onto a newer base.

The result is not a full rewrite of Skillshare sync and collect. Instead, it is a targeted refactor that:

- keeps `v0.19.0`'s existing skills and agents architecture intact
- makes `/resources` the primary management surface for `skills`, `agents`, `rules`, and `hooks`
- extracts shared managed-resource orchestration so CLI and server entrypoints stop re-implementing the same `rules` and `hooks` sync and collect flows

## Problem

After porting the managed `rules` and `hooks` work onto `v0.19.0`, the branch now has two kinds of drift:

### UI drift

`v0.19.0` introduced a consolidated `/resources` page for `skills` and `agents`, with a standardized header, search, filters, sort controls, and bulk interaction model.

Managed `rules` and `hooks` were added as separate first-class pages and sidebar entries, each with their own duplicated page shell. This diverges from the new `v0.19.0` product shape and makes the UI feel older than the base version.

### Orchestration drift

Managed resource sync and collect logic is spread across multiple layers:

- CLI sync code in `cmd/skillshare/sync.go`
- CLI managed-resource helpers in `cmd/skillshare/managed_resources.go`
- server sync code in `internal/server/handler_sync.go`
- server managed-resource helpers in `internal/server/managed_resource_sync.go`
- server collect handlers in `internal/server/handler_managed_rules.go` and `internal/server/handler_managed_hooks.go`

The managed `rules` and `hooks` code paths perform very similar work:

- resolve target-specific compile context
- load managed records
- scan discovered items when collecting
- compile managed output
- apply compiled files
- prune managed rule orphans where needed
- report created, overwritten, skipped, updated, and pruned results

The logic is currently correct enough to use, but the same orchestration concepts are repeated in different shapes for CLI and server code.

## Goals

- Preserve `v0.19.0` architecture decisions rather than replacing them.
- Make `/resources` the single management surface for `skills`, `agents`, `rules`, and `hooks`.
- Remove `Rules` and `Hooks` as primary sidebar destinations.
- Keep existing managed rule and hook editing pages working.
- Extract shared managed-resource sync and collect orchestration into internal reusable code.
- Keep CLI behavior and HTTP API behavior functionally stable unless a mismatch is clearly a bug.
- Reduce code duplication without forcing skills, agents, extras, and managed resources into one oversized generic engine.

## Non-Goals

- No rewrite of the core skills or agents sync architecture introduced in `v0.19.0`.
- No attempt to unify every syncable thing in Skillshare into a single global planner.
- No new markdown editing experience for `SKILL.md` or other files in this branch.
  That work exists on a separate branch and is explicitly out of scope here.
- No redesign of managed rule and hook detail editors beyond route and navigation integration.
- No broad renaming of public CLI flags or API payloads.

## Design Principles

### Follow `v0.19.0` first

Where this branch and `v0.19.0` disagree, `v0.19.0` wins unless there is a concrete regression or missing capability required for managed `rules` and `hooks`.

### Extract branch-added duplication, not product abstractions for their own sake

The refactor should remove duplicated orchestration and duplicated page shells that were introduced by the managed resource port. It should not invent a new meta-framework that the base project does not need.

### Keep entrypoints thin

CLI commands and HTTP handlers should validate inputs, call shared orchestration code, and format outputs. They should not own compile/apply/scan/prune workflows directly.

## Current-State Assessment

### UI

- `/resources` already provides the canonical `v0.19.0` shell for browsing `skills` and `agents`.
- `/rules` and `/hooks` duplicate the same shell structure with separate page-level implementations.
- The sidebar still exposes `Rules` and `Hooks` as primary navigation items, which conflicts with the desired consolidated `v0.19.0` direction.
- Managed rule and hook detail routes already exist and are useful. Those should stay.

### Backend

- CLI sync and server sync each run managed `rules` and `hooks` through similar but separate target loops.
- CLI collect and server collect each reconstruct managed collect behavior separately.
- Managed resource compilation and application already rely on lower-level packages such as `internal/resources/rules`, `internal/resources/hooks`, and `internal/resources/apply`. Those lower-level packages should remain the foundation.

The missing layer is a shared managed-resource orchestration package that sits above the low-level resource packages and below the CLI/server entrypoints.

## 1. UI Surface Alignment

`/resources` becomes the canonical browsing surface for:

- `skills`
- `agents`
- `rules`
- `hooks`

The `ResourcesPage` shell remains the visual and interaction model established by `v0.19.0`. Managed `rules` and `hooks` are added as additional top-level tabs in that shell.

Each tab owns its own data adapter for:

- list query
- tab counts
- search matching
- filter options
- sort options
- row or card rendering
- primary actions such as `New Agent`, `New Rule`, or `New Hook`

This keeps the page aligned with `v0.19.0` without forcing `skills`, `agents`, `rules`, and `hooks` into a single deeply generic renderer.

### Route behavior

Primary routes:

- `/resources` with tab state in the URL via `tab=skills|agents|rules|hooks`
- `/resources/new?kind=agent` stays as-is
- managed rule and hook detail/edit routes stay as-is

Compatibility routes:

- `/rules` redirects to `/resources?tab=rules`
- `/hooks` redirects to `/resources?tab=hooks`

Rules and hooks keep their existing secondary browsing mode inside the resources tab:

- `mode=managed`
- `mode=discovered`

Compatibility redirects should preserve this mode when present. For example:

- `/rules?mode=discovered` redirects to `/resources?tab=rules&mode=discovered`
- `/hooks?mode=managed` redirects to `/resources?tab=hooks&mode=managed`

Compatibility detail routes remain valid:

- `/rules/new`
- `/rules/manage/*`
- `/rules/discovered/:ruleRef`
- `/hooks/new`
- `/hooks/manage/*`
- `/hooks/discovered/:groupRef`

This preserves old links, tests, and navigation assumptions while making `/resources` the primary surface.

### Navigation changes

The sidebar removes `Rules` and `Hooks` as top-level manage items. `Resources` remains the single entrypoint.

Dashboard shortcuts or cards that currently point to `/rules` or `/hooks` may continue to work through redirects, but primary navigation should point to `/resources` tabs.

## 2. Shared Managed-Resource Orchestration

Introduce a new internal orchestration layer for managed `rules` and `hooks`.

The package location can be finalized during implementation, but the responsibility should be narrow and explicit. A plausible home is:

- `internal/resources/managed`

This package should own shared application-level workflows for managed resources, not the lower-level format-specific compile logic.

### Responsibilities

The shared managed orchestration layer should provide:

- managed resource selection and normalization for `rules`, `hooks`, or both
- sync planning and execution per target
- collect planning and execution from discovered inputs
- stable result structures for sync and collect
- dry-run behavior
- pruning behavior where applicable
- shared validation and collision detection paths where current CLI and server logic overlap

### Non-responsibilities

The shared layer should not:

- replace the lower-level `managedrules` and `managedhooks` packages
- take ownership of skills or agents sync
- become responsible for HTTP concerns or terminal rendering

## 3. Managed Sync Design

Managed sync remains a separate concern from skills and agents sync, but uses one shared executor underneath both CLI and server callers.

### Shared sync input

The sync executor should accept an explicit input object containing:

- selected resources: `rules`, `hooks`, or both
- target set
- project root or global root context
- dry-run flag
- force flag if required by the calling surface

### Shared sync behavior

For each target:

1. Resolve whether managed `rules` apply for that target.
2. Resolve whether managed `hooks` apply for that target.
3. Load managed records from the correct project or global store.
4. Compile target-specific output.
5. Apply compiled files.
6. Prune managed rule orphans where the target supports owned rule directories.
7. Return structured per-resource results.

### Result model

The shared result model should capture per target:

- resource kind
- updated files
- skipped or unchanged files
- pruned files
- errors

CLI and server code can then map this shared result into their own output shapes without rebuilding the core workflow.

### CLI integration

`cmd/skillshare/sync.go` and `cmd/skillshare/managed_resources.go` should stop directly owning the managed `rules` and `hooks` target workflows.

They should instead:

- parse CLI flags and resource selections
- prepare target entries and context
- call the shared managed sync executor
- format CLI summary lines and JSON output

### Server integration

`internal/server/handler_sync.go` and `internal/server/managed_resource_sync.go` should stop duplicating the managed target workflow.

They should instead:

- parse HTTP body
- select resources
- call the shared managed sync executor
- return UI-oriented JSON results

The server should continue to compose managed results with skills and agents results at the HTTP layer, because that composition is part of the existing `v0.19.0` server architecture.

## 4. Managed Collect Design

Managed collect currently exists in both CLI and server flows but is implemented through different input gathering paths.

The shared collect layer should separate:

- discovery selection
- managed collect execution

### Shared collect input

The collect executor should work on explicit discovered items already chosen by the caller:

- discovered rule items
- discovered hook items or normalized hook groups
- strategy such as overwrite or skip
- project or global root context
- dry-run flag when supported

This keeps the shared layer independent from whether the caller is a CLI that scanned the filesystem or an HTTP handler that received selected IDs from the UI.

### CLI integration

CLI collect continues to:

- scan local skills as it already does
- scan discovered rules or hooks as needed
- resolve target or global context
- call the shared managed collect executor

The CLI remains responsible for mixed flows like `skills + rules + hooks`, because that composition is part of the CLI command behavior. But the managed portion of that flow should not reimplement collect internals.

### Server integration

The server collect handlers for managed rules and hooks continue to:

- validate request bodies
- map selected IDs or group IDs to discovered items
- call the shared managed collect executor
- return created, overwritten, and skipped results

This keeps existing API semantics stable while removing branch-local duplication.

## 5. Package and Boundary Plan

The intended boundaries after refactor are:

### Low-level resource packages

- `internal/resources/rules`
- `internal/resources/hooks`
- `internal/resources/apply`

These continue to own record storage, compile behavior, and low-level application helpers.

### Shared orchestration package

A new package owns:

- managed sync execution
- managed collect execution
- common result types
- common target/resource selection helpers used specifically for managed resources

### CLI layer

The CLI layer owns:

- flag parsing
- command routing
- terminal rendering
- CLI JSON output shape
- composition with skills, agents, and extras flows already established in `v0.19.0`

### Server layer

The server layer owns:

- HTTP request parsing and validation
- response formatting
- route registration
- composition of managed results with existing UI sync responses

## 6. Testing Strategy

The refactor should be test-led around behavior preservation.

### Shared orchestration tests

Add focused tests for the new shared managed orchestration package covering:

- sync rules only
- sync hooks only
- sync rules and hooks together
- collect rules only
- collect hooks only
- dry-run behavior
- target incompatibility behavior
- rule prune behavior
- collect collision and invalid input behavior

### CLI regression tests

Update or preserve CLI tests to ensure:

- `sync --resources rules`
- `sync --resources hooks`
- `sync --resources rules,hooks`
- `collect --resources rules`
- `collect --resources hooks`
- `collect --resources skills,rules,hooks`

all still behave as before from the user's perspective.

### Server regression tests

Preserve or add tests for:

- `/api/sync` with managed resources selected
- managed rule collect endpoint
- managed hook collect endpoint

### UI tests

Add or update tests for:

- `/resources` tab switching across `skills`, `agents`, `rules`, and `hooks`
- removal of `Rules` and `Hooks` from primary sidebar navigation
- compatibility redirects from `/rules` and `/hooks`
- existing new/manage/discovered rule and hook routes

## 7. Migration and Rollout

This refactor should be implemented in the following order:

1. Extract shared managed sync executor and migrate CLI plus server sync call sites.
2. Extract shared managed collect executor and migrate CLI plus server collect call sites.
3. Refactor `/resources` to host `rules` and `hooks` tabs using the `v0.19.0` shell.
4. Convert `/rules` and `/hooks` list routes into compatibility redirects or thin wrappers.
5. Remove `Rules` and `Hooks` from sidebar primary navigation.
6. Update tests last to reflect the final route and navigation model while preserving compatibility.

This order keeps the highest-risk backend changes isolated from the UI consolidation and makes regressions easier to localize.

## 8. Error Handling

### Backend

- Shared executors should return typed or structured errors where the caller needs to distinguish invalid input from execution failures.
- HTTP handlers keep responsibility for mapping invalid input to `400` and execution failures to `500`.
- CLI callers keep responsibility for rendering warnings, failures, and JSON error output in the existing style.

### UI

- `/resources` tab content should preserve current empty states and fetch error behavior.
- Compatibility redirects should be transparent and should not alter existing rule or hook editor routes.

## 9. Risks and Mitigations

### Risk: over-generalizing the shared layer

Mitigation:
Keep the new package scoped to managed `rules` and `hooks` only. Do not force skills, agents, or extras into it.

### Risk: breaking existing routes and tests

Mitigation:
Keep detail and editor routes stable and add compatibility redirects for `/rules` and `/hooks`.

### Risk: changing CLI behavior while cleaning up internals

Mitigation:
Treat current CLI behavior as the compatibility surface and verify it with regression tests before and after refactor.

### Risk: UI consolidation becomes a design rewrite

Mitigation:
Use the existing `ResourcesPage` shell from `v0.19.0` as the only list-page visual model. Do not create a second competing shell.

## 10. Success Criteria

This refactor is successful when all of the following are true:

- `rules` and `hooks` are accessed from `/resources` tabs as part of the standardized `v0.19.0` management surface
- sidebar primary navigation no longer treats `Rules` and `Hooks` as separate peer pages
- CLI sync and collect for managed `rules` and `hooks` use shared internal orchestration
- server sync and managed collect endpoints use the same shared internal orchestration
- managed rule and hook detail/edit routes still work
- tests pass without changing the intended behavior of existing user-facing commands and APIs

## Out of Scope Reminder

Inline markdown editing in the Skillshare UI, including editing `SKILL.md` inside a modal, is intentionally excluded from this branch and should remain on the separate worktree or PR dedicated to that feature.
