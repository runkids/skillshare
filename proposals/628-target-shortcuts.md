# Feature Proposal: Optional target icon shortcuts in skill rows

Tracking issue: [#628](https://github.com/runkids/skillshare/issues/628).

## Problem

During repeated cross-agent library review, opening a row menu or detail page for
every target change is repetitive. A local prototype makes receiving-agent icons
in skill rows actionable, so the user can change a target filter in context.

The existing row-menu **Set targets** action already supports target editing, as
the maintainer noted on #625. This proposal is only an optional shortcut; it does
not claim a missing core capability or replace that menu.

## Proposed Solution

Explore a deliberate, opt-in editing affordance for target icons in the installed
skills list. Keep the ordinary status display and existing controls available.
Do not expand #625's hierarchy-only scope.

- Make the editable state clear before a click changes configuration. Use
  accessible names/pressed states, adequate hit areas and keyboard operation.
  Clicking a control must not navigate to details or select the enclosing row.
- Reuse supported existing target policy and mutation ownership. Preserve unrelated
  include/exclude patterns and serialize pending changes; show failures without
  presenting an unsaved selection as successful.
- Preserve disabled resources, target-level off switches, native incompatibility,
  managed/project ownership and read-only states. Offer no shortcut when an action
  cannot be represented safely by the existing supported operation.
- Clearly separate editing selection from applying Sync. Do not automatically
  sync, install, enable child skills, or change a package's descendants.
- Use existing API clients, query events, theme tokens and translations. Keep grid,
  tree and agent views unchanged in an initial implementation.

The local prototype edits receiving-target include/exclude filters using existing
logic. That is **not identical** to changing a resource's declared targets through
Set targets. The UI must name which setting it changes; implementation should not
introduce a third policy or silently conflate these two existing settings. If this
cannot stay understandable, opening the existing editor from an icon is preferable
to a new one-click mutation.

Likely areas: `ResourcesPage`, existing target-filter helpers and locale strings.
No new runtime dependency is proposed. The prototype is local reference behavior,
not an upstream implementation or proof that every target/state is supported.

## Alternatives Considered

- Retain only Set targets/detail controls: clear and already supported.
- Let an icon open that editor: fewer navigation steps, with existing semantics.
- Immediate toggles by default: fast, but increases accidental edits and ambiguity;
  an explicit editing affordance is preferable for evaluation.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines), provisional if direct editing is accepted.
- [ ] Large (10+ files, 500+ lines)

This PR adds one proposal only. Reusing the existing editor could make any eventual
implementation smaller. Translation coverage can increase the touched file count.

## Open Questions

1. Is an icon shortcut useful enough beside the existing row menu?
2. Should it open the current editor rather than mutate configuration directly?
3. If direct edits are considered, which existing setting should it expose and how
   should global/project ownership and pending/error states be communicated?
