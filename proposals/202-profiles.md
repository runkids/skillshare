# Feature Proposal: Named skill profiles

Issue: [#202](https://github.com/runkids/skillshare/issues/202)

## Problem

#202 asks to assign skills to named profiles and switch between them easily. Existing target filters can express the selection, as discussed in #156. Users still have to edit filters for each target, run sync and repeat those edits when switching back. There is no named selection, and switching rewrites the config that some users share between machines.

## Proposed Solution

Keep the model small:

1. Define named skill include/exclude selections under `profiles:` in global or project config, using the existing `filepath.Match` target-filter syntax.
2. Keep one active profile per config in machine-local state, outside the config and shared settings.
3. Apply the profile as another narrowing layer alongside ignores, target filters and skill routing; source skills stay installed and unchanged.
4. Offer CLI definition/membership commands and `profile use`; add dashboard preview/switching first, then an editor and skill-tree assignment.
5. Plan and apply through the existing skill target runner. A switch can deselect many copies at once, making preservation of local edits important. Preserve modified managed copies and unmanaged entries during every sync with a profile active; discarding a listed modified managed copy requires a verified snapshot and a final fingerprint check.

Implementation extends configuration loading/formatting, both schemas, the selection seam and existing writers, with a small profile package, a shared pattern leaf and an apply adapter. CLI and HTTP handlers call the same rules. It uses existing YAML and file-lock dependencies. No new dependency is proposed.

Definitions preserve comments and unknown keys. An unknown selection key makes that profile unusable on this binary; config loading still succeeds, and an unavailable active profile refuses skill sync rather than deploying all skills. Repair commands and unrelated resource operations remain available. The active name never comes from a shared file.

No-profile installations retain their current behaviour. Copy manifests gain a fingerprint of the output after naming rewrites, separate from the source checksum. Definitions with no active profile are inert except for serializing skill sync with the profile lock. An incomplete switch records pending state and can be completed by the next sync.

The switch touches only enabled own merge/copy skill targets. Symlink-mode, disabled, registered-project targets and non-skill resources are excluded. Profiles never install, fetch or update anything; federation-installed skills keep their pins and source content. Adding profiles to #823's shared-section allowlist is a separate follow-up. Older binaries cannot enforce profile state and can drop definitions when saving, so users sharing config must upgrade every machine first.

## Alternatives Considered

- Edit existing target filters: already works, but leaves the naming, repeated edits and one-step switching gap.
- Switch separate configs with `SKILLSHARE_CONFIG`: works, but duplicates configuration and makes switching less convenient.
- Rewrite `.skillignore`: affects discovery and source-level operations instead of only deployment to targets.
- Store separate source trees or swap target folders: duplicates content and interferes with tracked skills.
- Put the active name in shared config: a frequent local choice would rewrite and switch other machines.
- Build tags, inheritance, multiple active profiles or per-target activation: unnecessary for the requested outcome; existing target filters remain the per-target lever.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines)

Delivery is by usable outcome: proposal; a conditional standalone copy-fingerprint bugfix only if reproduced; define and switch profiles in one core PR; assign skills and explain selection; dashboard see/switch; dashboard create/edit. The core is approximately 5,400 changed lines, kept together with its safety checks, lock, loaders, refusal paths and live-server behaviour. Ordered commits make it reviewable without splitting to fit a size limit. Estimates are forecasts, not measured diffs or test results.

Agents, extras, MCP, hooks and plugins in profiles; multiple active profiles; inheritance; shared defaults; and per-target activation are outside this series.

This is an unapproved reference implementation proposal for #202, currently labelled `pending`, not `approved`. The proposal reply is [on the issue](https://github.com/runkids/skillshare/issues/202#issuecomment-6843113583). Draft reference PRs have been offered; maintainer acceptance and merging remain undecided.

## Open Questions

These are the five points for the maintainer:

1. Whether you want this at all, given your answer in #156.
2. Delivery of the core slice: one pull request of about 5,400 lines, a stack you merge in an order you name, or a reference you implement from. CONTRIBUTING says implementation is normally yours. The author's current delivery plan is one core reference PR with ordered commits.
3. Whether the edit-preserving rule should apply to every copy-mode sync, not only when a profile is active. This proposal keeps existing behaviour for users without profiles.
4. The name `profile` beside `audit --profile`.
5. An empty profile selecting nothing.
