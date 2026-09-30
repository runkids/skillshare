---
sidebar_position: 2
---

# First Sync

A complete first-time setup, in order. Roughly five minutes from install to a working sync. Two variations — restoring on another machine, and running unattended on a headless box — are documented at the end of this page.

## Prerequisites

- macOS, Linux, or Windows
- At least one AI CLI installed (Claude Code, Cursor, Codex, etc.)

## 1. Install the CLI

**Homebrew (macOS / Linux):**
```bash
brew install skillshare
```

:::note
Homebrew releases can lag behind by a few days. For the latest, use the install script.
:::

**Install script (macOS / Linux):**
```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

The script installs to `~/.local/bin` by default, so normal installs and updates do not need `sudo`. If the installer prints PATH setup instructions, follow them before running `skillshare`. Add the suggested line to your shell config (such as `~/.zshrc` or `~/.bashrc`) for future terminals. Set `INSTALL_DIR` to use another location.

Existing installations are not moved or removed. If an older binary takes precedence in PATH, the installer reports its location; put the new installation first in PATH to use it. A protected custom directory can still require `sudo`.

For a system-wide installation:

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | INSTALL_DIR=/usr/local/bin sh
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

:::tip Updating later
`skillshare upgrade` detects how you installed (Homebrew, script, manual) and updates the CLI in place.
:::

## 2. Initialize

```bash
skillshare init
```

<p>
  <img src="/img/init-with-mode.png" alt="Interactive init flow" width="720" />
</p>

`init` walks you through four choices:

1. **Source directory** — defaults to `~/.config/skillshare/skills/`. Press Enter to accept.
2. **Git remote** — paste the URL of your personal skills repo (e.g. `git@github.com:you/skills.git`). If you don't have one yet, create an empty repo on GitHub first; you can also skip and add a remote later.
3. **Targets** — skillshare detects installed AI CLIs and lists them. Confirm, or deselect any you don't want.
4. **Built-in skill** — optional. Adds a `/skillshare` command so your AI CLI can invoke skillshare directly.

### Choosing a sync mode

`init` accepts `--mode <merge|copy|symlink>` to set the default for newly-added targets:

- `merge` (default) — per-skill symlinks; pre-existing target-local skills are preserved
- `symlink` — the whole target directory becomes one symlink (fastest, replaces the directory)
- `copy` — real files; changes apply on the next `sync`

Per-target overrides are available later via `skillshare target <name> --mode <mode>`.

## 3. Install a skill

```bash
skillshare install anthropics/skills/skills/pdf
```

Every install runs a security audit. Critical findings block the install; pass `--force` only when you've reviewed and accept the risk.

## 4. Sync

```bash
skillshare sync
```

Every configured target now points at your source.

## 5. Verify

```bash
skillshare status
```

```text
$ skillshare status

Source
─────────────────────────────────────────
✓ ~/.config/skillshare/skills (43 skills, 2026-09-28 12:39)
✓ ~/.config/skillshare/agents (2 agents, 2026-09-28 12:39)

Targets
─────────────────────────────────────────
claude
  skills   merged       [merge] ~/.claude/skills (43 shared, 0 local)
  agents   merged       [merge] 2/2 linked
cursor
  skills   merged       [merge] ~/.cursor/skills (43 shared, 0 local)
  agents   merged       [merge] 2/2 linked
gemini
  skills   merged       [merge] ~/.gemini/skills (43 shared, 0 local)
…
```

The output shows the source path and every target. A synced target in `merge` mode reads `merged`, and its shared count includes the skill you just installed.

The dashboard (`skillshare ui`) shows the same state at a glance:

![Dashboard after the first sync: one source connected to every target, all in sync](/img/web-dashboard-demo.png)

---

## What just happened

1. **`init`** created `~/.config/skillshare/config.yaml` and `~/.config/skillshare/skills/`, auto-detected your AI CLIs, and — if you supplied a remote — cloned any pre-existing skills from it.
2. **`install`** cloned the skill into the source directory and ran a security audit. `.metadata.json` records the upstream URL and commit so `skillshare update` can pull future changes.
3. **`sync`** applied each target's configured mode. For example, in `merge` mode:
   ```
   ~/.claude/skills/pdf → ~/.config/skillshare/skills/pdf  (symlink)
   ```

In `merge` and `symlink` modes, edits to source appear instantly in every target. In `copy` mode they apply on the next `sync`. Pre-existing target-local skills are preserved in `merge` and `copy`; `skillshare backup` snapshots before destructive operations and `skillshare restore <target>` reverts.

Need a different mode for one target only? Override per target:

```bash
skillshare target <name> --mode copy
skillshare sync
```

See [Sync Modes](/docs/understand/sync-modes) for the full decision matrix.

---

## Variation: restoring on another machine

You already use skillshare elsewhere and have a personal skills repo on GitHub. On a new laptop, devcontainer, or VM, four commands restore everything — no prompts, no choices, idempotent on re-run:

```bash
# 1. Install the CLI (Homebrew or curl|sh — same as Step 1 above)
brew install skillshare

# 2. Clone your skills repo and add detected targets
skillshare init \
  --remote git@github.com:<you>/skills.git \
  --all-targets \
  --no-skill

# 3. Re-install tracked dependencies
#    (the _-prefixed dirs are gitignored, so they aren't in the cloned repo)
skillshare install https://github.com/<your-company>/skills --track --force

# 4. Sync
skillshare sync
```

`--no-skill` skips the built-in skill prompt; add it later with `skillshare upgrade --skill` if you want it on this machine.

---

## Variation: headless setup (no TTY)

For CI jobs, devcontainer post-create hooks, or cloud-VM provisioners, every prompt has a non-interactive flag:

```bash
skillshare init \
  --source ~/.config/skillshare/skills \
  --remote https://github.com/<you>/skills \
  --targets codex \
  --mode merge \
  --no-copy \
  --no-skill

skillshare install https://github.com/<your-company>/skills --track --force
skillshare sync
```

| Flag | Effect |
|---|---|
| `--source <path>` | Skip the source-path prompt |
| `--remote <url>` | Skip the remote prompt; clone if remote has content |
| `--targets <name>` | Add only the listed targets (use `--all-targets` to add every detected one) |
| `--mode merge` | Default sync mode for new targets |
| `--no-copy` | Skip the "copy existing target skills?" prompt; start empty |
| `--no-skill` | Skip the built-in skill prompt |

`--targets`, `--all-targets`, and `--no-targets` are mutually exclusive — pick one.

---

## What's next

- [Create your own skill](/docs/how-to/daily-tasks/creating-skills)
- [Sync across machines](/docs/how-to/sharing/cross-machine-sync)
- [Organization-wide skills](/docs/how-to/sharing/organization-sharing)
- [Agents](/docs/understand/agents) — manage single-file `.md` agents alongside skills
- [Sync modes](/docs/understand/sync-modes) — decision matrix and trade-offs
