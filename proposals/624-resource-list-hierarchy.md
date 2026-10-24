# Feature Proposal: Expandable skill package rows

Tracking issue: [#624](https://github.com/runkids/skillshare/issues/624).

## Problem

A skill package can contain its own `SKILL.md` and several nested skills. The
installed Resources list presents those entries as peers, so a package with many
children can dominate the list. It is difficult to find the main entry and
understand which skills belong together without scrolling through every child.

Related discussion: #247 asks for easier organization without losing upstream
Git links. This proposal changes only how existing skills appear in the dashboard
list; it does not move installed folders or introduce tagging.

## Proposed Solution

### Parent and child rows from real directory ancestry

In the installed skills list view, display a nested skill below the nearest
ancestor directory that also contains a discovered skill. Collapse children by
default, with an explicit expand button, a descendant count, indentation, and
keyboard and screen-reader support. Preserve the existing grid, tree, and agents
views. Similar names do not establish a parent relationship.

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
reveals `wardrobe`. The count includes all discovered descendants, not just
immediate children, and is distinct from a filtered result count.

### Search, filtering, and selection

A matching child must remain visible even when its parent does not match. Show
it as a top-level result if its parent is absent from the filtered group. Open
matching branches during search. Keep sibling sorting and the existing source
and folder group boundaries; a child whose parent is outside its current group
remains top-level in that group.

Keep selection attached to individual skill identities and preserve the existing
select-all and bulk-action scope across matching skills. Collapsing a package
does not change selection; selecting its parent does not implicitly select its
children. Expanding a row must not select, install, enable, or sync anything.

### Pagination

**A collapsed package counts as one top-level pagination entry.** Paginate the
filtered top-level roots within the existing groups; an independent skill also
counts as one entry. A matching child promoted because its parent is absent
counts as a top-level entry in that result.

Expanding a package displays its children on that same page without consuming
additional top-level page slots or splitting the subtree across pages. Expansion
does not change the top-level pagination total. Label that total distinctly from
the overall skill count and nested-skill badges; group headers do not consume
pagination slots. Existing page sizes and load-more controls can operate on these
root entries rather than cutting through a package's children.

### Boundaries and implementation areas

- Dashboard: `ResourcesPage`, reusable hierarchy/row logic, and locale strings.
- Data: reuse discovered skill paths; no new hierarchy metadata or dependency.
- Preserve CLI behavior, install locations, registries, ignore rules, sync
  semantics, source links, and native agent discovery.
- Enabling a parent does **not** override an excluded child's settings or
  guarantee that a native agent will load nested skills.

A local UI prototype informed the examples. This PR contains only this proposal,
following the project's contribution policy. The first implementation covers
real directory ancestry only.

## Alternatives Considered

- **Use only the existing tree view:** useful for directories, but it does not
  preserve the installed list's comparison columns and per-skill status controls.
- **Move children into separate top-level installs:** changes install/update
  identity and can duplicate contents; presentation should not require it.
- **Infer parentage from names:** can group unrelated skills. Directory ancestry
  provides an existing, concrete relationship instead.
- **Keep a flat list with search:** simple, but does not explain package membership.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines), initial hierarchy logic and UI estimate.
- [ ] Large (10+ files, 500+ lines)

This PR adds one proposal document. The implementation estimate excludes future
ideas below; translation coverage may increase the number of touched files.

## Future Ideas

These are outside the first implementation:

- A description column could be considered later as a separate small change.
- Per-row target editing already exists in the row menu as **Set targets**; this
  proposal does not add another editing control.
- Prefix series are deferred. Name-based grouping can combine unrelated skills
  and would introduce another grouping on top of source/folder grouping.

## Open Questions

1. Should hierarchy be the default in list view, or a saved display preference?
2. Are promoted matching children sufficient during filtering, or would retaining
   an unmatched ancestor as non-matching context be preferable?
3. For a package with many visible descendants, is the existing list rendering
   sufficient, or should a follow-up consider virtualization while keeping each
   package on one logical page?
