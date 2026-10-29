# Agent-specific variants of a tracked skill

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

## Implemented behavior

This PR implements logical counting and exact-agent routing. Sparse checkout remains deferred.

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

Recognition is automatic for repository-relative `skills/<known-agent>/<suffix>`
paths only when the same complete suffix occurs under at least two distinct canonical
agents beneath the same repository-relative `skills/` parent. Aliases of one agent
do not provide that second identity. A lone `skills/agents/planner` or
`skills/continue/helper` category remains ordinary, even when its name is a registry
alias. The outer recognized component owns the complete suffix, unless the agent-named
directory is itself a skill; its nested skills remain ordinary skills. Ordinary skills
and unknown layouts keep existing discovery behavior.

### Routing and precedence

Extend the existing shared discovery and target-selection paths rather than adding
separate variant walkers or per-command routing rules:

- Discovery retains physical entries and attaches logical identity and agent identity.
- Resolve an agent through the configuration's target registry and canonical aliases.
  Account targets such as `codex-work` use their configured `Agent: codex` identity.
  Do not maintain another tool-name list or infer identity from a target's folder.
- Automatic routing selects the exact agent variant: Claude receives Claude's file,
  Codex receives Codex's file, and an unrelated target receives neither by inference.
  Compatible targets sharing a destination folder use the union of their configured
  agent identities for automatic selection. Thus Codex and `universal` sharing
  `~/.agents/skills` both select the Codex variants and neither prunes them. This
  shared folder exposes the union to every reader; it does not isolate agents.
  Existing overlap/conflict diagnostics still reject incompatible settings.
- Routing precedence is source-local target override > explicit frontmatter targets >
  recognized variant-agent inference > existing `inferSkillTargetsFromRelPath`.
  Generic host-style path inference cannot override a recognized variant's agent.
  An empty source-local override still means all targets. Existing explicit target
  matching stays compatible.
- `.skillignore`, disabled state, and per-target include/exclude filters still apply
  to physical entries. An include cannot resurrect an ignored file; an excluded
  matching variant does not cause selection of another agent's variant.
- Missing matches, unknown agent directories, and ambiguous matches never cause a
  sibling or lexicographically first variant to be chosen. Report ambiguous
  variants in naming/sync diagnostics; do not invent a fallback to an agent named
  `shared` or `default`.
  Unrecognized paths retain ordinary discovery behavior rather than guessed routing.

Use `SelectTargetSkills` and existing target naming/resolution as the common selection
boundary for CLI/server sync, previews, status, diff, doctor, analyze, `skill move`,
and target-scoped skillserve catalogs. Selection preserves the physical entries;
missing matches select nothing and ambiguous automatic matches are skipped with
a diagnostic.
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
old all-target behavior. Recognition is automatic on upgrade. Sync removes old managed
entries whose inferred agent does not match the target, reports the automatic-routing
reason through CLI/server warnings (including dry-run), and names target overrides as
the recovery option. Source files remain intact; no opt-in configuration is added.

Per-skill routing applies to merge and copy modes. Full-directory symlink mode
exposes the complete source tree and cannot enforce variant selection or ignores;
status and doctor must explain that limitation and recommend merge or copy where
selective exposure is needed. Do not claim logical counts describe runtime exposure.
No new content rewrite is introduced. Existing prefixed copy naming may still
rewrite the copied `name:` field; source bytes and agent-specific instructions stay intact.

### Acceptance cases

Every case uses automatic recognition for the bounded tracked-source layout.

| Case | Expected result |
|---|---|
| One name, Claude/Codex files | One logical skill, two variants; exact file routed to each agent |
| Two names, Claude/Codex files | Two logical skills, four variants; two operations per target |
| Different contents | Source bytes unchanged; merge/flat or standard copy preserves selected file bytes |
| Prefixed copy naming | Only established naming transformation; agent-specific instructions preserved |
| Same basename elsewhere | Distinct repositories, parents, and full nested suffixes remain separate |
| Repository installed with `--into` | Actual tracked root scopes grouping and existing override keys |
| `--into codex/skills`: `_repo/internal/skills/claude/fix` | Recognized Claude variant wins over outer Codex host inference unless explicitly overridden |
| Single agent-like category or aliases of one agent only | Ordinary skills; no inferred variant routing or logical grouping |
| Root/repository ignores and disabled variant | Existing precedence; no ignored variant selected or substituted |
| Per-target includes/excludes | Legacy physical flat-name patterns retain their selection semantics |
| Frontmatter and source-local overrides | Explicit routing wins; empty override retains all-target meaning |
| Canonical alias and account target | Same exact agent variant selected using registry/`Agent` identity |
| Shared target directory | Compatible targets select the union of agent identities without pruning one another; incompatible settings retain overlap diagnostics |
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

The implementation extends shared discovery and selection, source/repository count
outputs, tests, and public docs. Status JSON adds `variant_count`, overview JSON adds
`variantCount`, and targets JSON adds `sourceVariantCount`; existing skill counts in
these source summaries become logical counts. Resource rows and operation/context
totals remain physical. No new persisted configuration or schema version is added.
Sparse checkout, content conversion, and a new naming/filter migration are outside this proposal's first implementation.

## Deferred

Sparse checkout, producer-declared shared/default fallbacks, and a grouped resource-list
UI remain outside this implementation. Physical resource rows retain their paths so
install/uninstall, filters, and override keys remain compatible. Existing empty
frontmatter list behavior is preserved; source-local empty overrides mean all targets.
