# Feature Proposal: Skill descriptions in the installed list

Tracking issue: [#627](https://github.com/runkids/skillshare/issues/627).

## Problem

A skill name rarely explains its full purpose. Reviewing a large installed library
requires opening many detail pages simply to read metadata already present in
`SKILL.md`. A local prototype puts that description beside the name.

The maintainer suggested in #625 that descriptions could be considered as a
separate small change. This proposal records that independent use case and leaves
the hierarchy-only scope of #625 unchanged.

## Proposed Solution

Offer descriptions in the installed **skills list** view, between Name and
Targets. Keep the current compact list available through a preference or responsive
treatment; a description column need not be the right default for every user.

- Read the description from the shared source skill. Include disabled skills and
  skills excluded from every receiving target. A same-named target-local skill
  must not supply another source skill's description.
- Handle existing YAML scalar and block descriptions. Render plain text, preserve
  meaningful text, truncate long rows, and provide keyboard-accessible full text.
  Missing or unreadable descriptions must not break the list.
- Keep source identity, sorting, filters, selection, actions, status and Sync
  unchanged. Preserve existing agent, grid and tree views.
- Use current theme tokens and localized labels; inspect populated non-English,
  light/dark, Clean/Playful and narrow-screen layouts before implementation lands.
- Avoid one eager request per installed skill. Prefer metadata in an appropriate
  shared list response, or a bounded visible-row strategy with existing query keys.
  Confirm invalidation after a skill edit/update so text cannot remain stale.

The local prototype reused Analyze descriptions and fetched missing source files.
That demonstrates the presentation, not a recommendation to ship its eager
fallback unchanged. The current resources list response has no description field,
so the maintainer should choose the shared data contract before implementation.

Likely areas: `ResourcesPage`, resource API types/metadata, existing frontmatter
helpers and locale strings. No new runtime dependency or description store.

## Alternatives Considered

- Keep only detail pages: simplest, but repeats navigation during library review.
- Tooltips on names: compact, but less discoverable and needs accessible full text.
- Edit a separate summary store: duplicates existing metadata and can become stale.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [x] Medium (3-10 files, 200-500 lines), provisional data/UI estimate.
- [ ] Large (10+ files, 500+ lines)

This PR adds one proposal only. Locale coverage may increase implementation file
count. It makes no product/API change and does not include the personal prototype.

## Open Questions

1. Should descriptions be an optional column, a second line, or responsive by size?
2. Which shared response should own source descriptions and their invalidation?
3. Is this narrow enough for a small implementation PR after design agreement?
