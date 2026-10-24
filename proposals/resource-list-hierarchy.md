# Feature Proposal: Make large skill lists easier to understand and manage

Tracking issue: [#624](https://github.com/runkids/skillshare/issues/624).

## Problem

A skill package can contain its own `SKILL.md` and several nested skills. The
Resources list presents those entries as peers, so a package with many children
can dominate the list. It is difficult to see the package's main entry, understand
which skills belong together, or compare unrelated skills without scrolling.

Names alone also do not explain what a skill does, and checking a skill's target
selection requires opening its detail page. This becomes repetitive when reviewing
a large shared library across several agents.

Related discussion: #247 asks for easier organization without losing upstream
Git links. This proposal focuses on the dashboard presentation of existing data;
it does not move installed folders or introduce tagging.

## Proposed Solution

### 1. Expandable parent and child rows

In the installed skills list view, display a nested skill below the nearest
ancestor directory that also contains a discovered skill. Collapse children by
default, with an explicit expand button, a child count, indentation, and keyboard
and screen-reader support. Preserve the existing grid, tree, and agents views.

For example, these existing paths:

```text
visual-pack/SKILL.md
visual-pack/skills/cover/SKILL.md
visual-pack/skills/portrait/SKILL.md
visual-pack/skills/portrait/skills/wardrobe/SKILL.md
standalone/SKILL.md
```

would initially appear as:

```text
> visual-pack                 3 nested skills
  standalone
```

Expanding the package reveals `cover` and `portrait`; expanding `portrait`
reveals `wardrobe`. A package's parentage comes from discovered paths, rather
than a guessed relationship based on similar names.

Filtering and searching must not hide a matching child merely because its parent
does not match. Promote such a child into the filtered result, or retain ancestor
context explicitly; open matching branches during search. Preserve sorting among
siblings, existing source/folder grouping, pagination, selection, and bulk actions.
Expanding a row must not select, install, enable, or sync anything.

### 2. Optional name-prefix series

Independent top-level skills such as `notes-auth`, `notes-search`, and `notes-tag`
may be easier to review as a collapsible `notes-*` series without moving their
folders. A local prototype uses a threshold of three matching first-hyphen
prefixes. This is a presentation hint, not a dependency or parent relationship.

Because common prefixes can group unrelated skills, the maintainer should decide
whether series are opt-in, configurable, or deferred. Real directory parentage
must take precedence. Series-level selection must state whether it covers only
direct members or all descendants and retain existing bulk-action confirmation.

### 3. Skill descriptions and target controls in context

As complementary list improvements, consider a description column between Name
and Targets, with truncation and access to the full text. Read existing skill
metadata, including skills excluded from every target; do not create a separate
user-maintained description store. Avoid one eager request per installed skill.
Keep narrow viewports usable and respect existing theme tokens and localization.

Target icons could expose the same include/exclude operation as the detail page,
with accessible labels and clear disabled, pending, and error states. Make clear
that editing target selection and applying Sync are separate actions. Preserve
global/project ownership, resource-kind restrictions, native compatibility rules,
and existing filters; clicking an icon must not navigate or select the row.
These two improvements can be evaluated separately from the hierarchy itself.

### Boundaries and implementation areas

- Dashboard: `ResourcesPage`, reusable row/grouping logic, and locale strings.
- Data: reuse discovered skill paths and existing target-filter APIs. Evaluate a
  shared metadata response if descriptions need additional server support.
- CLI, install locations, registries, ignore rules, sync semantics, and native
  agent discovery remain governed by their current behavior.
- Selecting or enabling a parent does **not** implicitly override an excluded
  child's settings or guarantee that every native agent will load nested skills.
- No new runtime dependency is proposed.

A local UI prototype informed the examples. This PR deliberately contains only
this proposal, following the project's contribution policy; it does not ask the
maintainer to merge the prototype or accept its heuristics unchanged.

## Alternatives Considered

- **Use only the existing tree view:** useful for directories, but it does not
  preserve the installed list's comparison columns and per-skill status controls.
- **Move all children into separate top-level installs:** changes install/update
  identity and can duplicate package contents; presentation should not require it.
- **Infer every parent from names:** confuses naming conventions with actual
  directory ancestry. Restrict name-based grouping to an optional series feature.
- **Keep a flat list with search:** remains available and is simple, but requires
  users to know names and does not explain package membership.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines), if all options and locales are implemented.

This PR adds one proposal document. A hierarchy-only implementation can be scoped
separately after maintainer feedback; descriptions and target toggles need not be
bundled with it.

## Open Questions

1. Should nested rows be default in list view, or a saved display preference?
2. During filtering, should unmatched ancestors remain visible as context?
3. Are prefix series useful enough to expose, and should grouping be opt-in?
4. Should selection and counts cover direct children or the full subtree? How
   should pagination avoid splitting a parent from its visible children?
5. Should the initial implementation include descriptions or target toggles, or
   keep those as independent follow-ups?
6. Which existing response should provide descriptions without eager per-row
   requests, and how should read-only/project-managed target states be presented?
