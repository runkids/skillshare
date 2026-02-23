---
sidebar_position: 2
---

# First Sync

Get your skills synced in 5 minutes.

## Prerequisites

- macOS, Linux, or Windows
- At least one AI CLI installed (Claude Code, Cursor, Codex, etc.)

---

## Step 1: Install skillshare

**Homebrew:**
```bash
brew install skillshare
```

:::note
All install methods include the web dashboard. `skillshare ui` automatically downloads UI assets on first launch — no extra setup needed.
:::

:::tip Updating
To update to the latest version, run `skillshare upgrade`. It auto-detects your install method (Homebrew, manual, etc.) and handles the rest.
:::

**macOS / Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

---

## Step 2: Initialize

```bash
skillshare init
```

<p>
  <img src="/img/init-with-mode.png" alt="Interactive init flow" width="720" />
</p>

This:
1. Creates your source directory (`~/.config/skillshare/skills/`)
2. Auto-detects installed AI CLIs
3. Sets up configuration
4. Optionally installs the built-in skillshare skill (adds `/skillshare` command to AI CLIs)

### Init mode tip

`init` supports `--mode` to set your starting sync behavior:

```bash
skillshare init --mode copy
```

- `merge` (default): per-skill links, local target skills preserved
- `copy`: real files, compatibility-first
- `symlink`: whole target dir linked

If you later run discover with mode:

```bash
skillshare init --discover --select cursor --mode copy
```

`--mode` is applied only to targets added in that discover run. Existing targets are not modified.

:::tip Built-in Skill
During init, you'll be prompted: `Install built-in skillshare skill? [y/N]`. This adds a skill that lets your AI CLI manage skillshare directly. You can skip it and install later with `skillshare upgrade --skill`.
:::

**With git remote (recommended for cross-machine sync):**
```bash
skillshare init --remote git@github.com:you/my-skills.git
```

If the remote already has skills (e.g., from another machine), they'll be pulled automatically during init.

---

## Step 3: Install your first skill

```bash
# Browse available skills
skillshare install anthropics/skills

# Or install directly
skillshare install anthropics/skills/skills/pdf
```

Skills are automatically scanned for security threats during install. If critical issues are found, the install is blocked — use `--force` to override.

---

## Step 4: Sync to all targets

```bash
skillshare sync
```

Your skill is now available in all your AI CLIs.

---

## Verify

```bash
skillshare status
```

You should see:
- Source directory with your skill
- Targets (Claude, Cursor, etc.) showing "synced"

---

## What's Next?

- [Create your own skill](/docs/guides/creating-skills)
- [Sync across machines](/docs/guides/cross-machine-sync)
- [Organization-wide skills](/docs/guides/organization-sharing)

## What Just Happened?

Here's what skillshare did behind the scenes:

1. **`init`** — Created `~/.config/skillshare/config.yaml` and `~/.config/skillshare/skills/`. Auto-detected your AI CLIs (Claude, Cursor, etc.) and added them as targets.

2. **`install`** — Cloned the skill from GitHub into your source directory (`~/.config/skillshare/skills/`). Ran a security audit automatically.

3. **`sync`** — Applied each target's configured sync mode (default is merge, which creates per-skill symlinks). For example:
   ```
   ~/.claude/skills/pdf → ~/.config/skillshare/skills/pdf  (symlink)
   ```

This means:
- **Mode-aware behavior** — Merge/symlink modes reflect source edits immediately; copy mode updates on next `sync`
- **Non-destructive** — Existing target-local skills are preserved in merge/copy mode
- **Reversible** — `skillshare backup` creates snapshots; `skillshare restore` reverts

Need a different behavior for one target? Use per-target overrides:

```bash
skillshare target <name> --mode copy
skillshare sync
```

See [Sync Modes](/docs/concepts/sync-modes) for the decision matrix and trade-offs.

## See Also

- [Core Concepts](/docs/concepts) — Understand source, targets, and sync modes
- [Daily Workflow](/docs/workflows/daily-workflow) — How to use skillshare day-to-day
- [Creating Skills](/docs/guides/creating-skills) — Write your own skills
