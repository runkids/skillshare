---
sidebar_position: 5.5
---

# Organizing Skills with Folders

As your skill collection grows, organizing them into folders keeps things manageable — and skillshare handles the rest automatically.

## Why Organize?

A flat list of 20+ skills becomes hard to navigate:

```
~/.config/skillshare/skills/
├── accessibility/
├── ascii-box-check/
├── core-web-vitals/
├── frontend-design/
├── performance/
├── react-best-practices/
├── remotion/
├── seo/
├── skill-creator/
├── ui-skills/
├── vue-best-practices/
├── vue-debug-guides/
├── web-artifacts-builder/
└── ... 20+ more
```

With folders, you get logical grouping while skillshare auto-flattens for AI CLIs:

```
SOURCE (organized)                     TARGET (auto-flattened)
───────────────────────────────────    ──────────────────────────────────
~/.config/skillshare/skills/           ~/.claude/skills/
├── frontend/                          ├── frontend__frontend-design
│   ├── frontend-design/               ├── frontend__react__react-best-..
│   ├── react/                         ├── frontend__ui-skills
│   │   └── react-best-practices/      ├── frontend__vue__vue-best-prac..
│   ├── ui-skills/                     ├── frontend__vue__vue-debug-gui..
│   └── vue/                           ├── utils__ascii-box-check
│       ├── vue-best-practices/        ├── utils__remotion
│       ├── vue-debug-guides/          ├── utils__skill-creator
│       └── ...                        ├── web-dev__accessibility
├── utils/                             ├── web-dev__core-web-vitals
│   ├── ascii-box-check/               └── ...
│   ├── remotion/
│   └── skill-creator/
└── web-dev/
    ├── accessibility/
    ├── core-web-vitals/
    └── ...
```

![Source vs Target comparison](/img/organizing-skills-comparison.png)

:::tip Real-world example
See [runkids/my-skills](https://github.com/runkids/my-skills) for a complete organized skill collection using this pattern.
:::

---

## How Auto-Flattening Works

Skillshare converts folder paths to flat names using `__` (double underscore) as separator:

| Source path | Synced target name |
|---|---|
| `frontend/react/react-best-practices/` | `frontend__react__react-best-practices` |
| `utils/remotion/` | `utils__remotion` |
| `web-dev/accessibility/` | `web-dev__accessibility` |

**Key points:**
- Only directories containing `SKILL.md` are treated as skills
- Intermediate folders (like `frontend/` itself) are just organizational — they don't need `SKILL.md`
- `list` and `sync` discover nested skills at any depth
- `check` and `update` also work with nested skills

---

## Working with Nested Skills

### list

Shows auto-flattened names (what AI CLIs see):

```bash
$ skillshare list -g
  → frontend__vue__vue-best-practices     github.com/vuejs-ai/skills/...
  → utils__remotion                       github.com/remotion-dev/skills/...
  → web-dev__accessibility                github.com/addyosmani/web-quality-...
```

### check

Detects nested skills and shows relative paths:

```bash
$ skillshare check -g
Checking for updates
─────────────────────────────────────────
▸  Source  ~/.config/skillshare/skills
│
├─ Items  0 tracked repo(s), 15 skill(s)

  ✓ frontend/frontend-design            up to date
  ✓ frontend/react/react-best-practices up to date
  ✓ utils/remotion                      up to date
  ✓ web-dev/accessibility               up to date
```

### update

Supports both **full paths** and **short names**:

```bash
# Full relative path
skillshare update -g frontend/react/react-best-practices

# Short name (basename) — auto-resolved
skillshare update -g react-best-practices

# Update everything
skillshare update -g --all
```

When a short name matches multiple skills, skillshare asks you to be more specific:

```
'my-skill' matches multiple items:
  - frontend/my-skill
  - backend/my-skill
Please specify the full path
```

---

## Install Directly into Folders

Use `--into` to install a skill into a subdirectory in one step — no manual `mv` needed:

```bash
# Install into a category folder
skillshare install anthropics/skills -s pdf --into frontend
# → ~/.config/skillshare/skills/frontend/pdf/

# Multi-level nesting
skillshare install ~/my-skill --into frontend/react
# → ~/.config/skillshare/skills/frontend/react/my-skill/

# Works with --track too
skillshare install github.com/team/skills --track --into devops
# → ~/.config/skillshare/skills/devops/_skills/

# Works in project mode
skillshare install anthropics/skills -s pdf --into tools -p
# → .skillshare/skills/tools/pdf/
```

After `skillshare sync`, targets show auto-flattened names:
- `frontend/pdf/` → `frontend__pdf`
- `frontend/react/my-skill/` → `frontend__react__my-skill`
- `devops/_skills/frontend/ui/` → `devops___skills__frontend__ui`

:::tip
`--into` creates intermediate directories automatically. No need to `mkdir` first.
:::

---

## Suggested Folder Structures

### By domain

```
skills/
├── frontend/
│   ├── react/
│   ├── vue/
│   └── css/
├── backend/
│   ├── api-design/
│   └── database/
├── devops/
│   ├── docker/
│   └── ci-cd/
└── utils/
    ├── git-workflow/
    └── code-review/
```

### By tool ecosystem

```
skills/
├── vue/
│   ├── vue-best-practices/
│   ├── vue-debug-guides/
│   ├── vue-pinia-best-practices/
│   └── vue-router-best-practices/
├── react/
│   └── react-best-practices/
└── web/
    ├── accessibility/
    ├── performance/
    └── seo/
```

### Mixed: personal + tracked repos

```
skills/
├── frontend/              # Personal organized skills
│   └── vue/
├── utils/                 # Personal utilities
│   └── ascii-box-check/
├── _team-skills/          # Tracked repo (auto-updated)
│   ├── code-review/
│   └── deploy/
└── _org-standards/        # Another tracked repo
    └── security/
```

---

## Version Control Your Skills

Organizing skills in folders pairs naturally with git:

```bash
skillshare init --remote git@github.com:yourname/my-skills.git
skillshare push -m "organize skills into categories"
```

This gives you:
- **History** of skill changes across machines
- **Backup** via GitHub/GitLab
- **Sharing** — others can browse and fork your collection
- **Cross-machine sync** via `skillshare pull` (see [Cross-Machine Sync](./cross-machine-sync))

---

## Migrating from Flat to Folders

:::tip New installs
For new skills, use `--into` to install directly into the right folder — see [Install Directly into Folders](#install-directly-into-folders) above.
:::

If you already have a flat skill collection:

```bash
cd ~/.config/skillshare/skills

# Create category folders
mkdir -p frontend/react frontend/react utils web-dev

# Move skills into folders
mv react-best-practices frontend/react/
mv react-debug-guides frontend/react/
mv react-best-practices frontend/react/
mv remotion utils/
mv accessibility web-dev/

# Re-sync to update target symlinks
skillshare sync
```

After `sync`, targets are updated automatically — old flat symlinks are cleaned up and new flattened names are created.

---

## Related

- [Source & Targets](/docs/concepts/source-and-targets#organize-with-folders-auto-flattening) — Auto-flattening concept
- [Tracked Repositories](/docs/concepts/tracked-repositories#nested-skills--auto-flattening) — Nested skills in tracked repos
- [Best Practices](./best-practices) — Naming conventions
- [Cross-Machine Sync](./cross-machine-sync) — Sync across computers
