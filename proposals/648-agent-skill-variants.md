# Feature Proposal: Agent-specific variants of a tracked skill

## Problem

Issue: [#648](https://github.com/runkids/skillshare/issues/648).

A tracked repository is a Git repository installed under Skillshare's source directory.
Some producers ship intentionally different files for the same skill, one per agent:

```text
_roborev/internal/skills/claude/roborev-fix/SKILL.md
_roborev/internal/skills/codex/roborev-fix/SKILL.md
```

At Skillshare revision `e64fac67`, these become two discovered skills with no inferred
target restriction. With default flat target naming, status reports `_roborev: 2 skills`;
merge sync sends both files to both Claude and Codex. This was reproduced with actual producer files from
[roborev revision 626dea47](https://github.com/kenn-io/roborev/tree/626dea478c5d5ab5c1d25845ce1474120f9c0f7b/internal/skills).

The Claude file uses `/roborev-fix` and `dangerouslyDisableSandbox`; the Codex file
uses `$roborev-fix` and `sandbox_permissions`. Choosing one file globally or merging
their contents would discard instructions needed by the other agent.

Per-target include/exclude filters and source-local target overrides already route
the copies correctly. `.skillignore` can hide unused variants. These controls
require consumers to describe the producer's layout and still count copies separately.

## Proposed Solution

Represent one logical skill with several physical variants inside a tracked repository.
Keep every variant's source path, files, and bytes intact; select existing files
for each target without format conversion or synthesized frontmatter.

### Identity and detection

For a recognized layout, identify a logical skill by this tuple:

```text
(tracked repository, shared parent before agent component, complete suffix after agent component)
(_roborev, internal/skills, roborev-fix)
```

Only the agent component differs between variants. Preserve the entire suffix:
`internal/skills/{claude,codex}/review/roborev-fix` is a different logical skill
from `internal/skills/{claude,codex}/roborev-fix`.
Different repositories or different shared parents never merge, even when their
`SKILL.md` names or final directory names are identical. Repositories nested with
`--into` retain their actual repository boundary.

Recognition must be bounded to an approved layout or explicit opt-in; recognizing
any directory named `claude` anywhere would create false groups. Activation and
single-variant recognition are open questions below. Ordinary skills keep their
existing discovery behavior.

### Routing and precedence

Extend the existing shared discovery and target-selection paths rather than adding
separate variant walkers or per-command routing rules:

- Discovery retains physical entries and attaches logical identity and agent identity.
- Resolve an agent through the configuration's target registry and canonical aliases.
  Account targets such as `codex-work` use their configured `Agent: codex` identity.
  Do not maintain another tool-name list or infer identity from a target's folder.
- Automatic routing selects the exact agent variant: Claude receives Claude's file,
  Codex receives Codex's file, and an unrelated target receives neither by inference.
  Shared destination paths, such as Codex and `universal`, do not prove agent identity.
  Reuse existing path-overlap/conflict diagnostics: targets sharing a directory
  cannot be promised isolation when their selected variants differ.
- Routing precedence is source-local target override > explicit frontmatter targets >
  recognized variant-agent inference > existing `inferSkillTargetsFromRelPath`.
  Generic host-style path inference cannot override a recognized variant's agent.
  An empty source-local override still means all targets. Existing explicit target
  matching stays compatible.
- `.skillignore`, disabled state, and per-target include/exclude filters still apply
  to physical entries. An include cannot resurrect an ignored file; an excluded
  matching variant does not cause selection of another agent's variant.
- Missing matches, unknown agent directories, and ambiguous matches never cause a
  sibling or lexicographically first variant to be chosen. Report unresolved
  recognized groups; do not invent a fallback to an agent named `shared` or `default`.
  Unrecognized paths retain ordinary discovery behavior rather than guessed routing.

Use `SelectTargetSkills` and existing target naming/resolution as the common selection
boundary for CLI/server sync, previews, status, diff, doctor, analyze, `skill move`,
and target-scoped skillserve catalogs. A shared result should explain selected,
filtered, missing, and ambiguous variants consistently.
Repository grouping belongs in discovery metadata, not in a status-only deduplicator.

### Counts, compatibility, and sync modes

Source and repository summaries distinguish logical skills from physical variants:
two logical skills with Claude and Codex copies are `2 skills, 4 variants`.
Sync results continue to count actual links, copies, updates, skips, and errors per
target; grouping must not reduce those totals or hide a failed variant operation.
Physical operation/catalog IDs remain stable. Grouping alone must not reduce analyze
or context-cost totals, which measure the physical files actually exposed to a target.
Ignored variants follow existing enabled/ignored reporting rather than inflating
the active logical count. A group with no enabled variant contributes no active skill.

Retain physical relative paths and flat names for existing filters, overrides,
uninstall operations, and destination naming. Do not silently rename installed
entries or broaden existing filters. Decide any future logical-name filter syntax
and migration separately, with compatibility tests.

In merge/copy mode, a target with no matching agent identity, including a universal-only
configuration, receives no agent-specific variant automatically. An explicit per-variant
target override can select the desired copy; an empty override preserves that variant's
old all-target behavior. The chosen activation mechanism must disclose this compatibility
effect and override options before enabling recognition. This proposal does not choose
new CLI/config fields; any opt-in surface remains part of the maintainer's activation decision.

Per-skill routing applies to merge and copy modes. Full-directory symlink mode
exposes the complete source tree and cannot enforce variant selection or ignores;
status and doctor must explain that limitation and recommend merge or copy where
selective exposure is needed. Do not claim logical counts describe runtime exposure.
No new content rewrite is introduced. Existing prefixed copy naming may still
rewrite the copied `name:` field; source bytes and agent-specific instructions stay intact.

### Acceptance cases for a future implementation

Every case assumes the maintainer-approved recognition mechanism is enabled for that
tracked source; no automatic upgrade activation is asserted, and recognition/activation
remain maintainer decisions.

| Case | Expected result |
|---|---|
| One name, Claude/Codex files | One logical skill, two variants; exact file routed to each agent |
| Two names, Claude/Codex files | Two logical skills, four variants; two operations per target |
| Different contents | Source bytes unchanged; merge/flat or standard copy preserves selected file bytes |
| Prefixed copy naming | Only established naming transformation; agent-specific instructions preserved |
| Same basename elsewhere | Distinct repositories, parents, and full nested suffixes remain separate |
| Repository installed with `--into` | Actual tracked root scopes grouping and existing override keys |
| `--into codex/skills`: `_repo/internal/skills/claude/fix` | Recognized Claude variant wins over outer Codex host inference unless explicitly overridden |
| Root/repository ignores and disabled variant | Existing precedence; no ignored variant selected or substituted |
| Per-target includes/excludes | Legacy physical flat-name patterns retain their selection semantics |
| Frontmatter and source-local overrides | Explicit routing wins; empty override retains all-target meaning |
| Canonical alias and account target | Same exact agent variant selected using registry/`Agent` identity |
| Shared target directory | No inferred permission for another tool solely because paths coincide |
| Universal-only or unmatched target, merge/copy | No automatic variant; explicit target override selects a copy, empty override preserves all-target behavior |
| Missing, unknown, shared, or ambiguous variant | No guessed substitute; applicable diagnostic or ordinary discovery |
| CLI/server/status/diff/doctor/analyze/`skill move`/target-scoped skillserve | Same selected physical entries and reasons; physical IDs and context-cost totals stay accurate |
| Merge/copy failure | Actual work/error counts remain independent of logical source counts |
| Full-directory symlink | Limitation visible; no promise that variant routing restricts exposed files |

## Alternatives Considered

- Keep manual filters/overrides: useful today, but neither logical grouping nor automatic routing.
- Deduplicate by basename or content: merges unrelated skills or misses intentionally different files.
- Convert one canonical file for every agent: a separate feature; the producer already supplies variants.
- Use sparse checkout: an optional later optimization, unnecessary for correct identity and routing.

## Scope

- [ ] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [x] Large (10+ files, 500+ lines): provisional implementation and tests estimate.

This PR adds one proposal document only (under 200 lines); eventual scope/size requires
maintainer agreement. The estimate covers discovery, shared selection, count outputs,
tests, and docs. Public API/schema changes also need maintainer agreement after review.
The eventual implementation updates affected public docs (README/website/wiki) and any
JSON schemas whose contracts actually change, documenting logical/variant fields compatibly.
Sparse checkout, content conversion, and a new naming/filter migration are outside this proposal's first implementation.

## Open Questions

1. Should grouping require producer/user opt-in, recognize `skills/<agent>/...` automatically,
   or support both? What evidence safely identifies a group when only one variant remains?
2. How should logical names and variant counts appear in existing CLI/JSON/UI output while
   retaining physical names, filters, override keys, and installed entries for compatibility?
3. Should a producer-declared shared variant be supported later, and how would explicit
   precedence resolve shared/specific overlap? This proposal assumes no inferred fallback.
