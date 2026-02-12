---
sidebar_position: 5
---

# search

Discover and install skills from GitHub repositories.

## Quick Start

```bash
skillshare search vercel       # Search by keyword
skillshare search              # Browse popular skills
```

This searches GitHub for repositories containing `SKILL.md` files that match your query.

## Browse Mode

When no query is provided, `search` browses popular skills on GitHub:

```bash
skillshare search              # Browse popular skills
skillshare search --list       # List popular skills
```

This uses `filename:SKILL.md` as the GitHub query and sorts results by star count, showing the most popular skill repositories first.

## How It Works

```
skillshare search [query]
        │
        ▼
GitHub Code Search API (filename:SKILL.md + query)
        │
        ▼
Fetch star counts for each repository
        │
        ▼
Sort by stars (most popular first)
        │
        ▼
Interactive selector → Install selected skill
```

## Preview

<p align="center">
  <img src="/img/search-demo.png" alt="search demo" width="720" />
</p>

**Controls:**
- `↑` `↓` — Navigate results
- `Enter` — Install selected skill
- `Ctrl+C` — Cancel and exit
- Type to filter results

After installing, you can search again or press `Enter` to quit.

## Options

| Flag | Description |
|------|-------------|
| `--project`, `-p` | Install to project-level config (`.skillshare/`) |
| `--global`, `-g` | Install to global config (`~/.config/skillshare`) |
| `--hub URL` | Search from a private hub index (local path or HTTP URL) |
| `--list`, `-l` | List results only, no install prompt |
| `--json` | Output as JSON (for scripting) |
| `--limit N`, `-n N` | Maximum results (default: 20, max: 100) |
| `--help`, `-h` | Show help |

:::tip Auto-detection
If neither `--project` nor `--global` is specified, skillshare auto-detects: if `.skillshare/config.yaml` exists in the current directory, it defaults to project mode; otherwise global mode.
:::

## Examples

### Browse Popular

```bash
skillshare search              # Browse popular skills (no query)
```

### Basic Search

```bash
skillshare search pdf           # Interactive search and install
skillshare search "code review" # Multi-word search
```

### List Mode

```bash
skillshare search commit --list
```

Output:
```
  1.  fix                      facebook/react/.claude/skills/fix        ★ 242.7k
      Use when you have lint errors, formatting issues...
  2.  verify                   facebook/react/.claude/skills/verify     ★ 242.7k
      Use when you want to validate changes before committing...
  3.  commit-helper            cockroachdb/cockroach/.claude/skills/commit-helper ★ 31.8k
      Help create git commits and PRs with properly formatted messages...
```

### JSON Output

```bash
skillshare search react --json --limit 5
```

```json
[
  {
    "Name": "react-patterns",
    "Description": "React and Next.js performance optimization...",
    "Source": "facebook/react/.claude/skills/react-patterns",
    "Stars": 242700,
    "Owner": "facebook",
    "Repo": "react",
    "Path": ".claude/skills/react-patterns"
  }
]
```

### Project Mode

```bash
skillshare search pdf -p           # Search and install to project
skillshare search react --project  # Same thing, long flag
```

Installed skills go to `.skillshare/skills/` and the project config is updated automatically. If the project hasn't been initialized yet, skillshare will run `init -p` first.

### Limit Results

```bash
skillshare search frontend -n 5   # Show only top 5 results
```

## Authentication

GitHub Code Search API requires authentication. skillshare automatically detects your credentials:

1. **GitHub CLI** (recommended) — If you're logged in with `gh`:
   ```bash
   gh auth login
   ```

2. **Environment variable** — Set `GITHUB_TOKEN` or `GH_TOKEN`:
   ```bash
   export GITHUB_TOKEN=ghp_your_token_here
   ```

### Creating a Token

If you don't use `gh` CLI:

1. Go to [GitHub Settings → Tokens](https://github.com/settings/tokens)
2. Generate new token (classic)
3. No scopes needed for public repos
4. Set the token:
   ```bash
   export GITHUB_TOKEN=ghp_your_token_here
   ```

## How Results are Ranked

1. **Search** — GitHub Code Search finds `SKILL.md` files matching your query
2. **Filter** — Removes forked repositories (duplicates)
3. **Fetch Stars** — Gets star count for each unique repository
4. **Sort** — Orders by stars (most popular first)
5. **Limit** — Returns top N results

This ensures high-quality, popular skills appear first.

## Private Index Search

Search from a private hub index instead of GitHub:

```bash
# Local file
skillshare search react --hub ./skillshare-hub.json

# HTTP URL
skillshare search react --hub https://internal.corp/skills/skillshare-hub.json

# Browse all skills (empty query)
skillshare search --hub ./skillshare-hub.json --json

# Equals syntax also works
skillshare search react --hub=./skillshare-hub.json
```

Build an index with [`hub index`](./hub.md):

```bash
skillshare hub index                    # Generate skillshare-hub.json
skillshare search --hub ./skillshare-hub.json  # Search it
```

For more details, see the [Hub Index Guide](../guides/hub-index.md).

## Tips

### Find Official Skills

Search for well-known organizations:
```bash
skillshare search anthropic    # Anthropic's skills
skillshare search facebook     # Meta/Facebook skills
skillshare search vercel       # Vercel's skills
```

### Find Specific Functionality

Search by what you want to do:
```bash
skillshare search "pull request"
skillshare search deployment
skillshare search testing
skillshare search database
```

### Continuous Search

In interactive mode, after installing a skill (or canceling), you can search again without restarting:

```
? Search again (or press Enter to quit): react
```

## Troubleshooting

### "GitHub Code Search API requires authentication"

Run `gh auth login` or set `GITHUB_TOKEN`. See [Authentication](#authentication).

### "GitHub API rate limit exceeded"

- Authenticated users: 30 requests/minute for Code Search
- Wait a minute and try again
- Use `--limit` to reduce API calls

### New Repository Not Found

GitHub indexes new repositories with a delay (hours to days). If your repo isn't found:
- Install directly: `skillshare install owner/repo/path/to/skill`
- Wait for GitHub to index the repository

### Results Don't Match Query

GitHub Code Search matches content inside `SKILL.md` files. A skill mentioning "vercel" in its description will appear in vercel searches, even if the skill isn't specifically about Vercel.

Use `--list` to review results before installing.
