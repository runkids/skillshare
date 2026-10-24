# Feature Proposal: Optional prefix series in installed skill lists

Tracking issue: [#629](https://github.com/runkids/skillshare/issues/629).

## Problem

Independent skills deliberately named as a family can occupy many peer rows in a
large list. A local prototype folds `notes-auth`, `notes-search` and `notes-tag`
into an expandable `notes-*` series while leaving their installed folders intact.

On #625 the maintainer noted that name-based grouping can combine unrelated skills
and adds complexity on top of source/folder grouping. This proposal acknowledges
that concern and records a **separate, optional future direction**. It does not
ask to add series back to the hierarchy-first implementation or block that work.

## Proposed Solution

If the feature is useful at all, make series disabled by default and let users
explicitly choose a naming family. A prefix is a display hint, never evidence of
parentage, dependency, invocation order or native agent support.

- Preserve actual directory ancestry as a distinct relationship. Group only within
  the existing source/folder boundary; do not move rows across sources to combine
  an attractive name match.
- Keep skill identities, paths, install/update records, target declarations,
  ignore rules and sync behavior unchanged. A series is not an installed resource.
- Search/filter results must still reveal individual matching skills. Preserve
  sibling sort order and make pagination count logical series consistently,
  without splitting expanded members away from their series header.
- Keep per-skill selection as the initial behavior; a series expand control does
  not select members or apply bulk mutations. Any future series-select control
  must define its exact matched-member scope before implementation.
- Provide an obvious way to view the ordinary list. Use existing accessible expand
  controls, tokens and translated labels, rather than inventing another theme.

The local prototype automatically groups three or more names sharing the part
before their first hyphen. That threshold is a description of local experience,
not a proposed universal default. An explicit allowlist of series avoids guessing
that arbitrary common prefixes represent a useful family.

Likely areas, only if approved: reusable list grouping, `ResourcesPage`, a display
preference and locale strings. No folder migration or new runtime dependency.

## Alternatives Considered

- No prefix series: the current list and source/folder grouping remain valid.
- Real directory hierarchy only: more reliable and already the focus of #625.
- User-assigned groups/tags: more explicit, but requires a separate metadata design.
- Automatic prefix grouping for all users: easy to trigger and prone to false groups.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines), provisional if the idea is accepted.
- [ ] Large (10+ files, 500+ lines)

This PR adds one proposal only. It may remain deferred or be rejected independently
of #625. Locale coverage can increase implementation file count.

## Open Questions

1. Is a separate series dimension worth maintaining, even as an opt-in feature?
2. Would explicit families solve the false-grouping concern, or are tags preferable?
3. Should this stay a local customization rather than part of the dashboard?
