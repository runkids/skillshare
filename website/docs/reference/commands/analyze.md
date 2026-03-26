---
sidebar_position: 4
---

# analyze

Analyze context window usage for each target's skills.

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

### CI/Scripting

Get machine-readable context metrics:

```bash
skillshare analyze --json | jq '.targets[].always_loaded.estimated_tokens'
```

## What It Does

`analyze` calculates two layers of context cost for each skill:

1. **Always loaded** — `name + description` from SKILL.md frontmatter (loaded into context on every request for skill matching)
2. **On-demand** — Skill body after frontmatter (loaded only when the skill is triggered)

Token estimates use `chars / 4` as an approximation.

## Interactive TUI

By default, `analyze` launches an interactive TUI with:

- **Left panel** — Skill list sorted by token cost, with color-coded dots (red/yellow/green by percentile)
- **Right panel** — Detail view: token breakdown, path, tracked status, description preview
- **Bottom bar** — Target selector (Tab/Shift+Tab to switch) + token totals + estimation formula

### TUI Controls

| Key | Action |
|-----|--------|
| `↑`/`↓` | Navigate skill list |
| `←`/`→` | Page up/down |
| `Tab` / `Shift+Tab` | Switch target |
| `/` | Filter skills by name |
| `s` | Cycle sort: tokens↓ → tokens↑ → name A→Z → name Z→A |
| `Ctrl+d` / `Ctrl+u` | Scroll detail panel |
| `q` | Quit |

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
Context Analysis (global)
ℹ claude (7 skills)
  Always loaded:  ~362 tokens
  On-demand max:  ~22 tokens
```

### Verbose

```
skillshare analyze --verbose

Context Analysis (global)
ℹ claude (7 skills)
  Always loaded:  ~362 tokens
  On-demand max:  ~22 tokens

  Largest descriptions:
  my-big-skill                     ~180 tokens
  another-skill                    ~120 tokens
  ...
```

### Single Target

Passing a target name automatically enables verbose output:

```bash
skillshare analyze claude
```

## Options

| Flag | Description |
|------|-------------|
| `[target]` | Show details for a single target (auto-enables verbose) |
| `--verbose`, `-v` | Show top 10 largest descriptions per target |
| `--no-tui` | Disable interactive TUI, print plain text |
| `--project`, `-p` | Analyze project-level skills (`.skillshare/`) |
| `--global`, `-g` | Analyze global skills (`~/.config/skillshare`) |
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
          "body_tokens": 100
        }
      ]
    }
  ]
}
```

## Project Mode

```bash
skillshare analyze -p                  # Interactive TUI for project skills
skillshare analyze -p --verbose        # Verbose text output
skillshare analyze -p claude           # Single target details
skillshare analyze -p --json           # JSON output
```

## See Also

- [list](/docs/reference/commands/list) — View installed skills
- [audit](/docs/reference/commands/audit) — Scan skills for security threats
- [tui](/docs/reference/commands/tui) — Toggle interactive TUI on/off
