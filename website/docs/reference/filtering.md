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
| SKILL.md `targets` | Restricts to listed targets | Per skill frontmatter | YAML list | Sync (parsed at discovery) |
| Target include/exclude | Per target | `config.yaml` or CLI flags | Go [`filepath.Match`](https://pkg.go.dev/path/filepath#Match) glob | Sync |

:::note Sync mode caveat
All three layers only apply to **merge** and **copy** sync modes.
In **symlink** mode the entire source directory is linked as one unit — per-skill filtering has no effect.
:::

## Evaluation order and precedence

A skill must pass **all** layers to reach a target:

1. **`.skillignore`** — evaluated at discovery. Matching skills never enter the sync pipeline.
2. **Target include/exclude** — evaluated at sync (`FilterSkills`). Skills are discovered but skipped for non-matching targets.
3. **SKILL.md `targets`** — evaluated at sync (`FilterSkillsByTarget`). Skills are restricted to their declared targets.

## .skillignore

**Locations:**
- Source root: `~/.config/skillshare/skills/.skillignore` — applies to all skills
- Tracked repo root: `_team-repo/.skillignore` — applies only within that repo

**Syntax:** Full [gitignore](https://git-scm.com/docs/gitignore) — `*` (single segment), `**` (any depth), `?`, `[abc]`, `!pattern` (negation), `/pattern` (anchored), `pattern/` (directory-only).

**`.skillignore.local`:** Place alongside `.skillignore`. Patterns are appended after the base file — last matching rule wins. Use `!pattern` to un-ignore. Don't commit this file.

**CLI visibility:**

| Command | Output |
|---------|--------|
| `skillshare sync` | Count + skill names |
| `skillshare status --json` | `source.skillignore` object with patterns and ignored list |
| `skillshare doctor` | Pattern count and ignored count |

📖 [File structure reference](/docs/reference/appendix/file-structure#skillignore-optional)

## SKILL.md targets field

**Format:** Top-level or nested under `metadata`:

```yaml
# Either format works
targets: [claude, cursor]

# Or nested
metadata:
  targets: [claude, cursor]
```

**Behavior:** Whitelist — the skill only syncs to the listed targets. Omitting the field means sync to all targets.

**Aliases:** Target names support aliases. `claude` matches a target configured as `claude-code`. See [Supported Targets](/docs/reference/targets/supported-targets).

📖 [Skill format — targets field](/docs/understand/skill-format#targets)

## Target include/exclude filters

**Set via CLI:**

```bash
skillshare target claude --add-include "team-*"
skillshare target cursor --add-exclude "legacy-*"
skillshare target claude --remove-include "team-*"
```

**Stored in:** `config.yaml` under `targets.<name>.include` / `targets.<name>.exclude`.

**Syntax:** Go [`filepath.Match`](https://pkg.go.dev/path/filepath#Match) glob patterns matched against the flat skill name (e.g., `_team__frontend__ui`).

| Supported | Not supported |
|-----------|--------------|
| `*` (any chars) | `**` (recursive) |
| `?` (single char) | `{a,b}` (brace expansion) |
| `[abc]` (char class) | |

**Precedence:** When both `include` and `exclude` are set, `include` is applied first, then `exclude`. A skill matching both is excluded.

**Visual editor:** `skillshare ui` → Targets page → "Customize filters" button.

📖 [Target command](/docs/reference/commands/target#target-filters-includeexclude) · [Filter behavior examples](/docs/reference/commands/sync#filter-behavior-examples) · [Configuration](/docs/reference/targets/configuration#include--exclude-target-filters)

## See also

- [Filtering Skills](/docs/how-to/daily-tasks/filtering-skills) — scenario-driven how-to guide
