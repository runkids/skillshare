<p align="center" style="margin-bottom: 0;">
  <img src=".github/assets/skillshare-logo-card.png" alt="skillshare" width="280">
</p>

<h1 align="center" style="margin-top: 0.5rem; margin-bottom: 0.5rem;">skillshare</h1>

<p align="center">
  <a href="README.md">English</a> · <a href="README-ja.md">日本語</a> · <a href="README-ko.md">한국어</a> · <a href="README-zh-CN.md">简体中文</a> · <a href="README-zh-TW.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://skillshare.runkids.cc"><img src="https://img.shields.io/badge/Website-skillshare.runkids.cc-blue?logo=docusaurus" alt="Website"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <a href="https://github.com/runkids/skillshare/releases"><img src="https://img.shields.io/github/v/release/runkids/skillshare" alt="Release"></a>
  <a href="https://github.com/runkids/skillshare/releases"><img src="https://img.shields.io/github/downloads/runkids/skillshare/total" alt="Downloads"></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-blue" alt="Platform">
  <a href="https://deepwiki.com/runkids/skillshare"><img src="https://deepwiki.com/badge.svg" alt="Ask DeepWiki"></a>
  <a href="https://ko-fi.com/williehung"><img src="https://img.shields.io/badge/Support-skillshare-FF5E5B?logo=kofi&logoColor=white" alt="Support skillshare on Ko-fi"></a>
</p>

<p align="center">
  <a href="https://github.com/runkids/skillshare/stargazers"><img src="https://img.shields.io/github/stars/runkids/skillshare?style=social" alt="Star on GitHub"></a>
</p>

<p align="center">
  <a href="https://trendshift.io/repositories/21835" target="_blank"><img src="https://trendshift.io/api/badge/repositories/21835" alt="runkids%2Fskillshare | Trendshift" style="width: 250px; height: 55px;" width="250" height="55"/></a>
</p>

<p align="center">
  <strong>Your AI coding setup, everywhere.</strong><br>
  Manage skills, agents, rules, MCP connections and hooks in one place.<br>
  For Claude Code, Codex, Pi, OpenCode and more.
</p>

<p align="center">
  <a href="https://skillshare.runkids.cc">Website</a> •
  <a href="#installation">Install</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#highlights">Highlights</a> •
  <a href="#cli-and-ui-preview">Screenshots</a> •
  <a href="#desktop-app">Desktop App</a> •
  <a href="https://skillshare.runkids.cc/docs">Docs</a>
</p>

<p align="center">
  <img src=".github/assets/demo.gif" alt="skillshare demo" width="960">
</p>

> [!NOTE]
> **Latest**: v0.24.6 — `diff` and the dashboard's Sync tab no longer list converted agents (Codex, OpenCode) as pending after a sync, and follow each target's include/exclude; the Targets list links each part of a target straight to its tab; Memory gets a **Refresh** button, and agents keep short facts in `INDEX.md`; and the dashboard rejects skill names that point outside the skills folder. [All releases →](https://github.com/runkids/skillshare/releases)

## Why skillshare

Switching AI tools should not mean rebuilding your setup.
skillshare gives your skills and other AI resources a home you control.

- **Switch tools, keep your skills** — edit once, then sync to Claude Code, Codex, Pi and the other tools you use.
- **Take your setup with you** — version your source in Git and pull it onto another machine.
- **Share with your team** — keep project resources with your code and distribute shared skills through tracked repositories.

One teammate uses Claude Code, another uses Codex. Keep their shared code-review checklist in `.skillshare/` alongside the project. New teammates install the declared skills and sync to the configured tools instead of copying instructions from chat. [Team onboarding →](https://skillshare.runkids.cc/docs/how-to/recipes/team-onboarding-recipe)

Use the desktop app or CLI to manage everything locally, [audit skills before use](https://skillshare.runkids.cc/docs/reference/commands/audit), and [choose what each tool receives](https://skillshare.runkids.cc/docs/how-to/daily-tasks/filtering-skills).

> Coming from another tool? [Migration Guide](https://skillshare.runkids.cc/docs/how-to/advanced/migration) · [Comparison](https://skillshare.runkids.cc/docs/understand/philosophy/comparison)

## CLI and UI Preview

| Skill Detail | Security Audit |
|---|---|
| <img src=".github/assets/skill-detail-tui.png" alt="CLI sync output" width="480" height="300"> | <img src=".github/assets/audit-tui.png" alt="CLI install with security audit" width="480" height="300"> |

| UI Dashboard | UI Skills |
|---|---|
| <img src=".github/assets/ui/web-dashboard-demo.png" alt="Web dashboard overview" width="480"> | <img src=".github/assets/ui/web-skills-demo.png" alt="Web UI skills page" width="480"> |

## Installation

> [!TIP]
> **Use skillshare from your desktop.** [Get Skillshare App](https://github.com/runkids/skillshare-app/releases/latest) for macOS (Apple Silicon), Windows and Linux. First launch helps you install or locate the CLI, choose your AI tools and run your first sync. [Installation guide](https://skillshare.runkids.cc/docs/getting-started/desktop-app).

<a id="desktop-app"></a>

### Desktop App — visual setup and daily management

[Skillshare App](https://github.com/runkids/skillshare-app) puts skills, agents, MCP and hooks in a desktop window. Install the app, open it, and follow first-launch setup.

macOS (Apple Silicon), with Homebrew:

```bash
brew tap runkids/tap
brew install --cask skillshare-app
```

**Windows / Linux, or a manual macOS install:** [Download the latest app installers](https://github.com/runkids/skillshare-app/releases/latest). See the [Desktop App guide](https://skillshare.runkids.cc/docs/getting-started/desktop-app) for platform details.

### CLI: macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

The script installs to `~/.local/bin` by default, so normal installs and updates do not need `sudo`. If the installer prints PATH setup instructions, follow them before running `skillshare`. Add the suggested line to your shell config (such as `~/.zshrc` or `~/.bashrc`) for future terminals. Set `INSTALL_DIR` to use another location.

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

### CLI: Homebrew

```bash
brew install skillshare
```

> **Tip:** Run `skillshare upgrade` to update to the latest version. It auto-detects your install method and handles the rest.

### GitHub Actions

```yaml
- uses: runkids/setup-skillshare@v1
  with:
    source: ./skills
- run: skillshare sync
```

See [`setup-skillshare`](https://github.com/marketplace/actions/setup-skillshare) for all options (audit, project mode, version pinning).

### Shorthand (Optional)

Add an alias to your shell config (`~/.zshrc` or `~/.bashrc`):

```bash
alias ss='skillshare'
```

## Quick Start

```bash
skillshare init            # Create config, source, and detected targets
skillshare sync            # Sync skills to all targets
```

## How It Works

- macOS / Linux: `~/.config/skillshare/`
- Windows: `%AppData%\skillshare\`

```
┌─────────────────────────────────────────────────────────────┐
│                    Source Directory                         │
│   ~/.config/skillshare/skills/    ← skills (SKILL.md)       │
│   ~/.config/skillshare/agents/    ← agents                  │
│   ~/.config/skillshare/extras/    ← rules, commands, etc.   │
└─────────────────────────────────────────────────────────────┘
                              │ sync
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
       ┌───────────┐   ┌───────────┐   ┌───────────┐
       │  Claude   │   │  OpenCode │   │ OpenClaw  │   ...
       └───────────┘   └───────────┘   └───────────┘
```

| Platform | Skills Source | Agents Source | Extras Source | Link Type |
|----------|---------------|---------------|---------------|-----------|
| macOS/Linux | `~/.config/skillshare/skills/` | `~/.config/skillshare/agents/` | `~/.config/skillshare/extras/` | Symlinks |
| Windows | `%AppData%\skillshare\skills\` | `%AppData%\skillshare\agents\` | `%AppData%\skillshare\extras\` | NTFS Junctions for folders (no admin required); file symlinks need Developer Mode, otherwise copied |

| | Imperative (install-per-command) | Declarative (skillshare) |
|---|---|---|
| **Source of truth** | Skills copied independently | Single source → symlinks (or copies) |
| **New machine setup** | Re-run every install manually | `git clone` config + `sync` |
| **Security audit** | None | Built-in `audit` + auto-scan on install/update |
| **Web dashboard** | None | `skillshare ui` |
| **Runtime dependency** | Node.js + npm | None (single Go binary) |

> [Full comparison →](https://skillshare.runkids.cc/docs/understand/philosophy/comparison)

## Highlights

**Install & update skills** —from GitHub, GitLab, or any Git host

```bash
skillshare install github.com/reponame/skills
skillshare update --all
skillshare target claude --mode copy  # if symlinks don't work
```

**Symlink issues?** — switch to copy mode per target

```bash
skillshare target <name> --mode copy
skillshare sync
```

**Security audit** —scan before skills reach your agent

```bash
skillshare audit
```

**Project skills** —per-repo, committed with your code

```bash
skillshare init -p && skillshare sync
```

**Agents** —sync custom agents to agent-capable targets

```bash
skillshare sync agents            # sync agents only
skillshare sync --all             # sync skills + agents + extras + MCP + hooks together
```

**Extras** —manage rules, commands, prompts & more

```bash
skillshare extras init rules          # create a "rules" extra
skillshare sync --all                 # sync skills + agents + extras + MCP + hooks together
skillshare extras collect rules       # collect local files back to source
```

**MCP connections** —configure once for Claude Code, Codex, Pi, VS Code, OpenCode and more

```bash
skillshare mcp add                    # guided URL or JSON setup
skillshare sync mcp --dry-run         # preview native configuration changes
skillshare sync mcp                   # apply connection settings
```

Keep definitions in `config.yaml` or reference a separate `mcp.yaml`.
See [MCP setup](https://skillshare.runkids.cc/docs/how-to/daily-tasks/sharing-mcp)
for examples, environment references and importing existing connections.

Manage native hooks without executing them:

```bash
skillshare hooks add check --file ./check.yaml
skillshare hooks sync --dry-run
skillshare hooks sync
```

**Plugins** —install a complete plugin and pick which tools receive it

```bash
skillshare plugin add                 # guided: source, plugin, targets, review
skillshare plugin add owner/repo --target claude --target codex --no-tui
skillshare sync plugins --dry-run     # plugins sync separately from sync --all
```

Existing native installations can be adopted with `plugin import`.
See [Manage plugins across tools](https://skillshare.runkids.cc/docs/how-to/daily-tasks/sharing-plugins).

**Shell completion** —tab-complete commands, flags, and subcommands

```bash
skillshare completion bash --install   # also: zsh, fish, powershell, nushell
```

**Local checkpoints** — commit source changes without pushing

```bash
skillshare commit -m "Update review skill"
skillshare commit --dry-run
```

**Web dashboard** — visual control panel

```bash
skillshare ui
```

[All commands & guides →](https://skillshare.runkids.cc/docs/reference/commands)

## Contributing

Contributions welcome! Open an issue first, then submit a draft PR with tests.
See [CONTRIBUTING.md](CONTRIBUTING.md) for setup details.

```bash
git clone https://github.com/runkids/skillshare.git && cd skillshare
make check  # format + lint + test
```

> [!TIP]
> Not sure where to start? Browse [open issues](https://github.com/runkids/skillshare/issues) or try the [Playground](https://skillshare.runkids.cc/docs/learn/with-playground) for a zero-setup dev environment.

## Contributors

Thanks to everyone who helped shape skillshare.

<a href="https://github.com/leeeezx"><img src="https://github.com/leeeezx.png" width="50" style="border-radius:50%" alt="leeeezx"></a>
<a href="https://github.com/Vergil333"><img src="https://github.com/Vergil333.png" width="50" style="border-radius:50%" alt="Vergil333"></a>
<a href="https://github.com/romanr"><img src="https://github.com/romanr.png" width="50" style="border-radius:50%" alt="romanr"></a>
<a href="https://github.com/xocasdashdash"><img src="https://github.com/xocasdashdash.png" width="50" style="border-radius:50%" alt="xocasdashdash"></a>
<a href="https://github.com/philippe-granet"><img src="https://github.com/philippe-granet.png" width="50" style="border-radius:50%" alt="philippe-granet"></a>
<a href="https://github.com/terranc"><img src="https://github.com/terranc.png" width="50" style="border-radius:50%" alt="terranc"></a>
<a href="https://github.com/benrfairless"><img src="https://github.com/benrfairless.png" width="50" style="border-radius:50%" alt="benrfairless"></a>
<a href="https://github.com/nerveband"><img src="https://github.com/nerveband.png" width="50" style="border-radius:50%" alt="nerveband"></a>
<a href="https://github.com/EarthChen"><img src="https://github.com/EarthChen.png" width="50" style="border-radius:50%" alt="EarthChen"></a>
<a href="https://github.com/gdm257"><img src="https://github.com/gdm257.png" width="50" style="border-radius:50%" alt="gdm257"></a>
<a href="https://github.com/skovtunenko"><img src="https://github.com/skovtunenko.png" width="50" style="border-radius:50%" alt="skovtunenko"></a>
<a href="https://github.com/TyceHerrman"><img src="https://github.com/TyceHerrman.png" width="50" style="border-radius:50%" alt="TyceHerrman"></a>
<a href="https://github.com/1am2syman"><img src="https://github.com/1am2syman.png" width="50" style="border-radius:50%" alt="1am2syman"></a>
<a href="https://github.com/thealokkr"><img src="https://github.com/thealokkr.png" width="50" style="border-radius:50%" alt="thealokkr"></a>
<a href="https://github.com/JasonLandbridge"><img src="https://github.com/JasonLandbridge.png" width="50" style="border-radius:50%" alt="JasonLandbridge"></a>
<a href="https://github.com/masonc15"><img src="https://github.com/masonc15.png" width="50" style="border-radius:50%" alt="masonc15"></a>
<a href="https://github.com/richardwhatever"><img src="https://github.com/richardwhatever.png" width="50" style="border-radius:50%" alt="richardwhatever"></a>
<a href="https://github.com/reneleonhardt"><img src="https://github.com/reneleonhardt.png" width="50" style="border-radius:50%" alt="reneleonhardt"></a>
<a href="https://github.com/ndeybach"><img src="https://github.com/ndeybach.png" width="50" style="border-radius:50%" alt="ndeybach"></a>
<a href="https://github.com/hhh2210"><img src="https://github.com/hhh2210.png" width="50" style="border-radius:50%" alt="hhh2210"></a>
<a href="https://github.com/leoarry"><img src="https://github.com/leoarry.png" width="50" style="border-radius:50%" alt="leoarry"></a>
<a href="https://github.com/salmonumbrella"><img src="https://github.com/salmonumbrella.png" width="50" style="border-radius:50%" alt="salmonumbrella"></a>
<a href="https://github.com/daylamtayari"><img src="https://github.com/daylamtayari.png" width="50" style="border-radius:50%" alt="daylamtayari"></a>
<a href="https://github.com/dstotijn"><img src="https://github.com/dstotijn.png" width="50" style="border-radius:50%" alt="dstotijn"></a>
<a href="https://github.com/ipruning"><img src="https://github.com/ipruning.png" width="50" style="border-radius:50%" alt="ipruning"></a>
<a href="https://github.com/massukio"><img src="https://github.com/massukio.png" width="50" style="border-radius:50%" alt="massukio"></a>
<a href="https://github.com/kevincobain2000"><img src="https://github.com/kevincobain2000.png" width="50" style="border-radius:50%" alt="kevincobain2000"></a>
<a href="https://github.com/StephenPAdams"><img src="https://github.com/StephenPAdams.png" width="50" style="border-radius:50%" alt="StephenPAdams"></a>
<a href="https://github.com/mk-imagine"><img src="https://github.com/mk-imagine.png" width="50" style="border-radius:50%" alt="mk-imagine"></a>
<a href="https://github.com/Curtion"><img src="https://github.com/Curtion.png" width="50" style="border-radius:50%" alt="Curtion"></a>
<a href="https://github.com/amdoi7"><img src="https://github.com/amdoi7.png" width="50" style="border-radius:50%" alt="amdoi7"></a>
<a href="https://github.com/jessica-engel"><img src="https://github.com/jessica-engel.png" width="50" style="border-radius:50%" alt="jessica-engel"></a>
<a href="https://github.com/AlimuratYusup"><img src="https://github.com/AlimuratYusup.png" width="50" style="border-radius:50%" alt="AlimuratYusup"></a>
<a href="https://github.com/thor-shuang"><img src="https://github.com/thor-shuang.png" width="50" style="border-radius:50%" alt="thor-shuang"></a>
<a href="https://github.com/bishopmatthew"><img src="https://github.com/bishopmatthew.png" width="50" style="border-radius:50%" alt="bishopmatthew"></a>
<a href="https://github.com/chaosky"><img src="https://github.com/chaosky.png" width="50" style="border-radius:50%" alt="chaosky"></a>
<a href="https://github.com/iFwu"><img src="https://github.com/iFwu.png" width="50" style="border-radius:50%" alt="iFwu"></a>
<a href="https://github.com/ildunari"><img src="https://github.com/ildunari.png" width="50" style="border-radius:50%" alt="ildunari"></a>
<a href="https://github.com/aestilog"><img src="https://github.com/aestilog.png" width="50" style="border-radius:50%" alt="aestilog"></a>
<a href="https://github.com/xarthurx"><img src="https://github.com/xarthurx.png" width="50" style="border-radius:50%" alt="xarthurx"></a>
<a href="https://github.com/m0cun"><img src="https://github.com/m0cun.png" width="50" style="border-radius:50%" alt="m0cun"></a>
<a href="https://github.com/bit3125"><img src="https://github.com/bit3125.png" width="50" style="border-radius:50%" alt="bit3125"></a>
<a href="https://github.com/eekryuos"><img src="https://github.com/eekryuos.png" width="50" style="border-radius:50%" alt="eekryuos"></a>
<a href="https://github.com/Bongseop-Kim"><img src="https://github.com/Bongseop-Kim.png" width="50" style="border-radius:50%" alt="Bongseop-Kim"></a>
<a href="https://github.com/sophodex"><img src="https://github.com/sophodex.png" width="50" style="border-radius:50%" alt="sophodex"></a>
<a href="https://github.com/PeterTianbuhan"><img src="https://github.com/PeterTianbuhan.png" width="50" style="border-radius:50%" alt="PeterTianbuhan"></a>
<a href="https://github.com/dotned"><img src="https://github.com/dotned.png" width="50" style="border-radius:50%" alt="dotned"></a>
<a href="https://github.com/ismferd"><img src="https://github.com/ismferd.png" width="50" style="border-radius:50%" alt="ismferd"></a>
<a href="https://github.com/jblackburn21"><img src="https://github.com/jblackburn21.png" width="50" style="border-radius:50%" alt="jblackburn21"></a>
<a href="https://github.com/jnhu76"><img src="https://github.com/jnhu76.png" width="50" style="border-radius:50%" alt="jnhu76"></a>
<a href="https://github.com/jacobleft"><img src="https://github.com/jacobleft.png" width="50" style="border-radius:50%" alt="jacobleft"></a>
<a href="https://github.com/rhysmcneill"><img src="https://github.com/rhysmcneill.png" width="50" style="border-radius:50%" alt="rhysmcneill"></a>
<a href="https://github.com/druellan"><img src="https://github.com/druellan.png" width="50" style="border-radius:50%" alt="druellan"></a>
<a href="https://github.com/12britz"><img src="https://github.com/12britz.png" width="50" style="border-radius:50%" alt="12britz"></a>
<a href="https://github.com/askpatrickw"><img src="https://github.com/askpatrickw.png" width="50" style="border-radius:50%" alt="askpatrickw"></a>
<a href="https://github.com/Almost42"><img src="https://github.com/Almost42.png" width="50" style="border-radius:50%" alt="Almost42"></a>
<a href="https://github.com/Brett-Best"><img src="https://github.com/Brett-Best.png" width="50" style="border-radius:50%" alt="Brett-Best"></a>
<a href="https://github.com/isCopyman"><img src="https://github.com/isCopyman.png" width="50" style="border-radius:50%" alt="isCopyman"></a>
<a href="https://github.com/svob"><img src="https://github.com/svob.png" width="50" style="border-radius:50%" alt="svob"></a>
<a href="https://github.com/michal-grzelak"><img src="https://github.com/michal-grzelak.png" width="50" style="border-radius:50%" alt="michal-grzelak"></a>
<a href="https://github.com/shikbupt"><img src="https://github.com/shikbupt.png" width="50" style="border-radius:50%" alt="shikbupt"></a>
<a href="https://github.com/LeoYeAI"><img src="https://github.com/LeoYeAI.png" width="50" style="border-radius:50%" alt="LeoYeAI"></a>
<a href="https://github.com/TIR44"><img src="https://github.com/TIR44.png" width="50" style="border-radius:50%" alt="TIR44"></a>
<a href="https://github.com/Ajaymamtora"><img src="https://github.com/Ajaymamtora.png" width="50" style="border-radius:50%" alt="Ajaymamtora"></a>
<a href="https://github.com/vishaldialpad"><img src="https://github.com/vishaldialpad.png" width="50" style="border-radius:50%" alt="vishaldialpad"></a>
<a href="https://github.com/kankan0829"><img src="https://github.com/kankan0829.png" width="50" style="border-radius:50%" alt="kankan0829"></a>
<a href="https://github.com/dnabb"><img src="https://github.com/dnabb.png" width="50" style="border-radius:50%" alt="dnabb"></a>
<a href="https://github.com/thinhngotony"><img src="https://github.com/thinhngotony.png" width="50" style="border-radius:50%" alt="thinhngotony"></a>
<a href="https://github.com/skaurus"><img src="https://github.com/skaurus.png" width="50" style="border-radius:50%" alt="skaurus"></a>
<a href="https://github.com/jamesbraza"><img src="https://github.com/jamesbraza.png" width="50" style="border-radius:50%" alt="jamesbraza"></a>
<a href="https://github.com/2BAB"><img src="https://github.com/2BAB.png" width="50" style="border-radius:50%" alt="2BAB"></a>
<a href="https://github.com/zdlldz"><img src="https://github.com/zdlldz.png" width="50" style="border-radius:50%" alt="zdlldz"></a>
<a href="https://github.com/elstiaan"><img src="https://github.com/elstiaan.png" width="50" style="border-radius:50%" alt="elstiaan"></a>
<a href="https://github.com/EriaWalker"><img src="https://github.com/EriaWalker.png" width="50" style="border-radius:50%" alt="EriaWalker"></a>
<a href="https://github.com/FaintFlower"><img src="https://github.com/FaintFlower.png" width="50" style="border-radius:50%" alt="FaintFlower"></a>
<a href="https://github.com/yantinglin21"><img src="https://github.com/yantinglin21.png" width="50" style="border-radius:50%" alt="yantinglin21"></a>
<a href="https://github.com/chung1912"><img src="https://github.com/chung1912.png" width="50" style="border-radius:50%" alt="chung1912"></a>
<a href="https://github.com/r-fynn"><img src="https://github.com/r-fynn.png" width="50" style="border-radius:50%" alt="r-fynn"></a>
<a href="https://github.com/cescox"><img src="https://github.com/cescox.png" width="50" style="border-radius:50%" alt="cescox"></a>
<a href="https://github.com/hhdebb"><img src="https://github.com/hhdebb.png" width="50" style="border-radius:50%" alt="hhdebb"></a>
<a href="https://github.com/Snurppa"><img src="https://github.com/Snurppa.png" width="50" style="border-radius:50%" alt="Snurppa"></a>
<a href="https://github.com/s0undt3ch"><img src="https://github.com/s0undt3ch.png" width="50" style="border-radius:50%" alt="s0undt3ch"></a>
<a href="https://github.com/danscheer"><img src="https://github.com/danscheer.png" width="50" style="border-radius:50%" alt="danscheer"></a>
<a href="https://github.com/7zq12lvm-b"><img src="https://github.com/7zq12lvm-b.png" width="50" style="border-radius:50%" alt="7zq12lvm-b"></a>
<a href="https://github.com/DarkiT"><img src="https://github.com/DarkiT.png" width="50" style="border-radius:50%" alt="DarkiT"></a>
<a href="https://github.com/harisonw"><img src="https://github.com/harisonw.png" width="50" style="border-radius:50%" alt="harisonw"></a>
<a href="https://github.com/wuhaoyujerry"><img src="https://github.com/wuhaoyujerry.png" width="50" style="border-radius:50%" alt="wuhaoyujerry"></a>
<a href="https://github.com/star-nebula"><img src="https://github.com/star-nebula.png" width="50" style="border-radius:50%" alt="star-nebula"></a>
<a href="https://github.com/AdamMagued"><img src="https://github.com/AdamMagued.png" width="50" style="border-radius:50%" alt="AdamMagued"></a>

---

## Support skillshare ❤️

skillshare is free and open source.

If skillshare saves you time, consider buying me a coffee.
Every contribution helps with ongoing development, cross-platform testing,
documentation, and maintenance.

[☕ Buy me a coffee](https://ko-fi.com/williehung)

You can also help by giving skillshare a ⭐ on GitHub!

## Star History

[![Star History Chart](https://star-history.dera.page/svg?repos=runkids/skillshare&type=date&legend=top-left)](https://star-history.dera.page/#runkids/skillshare&type=date&legend=top-left)

---

## License

MIT
