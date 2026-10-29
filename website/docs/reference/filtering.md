---
sidebar_position: 3
---

# Filtering Reference

Complete specification of the three filtering layers that control which skills reach which targets.

:::tip Looking for quick guidance?
See [Filtering Skills](/docs/how-to/daily-tasks/filtering-skills) for a scenario-driven guide.
:::

## Overview

| Layer | Scope | Where to set | Syntax | Evaluated at |
|-------|-------|-------------|--------|-------------|
| `.skillignore` | Hides from all targets | Source dir or tracked repo root | [gitignore](https://git-scm.com/docs/gitignore) | Discovery |
| SKILL.md `metadata.targets` | Restricts skills to listed targets | Per skill frontmatter | YAML list | Sync (parsed at discovery) |
| Agent `targets` | Restricts an agent to listed targets | Per agent frontmatter | YAML list | Sync (parsed at discovery) |
| Target include/exclude | Per target, per resource | `config.yaml` or CLI flags | Go [`filepath.Match`](https://pkg.go.dev/path/filepath#Match) glob | Sync |

:::note Sync mode caveat
All three layers only apply to **merge** and **copy** sync modes.
In **symlink** mode the entire source directory is linked as one unit — per-skill filtering has no effect.
:::

## Evaluation order and precedence

A skill must pass **all** layers to reach a target:

1. **`.skillignore`** — evaluated at discovery. Matching skills never enter the sync pipeline.
2. **Target include/exclude** — evaluated at sync (`FilterSkills`). Skills are discovered but skipped for non-matching targets.
3. **SKILL.md `metadata.targets`** — evaluated at sync (`FilterSkillsByTarget`). Skills are restricted to their declared targets.

## .skillignore

**Locations:**
- Source root: `~/.config/skillshare/skills/.skillignore` — applies to all skills
- Tracked repo root: `_team-repo/.skillignore` — applies only within that repo

**Syntax:** Full [gitignore](https://git-scm.com/docs/gitignore) — `*` (single segment), `**` (any depth), `?`, `[abc]`, `!pattern` (negation), `/pattern` (anchored), `pattern/` (directory-only).

**`.skillignore.local`:** Place alongside `.skillignore`. Patterns are appended after the base file — last matching rule wins. Use `!pattern` to un-ignore. Don't commit this file. A pattern without `/` matches that name at any depth, so `!feature-radar` also re-enables `feature-radar/feature-radar`; write `!/feature-radar` to match only the top-level folder. The dashboard shows this file's rules in **Settings → Files** and reports a toggle that it overrides as failed.

**CLI visibility:**

| Command | Output |
|---------|--------|
| `skillshare sync` | Count + skill names |
| `skillshare status --json` | `source.skillignore` object with patterns and ignored list |
| `skillshare doctor` | Pattern count and ignored count |

📖 [File structure reference](/docs/reference/appendix/file-structure#skillignore-optional)

## SKILL.md targets field {#skillmd-targets-field}

**Format:** Top-level or nested under `metadata`:

```yaml
# Preferred
metadata:
  targets: [claude, cursor]

# Legacy fallback
targets: [claude, cursor]
```

**Behavior:** Whitelist — the skill only syncs to the listed targets. Omitting the field means sync to all targets. If both `metadata.targets` and top-level `targets` are present, `metadata.targets` wins.

**Tracked repos:** targets set in the dashboard for a skill inside a tracked repo are stored in the source's `.metadata.json` instead of its SKILL.md, so the clone stays clean and `update` keeps working. That setting takes precedence over the skill's `metadata.targets`.

### Agent-specific variants in tracked repositories

A tracked repository can ship separate instructions for each agent:

```text
_team/internal/skills/claude/review/SKILL.md
_team/internal/skills/codex/review/SKILL.md
```

Skillshare recognizes repository-relative `skills/<known-agent>/<skill-path>`
layouts automatically, including canonical aliases such as `claude-code` and
repositories installed under `--into`. Merge and copy sync select the matching
agent's file. A target with `agent: codex`, such as a second Codex account, gets
the Codex variant. A shared skills directory does not establish agent identity:
`universal` does not automatically receive the Codex variant.

The repository, complete path before the agent component, and complete path after
it identify one logical skill. Different repositories, prefixes, or suffixes
stay separate. A single recognized variant still uses agent routing. Unknown
agent directories and layouts outside tracked repositories keep ordinary behavior;
`shared` and `default` have no inferred fallback meaning.

Source-local target overrides take precedence over nonempty frontmatter targets,
then recognized variant inference, then existing host-style path inference.
An empty source-local override means all targets. Explicit target declarations
keep their existing matching behavior. Ignores and include/exclude filters apply
to physical paths as before. Missing matches select nothing; duplicate canonical
aliases in one group are skipped with a sync warning until filters or overrides
select one.

`status` and dashboard source/repository totals count logical skills. Four files
for two skills appear as `2 skills, 4 variants` in text status. Status JSON adds
`variant_count` beside logical `skill_count`; overview JSON uses `variantCount`,
and the targets API exposes `sourceVariantCount`. Variant counts are physical
enabled file totals, including ordinary skills. Resource lists, operation IDs,
sync totals, and context-cost reports retain physical entries. Source files are
unchanged; existing prefixed copy naming can still rewrite the copy's `name:`.

Full-directory symlink mode exposes every source variant and cannot enforce
this routing. Use merge or copy for agent selection. Targets sharing a destination
directory also cannot keep different selections isolated; existing overlap
diagnostics apply. Sparse checkout is not supported by this recognition.

After upgrading, merge/copy sync removes previously managed variants whose agent
does not match the target. The CLI and server warn when automatic agent routing
removes those entries, including during dry-run. Set a source-local target override
for the desired variant to restore it; an empty override restores all-target routing.
The source files remain intact. The targets column shows effective routing, including
inference; it does not imply that the producer declared that field.

**Aliases:** Target names support aliases. `claude` matches a target configured as `claude-code`. See [Supported Targets](/docs/reference/targets/supported-targets).

📖 [Skill format — targets field](/docs/understand/skill-format#targets)

**Agents** support the same whitelist via a top-level `targets` list in the agent's frontmatter. Agents without the field sync to every agent-capable target. See [Agents — Agent File Format](/docs/understand/agents#agent-file-format).

## Target include/exclude filters {#target-includeexclude-filters}

**Set via CLI:**

```bash
# Skills
skillshare target claude --add-include "team-*"
skillshare target cursor --add-exclude "legacy-*"
skillshare target claude --remove-include "team-*"

# Agents
skillshare target claude --add-agent-include "team-*"
skillshare target claude --add-agent-exclude "draft-*"
skillshare target claude --remove-agent-include "team-*"
```

**Stored in:** `config.yaml` under `targets.<name>.include` / `targets.<name>.exclude` for skills, and `targets.<name>.agents.include` / `targets.<name>.agents.exclude` for agents.

**Syntax:** Go [`filepath.Match`](https://pkg.go.dev/path/filepath#Match) glob patterns matched against the flat resource name. Skills use flat skill names (e.g., `_team__frontend__ui`); agents use flat `.md` filenames.

| Supported | Not supported |
|-----------|--------------|
| `*` (any chars) | `**` (recursive) |
| `?` (single char) | `{a,b}` (brace expansion) |
| `[abc]` (char class) | |

**Precedence:** When both `include` and `exclude` are set, `include` is applied first, then `exclude`. A matching resource that hits both is excluded.

**Visual editor:** `skillshare ui` → Targets page → "Customize filters" button.

📖 [Target command](/docs/reference/commands/target#target-filters-includeexclude) · [Filter behavior examples](/docs/reference/commands/sync#filter-behavior-examples) · [Configuration](/docs/reference/targets/configuration#include--exclude-target-filters)

## See also

- [Filtering Skills](/docs/how-to/daily-tasks/filtering-skills) — scenario-driven how-to guide
