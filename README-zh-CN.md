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
</p>

<p align="center">
  <a href="https://github.com/runkids/skillshare/stargazers"><img src="https://img.shields.io/github/stars/runkids/skillshare?style=social" alt="Star on GitHub"></a>
</p>

<p align="center">
  <a href="https://trendshift.io/repositories/21835" target="_blank"><img src="https://trendshift.io/api/badge/repositories/21835" alt="runkids%2Fskillshare | Trendshift" style="width: 250px; height: 55px;" width="250" height="55"/></a>
</p>

<p align="center">
  <strong>你的 AI 编程环境，随处可用。</strong><br>
  在同一个地方管理 skills、agents、rules、MCP 连接与 hooks。<br>
  适用于 Claude Code、Codex、Pi、OpenCode 等工具。
</p>

<p align="center">
  <a href="https://skillshare.runkids.cc">官方网站</a> •
  <a href="#安装">安装</a> •
  <a href="#快速开始">快速开始</a> •
  <a href="#功能亮点">功能亮点</a> •
  <a href="#cli-与界面预览">界面截图</a> •
  <a href="#desktop-app">桌面应用</a> •
  <a href="https://skillshare.runkids.cc/docs">文档</a>
</p>

<p align="center">
  <img src=".github/assets/demo.gif" alt="skillshare demo" width="960">
</p>

> [!NOTE]
> **最新版本**：v0.24.6 — `diff` 与 Dashboard 的 Sync 标签页不再把已同步的转换后 agent（Codex、OpenCode）列为待同步，并遵循各 Target 的 include/exclude；Targets 列表中 Target 的每一部分都直接链接到对应标签页；Memory 新增 **Refresh** 按钮，agent 会把简短的事实记在 `INDEX.md`；指向 skills 文件夹之外的 skill 名称会被 Dashboard 拒绝。完整的新功能与修复见 [Releases](https://github.com/runkids/skillshare/releases) 和[更新日志](https://skillshare.runkids.cc/changelog)。

## 为什么用 skillshare

切换 AI 工具，不该每次都重新配置环境。
skillshare 把 skills 与其他 AI 资源集中到由你掌控的地方。

- **换工具，继续用你的 skills** — 修改一次，再同步到 Claude Code、Codex、Pi 和你使用的其他工具。
- **换电脑，带着环境走** — 用 Git 管理资源来源，在另一台电脑上 pull。
- **和团队共享** — 项目资源与代码一起管理，共用 skills 通过 tracked repo 分发。

一位同事用 Claude Code，另一位用 Codex。把共用的代码审查清单放在项目的 `.skillshare/` 中一起管理。新成员安装项目声明的 skills，再同步到配置好的工具，就不用从聊天记录复制指令。[团队 onboarding →](https://skillshare.runkids.cc/docs/how-to/recipes/team-onboarding-recipe)

通过桌面 App 或 CLI 在本地管理，[使用前审计 skills](https://skillshare.runkids.cc/docs/reference/commands/audit)，并[选择各工具接收哪些内容](https://skillshare.runkids.cc/docs/how-to/daily-tasks/filtering-skills)。

> 从其他工具迁移过来？ [迁移指南](https://skillshare.runkids.cc/docs/how-to/advanced/migration) · [对比](https://skillshare.runkids.cc/docs/understand/philosophy/comparison)

## CLI 与界面预览

| Skill 详情 | 安全审计 |
|---|---|
| <img src=".github/assets/skill-detail-tui.png" alt="Skill 详情" width="480" height="300"> | <img src=".github/assets/audit-tui.png" alt="安全审计" width="480" height="300"> |

| 网页仪表盘 | 网页 Skills 页面 |
|---|---|
| <img src=".github/assets/ui/web-dashboard-demo.png" alt="网页仪表盘" width="480"> | <img src=".github/assets/ui/web-skills-demo.png" alt="网页 Skills 页面" width="480"> |

## 安装

> [!TIP]
> **通过桌面 App 管理 skillshare。** [下载 Skillshare App](https://github.com/runkids/skillshare-app/releases/latest)，支持 macOS（Apple Silicon）、Windows 和 Linux。首次启动会引导你安装或选择 CLI、选择 AI 工具，并完成第一次同步。[安装指南](https://skillshare.runkids.cc/zh-Hans/docs/getting-started/desktop-app)。

<a id="desktop-app"></a>

### 桌面 App — 图形化设置与日常管理

[Skillshare App](https://github.com/runkids/skillshare-app) 将技能、代理、MCP 和 hooks 整合在桌面窗口中。安装并打开 App，再按照首次启动的引导完成设置。

macOS（Apple Silicon），使用 Homebrew：

```bash
brew tap runkids/tap
brew install --cask skillshare-app
```

**Windows／Linux，或手动安装 macOS 版本：**[下载最新 App 安装包](https://github.com/runkids/skillshare-app/releases/latest)。各平台的详细说明请参阅[桌面 App 安装指南](https://skillshare.runkids.cc/zh-Hans/docs/getting-started/desktop-app)。

### CLI：macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

脚本默认安装到 `~/.local/bin`，正常安装与更新不需要 `sudo`。只有安装器显示 PATH 设置提示时，才需要按提示设置后再运行 `skillshare`。可把提示的命令加入 shell 配置文件（例如 `~/.zshrc` 或 `~/.bashrc`），让之后打开的终端也能使用。可用 `INSTALL_DIR` 指定其他安装位置。

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

### CLI：Homebrew

```bash
brew install skillshare
```

> **提示：** 运行 `skillshare upgrade` 更新到最新版。它会自动判断你的安装方式并处理后续步骤。

### GitHub Actions

```yaml
- uses: runkids/setup-skillshare@v1
  with:
    source: ./skills
- run: skillshare sync
```

所有选项（audit、project 模式、锁定版本）请见 [`setup-skillshare`](https://github.com/marketplace/actions/setup-skillshare)。

### 简写（可选）

在 shell 配置文件（`~/.zshrc` 或 `~/.bashrc`）中加上 alias：

```bash
alias ss='skillshare'
```

## 快速开始

```bash
skillshare init            # 创建配置文件、来源目录，并检测已安装的 target
skillshare sync            # 把 skills 同步到所有 target
```

## 工作原理

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

| 平台 | Skills 来源 | Agents 来源 | Extras 来源 | 链接方式 |
|----------|---------------|---------------|---------------|-----------|
| macOS/Linux | `~/.config/skillshare/skills/` | `~/.config/skillshare/agents/` | `~/.config/skillshare/extras/` | Symlinks |
| Windows | `%AppData%\skillshare\skills\` | `%AppData%\skillshare\agents\` | `%AppData%\skillshare\extras\` | 文件夹用 NTFS Junction（不需要管理员权限）；文件 symlink 需要开启 Developer Mode，否则改为复制 |

| | 命令式（每次单独安装） | 声明式（skillshare） |
|---|---|---|
| **单一来源** | skills 各自复制，互不相关 | 一份来源，以 symlink（或复制）分发 |
| **新电脑的配置** | 手动重跑每一次安装 | `git clone` 配置，再 `sync` |
| **安全审计** | 无 | 内置 `audit`，install 和 update 时自动扫描 |
| **网页仪表盘** | 无 | `skillshare ui` |
| **运行时依赖** | Node.js + npm | 无（单一 Go 可执行文件） |

> [完整对比 →](https://skillshare.runkids.cc/docs/understand/philosophy/comparison)

## 功能亮点

**安装与更新 skills** — 来源可以是 GitHub、GitLab 或任何 Git 主机

```bash
skillshare install github.com/reponame/skills
skillshare update --all
skillshare target claude --mode copy  # symlink 不能用的时候
```

**Symlink 有问题？** — 单个 target 可以改用 copy 模式

```bash
skillshare target <name> --mode copy
skillshare sync
```

**安全审计** — 在 skills 进入 agent 之前先扫描

```bash
skillshare audit
```

**项目 skills** — 跟着 repo 走，和代码一起 commit

```bash
skillshare init -p && skillshare sync
```

**Agents** — 把自定义 agent 同步到支持 agent 的 target

```bash
skillshare sync agents            # 只同步 agents
skillshare sync --all             # skills、agents、extras、MCP、hooks 一起同步
```

**Extras** — 管理 rules、commands、prompts 等资源

```bash
skillshare extras init rules          # 创建名为 "rules" 的 extra
skillshare sync --all                 # skills、agents、extras、MCP、hooks 一起同步
skillshare extras collect rules       # 把本地文件收回来源
```

**MCP 连接** — 配置一次，Claude Code、Codex、Pi、VS Code、OpenCode 等工具都能用

```bash
skillshare mcp add                    # 引导式配置，输入 URL 或粘贴 JSON
skillshare sync mcp --dry-run         # 预览各工具配置文件会有的变更
skillshare sync mcp                   # 应用连接设置
```

定义可以放在 `config.yaml`，或另外引用一个 `mcp.yaml`。
示例、环境变量引用，以及导入已有连接的方式，请见 [MCP 配置](https://skillshare.runkids.cc/docs/how-to/daily-tasks/sharing-mcp)。

管理原生 hooks，管理操作不执行 hook：

```bash
skillshare hooks add check --file ./check.yaml
skillshare hooks sync --dry-run
skillshare hooks sync
```

**Plugins** — 安装完整的 plugin，并选择哪些工具要安装

```bash
skillshare plugin add                 # 引导式：来源、plugin、target、确认
skillshare plugin add owner/repo --target claude --target codex --no-tui
skillshare sync plugins --dry-run     # plugin 的同步独立于 sync --all
```

工具里已经装好的 plugin 可以用 `plugin import` 纳入管理。
请见[跨工具管理 plugin](https://skillshare.runkids.cc/docs/how-to/daily-tasks/sharing-plugins)。

**Shell 自动补全** — 用 Tab 补全命令、flag 和子命令

```bash
skillshare completion bash --install   # 也支持 zsh、fish、powershell、nushell
```

**本地检查点** — commit 来源目录的变更，但不 push

```bash
skillshare commit -m "Update review skill"
skillshare commit --dry-run
```

**网页仪表盘** — 可视化的控制面板

```bash
skillshare ui
```

[所有命令与指南 →](https://skillshare.runkids.cc/docs/reference/commands)

## 参与贡献

欢迎贡献！请先开 issue，再提交带测试的 draft PR。
环境配置请见 [CONTRIBUTING.md](CONTRIBUTING.md)。

```bash
git clone https://github.com/runkids/skillshare.git && cd skillshare
make check  # format + lint + test
```

> [!TIP]
> 不知道从哪里开始？看看 [open issues](https://github.com/runkids/skillshare/issues)，或试试 [Playground](https://skillshare.runkids.cc/docs/learn/with-playground)，无需任何配置就有开发环境。

## 贡献者

感谢每一位让 skillshare 变得更好的人。完整名单在[英文版 README](README.md#contributors)。

---

如果 skillshare 对你有帮助，欢迎给个 ⭐

## Star History

[![Star History Chart](https://star-history.dera.page/svg?repos=runkids/skillshare&type=date&legend=top-left)](https://star-history.dera.page/#runkids/skillshare&type=date&legend=top-left)

---

## 许可证

MIT
