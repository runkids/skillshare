---
sidebar_position: 4
---

# analyze

Analyze context window usage and skill quality for each target's skills.

```bash
skillshare analyze                    # Interactive TUI (default)
skillshare analyze claude             # Details for a single target
skillshare analyze --verbose          # Top 10 largest descriptions
skillshare analyze --json             # Machine-readable output
skillshare analyze -p                 # Project mode
```

## When to Use

### Optimize Context Budget

Identify which skills consume the most context window tokens:

```bash
skillshare analyze           # Browse all targets interactively
```

### Compare Across Targets

See how context usage differs between targets (e.g., Claude vs Cursor):

```bash
skillshare analyze           # Tab to switch targets in TUI
```

### Check Skill Quality

Find skills with missing fields, short descriptions, or no trigger phrases:

```bash
skillshare analyze           # Lint icons (✗/⚠) appear in TUI
```

### CI/Scripting

Get machine-readable context metrics and lint results:

```bash
skillshare analyze --json | jq '.targets[].always_loaded.estimated_tokens'
skillshare analyze --json | jq '.targets[].skills[] | select(.lint_issues | length > 0)'
```

## What It Does

`analyze` calculates two layers of context cost for each skill:

1. **Always loaded** — `name + description` from SKILL.md frontmatter (loaded into context on every request for skill matching)
2. **On-demand** — Skill body after frontmatter (loaded only when the skill is triggered)

Token estimates count about 4 characters per token for ASCII text and 1 token per character for CJK and other wide scripts.

Each target is measured as the tool sees it:

- **merge / copy mode** — source skills that pass the target's include/exclude and `targets:` filters, minus anything disabled in `.skillignore`, plus skills that already live in the target folder without coming from the source (flagged `local` in JSON). On a Claude target, a skill Claude Code syncs from claude.ai into `synced/<id>/skill-creator` reads `skill-creator (claude.ai)` in text output and the dashboard. JSON keeps the full `name`.
- **symlink mode** — the whole source folder, including `.skillignore`'d skills (flagged `disabled` in JSON), because the symlink exposes them anyway.

### Skill Quality Lint

In addition to token analysis, `analyze` runs a built-in lint engine against every skill. Lint rules check SKILL.md structure and description quality, surfacing issues directly in the TUI and JSON output.

| Rule | Severity | What it checks |
|------|----------|----------------|
| `missing-name` | error | `name` field is empty or missing |
| `missing-description` | error | `description` field is empty or missing |
| `empty-body` | error | Skill body (after frontmatter) is empty |
| `description-too-short` | warning | Description under 50 characters |
| `description-too-long` | warning | Description exceeds 1024-character target limit |
| `description-near-limit` | warning | Description between 900–1024 characters |
| `body-too-long` | warning | Skill body (after frontmatter) is more than 500 lines; move detail into reference files |
| `no-trigger-phrase` | warning | Description lacks trigger phrases (e.g. "Use when…") |

In the TUI, skills with lint issues show ✗ (error) or ⚠ (warning) icons next to their name. The detail panel includes a **Quality** section listing all findings.

## Interactive TUI

On a TTY, `analyze` opens an interactive view of one target at a time: skills sorted by token cost on the left, and the token breakdown, lint issues and description of the selected skill on the right. Opening a skill shows its files as written, front matter included, so a lint issue can be checked against the text. The keys are listed at the bottom of the screen. Use `--no-tui` for plain text. The TUI does not show MCP tool definitions; use `--no-tui` or `--json` for those.

### Color Coding

Token consumption levels use dynamic percentile thresholds per target:

| Color | Meaning |
|-------|---------|
| 🔴 Red | P75+ (top 25% consumers) |
| 🟡 Yellow | P25–P75 (middle 50%) |
| 🟢 Green | Below P25 (lowest 25%) |

## Example Output

### Default (--no-tui)

```
claude · 7 skills
  Always loaded  ~362 tokens
  On-demand max  ~22 tokens

  Add -v for the 10 largest descriptions
```

### MCP tool definitions {#mcp-tool-definitions}

A run over the whole source (no target argument and no `--filter`) also adds the MCP servers' tool definitions, counted per MCP target (the `mcp.targets` Agents such as `claude`, `codex` or `cursor`, which are a separate list from the skill targets):

```
MCP tool definitions (upper bound)  claude ~6.8K · codex ~5.7K

! claude: MCP tool definitions are ~6,750 tokens (budget: 5,000)
   Largest: github ~3,276, playwright ~1,932, sentry ~1,092
  1 MCP server not counted yet; run skillshare mcp check --live
```

The counts come from [`mcp check --live`](/docs/reference/commands/mcp#probe-servers-live), which estimates each server's tool names, descriptions and input schemas; `analyze` starts no server. A server no live check has counted is listed as not counted, never as 0. When none has been counted, `analyze` prints `MCP tool definitions have not been counted yet; run skillshare mcp check --live`. Each place is its own group: the global list, and each `mcp.projects` root, whose group also holds the global servers that project does not turn off, since the Agent loads them there too. Tool policies are not applied, so a total is an upper bound. The warning uses [`context_budget.warn_mcp_tool_tokens`](/docs/reference/targets/configuration#context_budget).

### Verbose

```
skillshare analyze --verbose

claude · 7 skills
  Always loaded  ~362 tokens
  On-demand max  ~22 tokens

  Largest descriptions
    my-big-skill   ~180 tokens
    another-skill  ~120 tokens
    …
```

### Single Target

Passing a target name automatically enables verbose output:

```bash
skillshare analyze claude
```

### Filter by Group

```bash
# See total token cost of all frontend skills
skillshare analyze claude --json --filter frontend

# Pre-populate the search box in TUI
skillshare analyze --filter marketing
```

## Options

| Flag | Description |
|------|-------------|
| `[target]` | Show details for a single target (auto-enables verbose) |
| `--verbose`, `-v` | Show top 10 largest descriptions per target |
| `--no-tui` | Disable interactive TUI, print plain text |
| `--project`, `-p` | Analyze project-level skills (`.skillshare/`) |
| `--global`, `-g` | Analyze global skills (`~/.config/skillshare`) |
| `--filter <text>` | Filter skills by name/path substring |
| `--json` | Output as JSON (for scripting/CI) |
| `--help`, `-h` | Show help |

:::tip Auto-detection
If neither `--project` nor `--global` is specified, skillshare auto-detects: if `.skillshare/config.yaml` exists in the current directory, it defaults to project mode; otherwise global mode.
:::

## JSON Output

```bash
skillshare analyze --json
```

```json
{
  "targets": [
    {
      "name": "claude",
      "skill_count": 7,
      "always_loaded": {
        "chars": 1448,
        "estimated_tokens": 362
      },
      "on_demand_max": {
        "chars": 88,
        "estimated_tokens": 22
      },
      "skills": [
        {
          "name": "my-skill",
          "description_chars": 180,
          "description_tokens": 45,
          "body_chars": 400,
          "body_tokens": 100,
          "lint_issues": [
            {
              "rule": "no-trigger-phrase",
              "severity": "warning",
              "category": "format",
              "message": "Description lacks trigger phrases (e.g. 'Use when...'); agents may not know when to invoke this skill"
            }
          ]
        }
      ]
    }
  ]
}
```

Skills with no lint issues omit the `lint_issues` field.

A whole-source run also adds an `mcp` object, omitted when no MCP target receives a server:

```json
"mcp": {
  "budget": 30000,
  "groups": [
    {
      "target": "claude", "agent": "claude", "tokens": 6750, "measured": 4,
      "servers": [
        { "name": "github", "tools": 26, "tokens": 3276, "checked": "2026-11-25T08:30:00Z" }
      ]
    }
  ]
}
```

`root` (the `mcp.projects` folder) is set on a project group. A server has `project` when it comes from a project, and `checked` only when a live check counted it; without it, `tokens` is not a count. `tokens` of a group adds up the counted servers, and `measured` is how many that is.

## Project Mode

```bash
skillshare analyze -p                  # Interactive TUI for project skills
skillshare analyze -p --verbose        # Verbose text output
skillshare analyze -p claude           # Single target details
skillshare analyze -p --json           # JSON output
```

## Filtering

Use `--filter` to narrow results to a subset of skills. The filter performs a case-insensitive
substring match on the skill's relative path (which includes the group directory).

For example, if you installed skills with `--into frontend`:
- `--filter frontend` matches all skills in the `frontend/` group
- `--filter react` matches any skill with "react" in its path

In TUI mode, `--filter` pre-populates the filter input. You can also use the `/` key to start filtering interactively.

In JSON mode, the output includes a `filtered_summary` with aggregated token counts:

```json
{
  "filter": "frontend",
  "matched_count": 5,
  "total_count": 50,
  "filtered_summary": {
    "always_loaded": { "chars": 2400, "tokens": 600 },
    "on_demand": { "chars": 8000, "tokens": 2000 },
    "total": { "chars": 10400, "tokens": 2600 }
  },
  "skills": [...]
}
```

The Web UI also shows a dynamic token summary bar when search or filter is active.

## Budget Warnings

When `context_budget` thresholds are configured, `analyze` shows a warning if any target exceeds the budget (`warn_mcp_tool_tokens` for MCP tool definitions). See [sync — Context Cost](/docs/reference/commands/sync#context-cost) for configuration details.

## See Also

- [list](/docs/reference/commands/list) — View installed skills
- [audit](/docs/reference/commands/audit) — Scan skills for security threats
- [tui](/docs/reference/commands/tui) — Toggle interactive TUI on/off
