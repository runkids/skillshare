<p align="center" style="margin-bottom: 0;">
  <img src=".github/assets/logo.png" alt="skillshare" width="280">
</p>

<h1 align="center" style="margin-top: 0.5rem; margin-bottom: 0.5rem;">skillshare</h1>

<p align="center">
  <a href="https://skillshare.runkids.cc"><img src="https://img.shields.io/badge/Website-skillshare.runkids.cc-blue?logo=docusaurus" alt="网站"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/许可证-MIT-yellow.svg" alt="License: MIT"></a>
  <a href="https://github.com/runkids/skillshare/releases"><img src="https://img.shields.io/github/v/release/runkids/skillshare" alt="发布版本"></a>
  <img src="https://img.shields.io/badge/平台-macOS%20%7C%20Linux%20%7C%20Windows-blue" alt="平台">
  <a href="https://goreportcard.com/report/github.com/runkids/skillshare"><img src="https://goreportcard.com/badge/github.com/runkids/skillshare" alt="Go 质量报告"></a>
  <a href="https://deepwiki.com/runkids/skillshare"><img src="https://deepwiki.com/badge.svg" alt="DeepWiki 提问"></a>
</p>

<p align="center">
  <a href="https://github.com/runkids/skillshare/stargazers"><img src="https://img.shields.io/github/stars/runkids/skillshare?style=social" alt="在 GitHub 上点亮 Star"></a>
</p>

<p align="center">
  <a href="https://trendshift.io/repositories/21835" target="_blank"><img src="https://trendshift.io/api/badge/repositories/21835" alt="runkids%2Fskillshare | Trendshift" style="width: 250px; height: 55px;" width="250" height="55"/></a>
</p>

<p align="center">
  <strong>AI CLI 技能（Skills）、智能体（Agents）、规则（Rules）、命令（Commands）等资源的唯一事实来源。</strong><br>
  一键同步到所有平台——从个人到组织级全覆盖。<br>
  支持 Codex、Claude Code、OpenClaw、OpenCode 及 60+ 更多工具。
</p>

<p align="center">
  <img src=".github/assets/demo.gif" alt="skillshare 演示" width="960">
</p>

<p align="center">
  <a href="https://skillshare.runkids.cc">官网</a> •
  <a href="#安装">安装</a> •
  <a href="#快速开始">快速开始</a> •
  <a href="#亮点功能">亮点功能</a> •
  <a href="#cli-和-ui-预览">截图预览</a> •
  <a href="https://skillshare.runkids.cc/docs">文档</a>
</p>

> [!NOTE]
> **最新版本**: [v0.19.12](https://github.com/runkids/skillshare/releases/tag/v0.19.12) — config.yaml 中的 `skills:` 字段现在会保留（修复了团队共享问题）。[查看全部版本 →](https://github.com/runkids/skillshare/releases)

## 为什么选择 skillshare

每个 AI CLI 都有自己的技能目录。
你在一个工具里编辑了技能，却忘了复制到另一个，最后记不清哪个在哪里。

skillshare 解决了这个问题：

- **单一来源，覆盖所有智能体** — 一条 `skillshare sync` 命令同步到 Claude、Cursor、Codex 及 60+ 工具
- **智能体管理** — 将自定义智能体与技能一起同步到支持智能体的目标端
- **不止于技能** — 使用 [extras](https://skillshare.runkids.cc/docs/reference/targets/configuration#extras) 管理规则、命令、提示词及任何基于文件的资源
- **从任何地方安装** — GitHub、GitLab、Bitbucket、Azure DevOps 或任何自托管的 Git 仓库
- **内置安全** — 在使用前审计技能是否存在提示注入和数据泄露风险
- **团队就绪** — 项目中通过 `.skillshare/` 管理技能，组织级技能通过代码仓库同步
- **本地轻量** — 单一二进制文件，无需注册中心，无遥测，完全支持离线使用
- **细粒度过滤** — 通过 [`.skillignore`](https://skillshare.runkids.cc/docs/how-to/daily-tasks/filtering-skills)、SKILL.md 中的 `targets` 字段以及按目标端的 include/exclude 配置，精确控制哪些技能同步到哪些目标端

> 从其他工具迁移？[迁移指南](https://skillshare.runkids.cc/docs/how-to/advanced/migration) · [功能对比](https://skillshare.runkids.cc/docs/understand/philosophy/comparison)

## 工作原理

- macOS / Linux：`~/.config/skillshare/`
- Windows：`%AppData%\skillshare\`

```
┌─────────────────────────────────────────────────────────────┐
│                    源目录                                    │
│   ~/.config/skillshare/skills/    ← 技能（SKILL.md）         │
│   ~/.config/skillshare/agents/    ← 智能体                    │
│   ~/.config/skillshare/extras/    ← 规则、命令等              │
└─────────────────────────────────────────────────────────────┘
                              │ sync
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
       ┌───────────┐   ┌───────────┐   ┌───────────┐
       │  Claude   │   │  OpenCode │   │ OpenClaw  │   ...
       └───────────┘   └───────────┘   └───────────┘
```

| 平台 | 技能源目录 | 智能体源目录 | 扩展资源源目录 | 链接方式 |
|----------|---------------|---------------|---------------|-----------|
| macOS/Linux | `~/.config/skillshare/skills/` | `~/.config/skillshare/agents/` | `~/.config/skillshare/extras/` | 符号链接 |
| Windows | `%AppData%\skillshare\skills\` | `%AppData%\skillshare\agents\` | `%AppData%\skillshare\extras\` | NTFS 交接点（无需管理员权限） |

| | 命令式（逐命令安装） | 声明式（skillshare） |
|---|---|---|
| **事实来源** | 技能各自独立复制 | 单一来源 → 符号链接（或复制） |
| **新机器配置** | 重新手动执行每次安装 | `git clone` 配置 + `sync` |
| **安全审计** | 无 | 内置 `audit` + 安装/更新时自动扫描 |
| **Web 仪表盘** | 无 | `skillshare ui` |
| **运行时依赖** | Node.js + npm | 无（单一 Go 二进制文件） |

> [完整对比 →](https://skillshare.runkids.cc/docs/understand/philosophy/comparison)

## CLI 和 UI 预览

| 技能详细页 | 安全审计 |
|---|---|
| <img src=".github/assets/skill-detail-tui.png" alt="CLI 同步输出" width="480" height="300"> | <img src=".github/assets/audit-tui.png" alt="CLI 安装附带安全审计" width="480" height="300"> |

| UI 仪表盘 | UI 技能列表 |
|---|---|
| <img src=".github/assets/ui/web-dashboard-demo.png" alt="Web 仪表盘概览" width="480"> | <img src=".github/assets/ui/web-skills-demo.png" alt="Web UI 技能页面" width="480"> |

## 安装

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

### Homebrew

```bash
brew install skillshare
```

> **提示：** 运行 `skillshare upgrade` 即可更新到最新版本。它会自动检测你的安装方式并完成后续操作。

### GitHub Actions

```yaml
- uses: runkids/setup-skillshare@v1
  with:
    source: ./skills
- run: skillshare sync
```

查看 [`setup-skillshare`](https://github.com/marketplace/actions/setup-skillshare) 获取所有选项（审计、项目模式、版本锁定等）。

### 缩写别名（可选）

在 shell 配置（`~/.zshrc` 或 `~/.bashrc`）中添加别名：

```bash
alias ss='skillshare'
```

## 快速开始

```bash
skillshare init            # 创建配置、源目录并检测目标端
skillshare sync            # 将技能同步到所有目标端
```

## 亮点功能

**安装和更新技能** — 从 GitHub、GitLab 或任何 Git 仓库

```bash
skillshare install github.com/reponame/skills
skillshare update --all
skillshare target claude --mode copy  # 如果符号链接不适用
```

**符号链接有问题？** — 为每个目标端切换到复制模式

```bash
skillshare target <名称> --mode copy
skillshare sync
```

**安全审计** — 在技能到达智能体之前进行扫描

```bash
skillshare audit
```

**项目级技能** — 按仓库管理，随代码一起提交

```bash
skillshare init -p && skillshare sync
```

**智能体** — 将自定义智能体同步到支持智能体的目标端

```bash
skillshare sync agents            # 仅同步智能体
skillshare sync --all             # 同步技能 + 智能体 + 扩展资源
```

**扩展资源** — 管理规则、命令、提示词等

```bash
skillshare extras init rules          # 创建一个 "rules" 扩展
skillshare sync --all                 # 同步技能 + 扩展资源
skillshare extras collect rules       # 将本地文件收集回源目录
```

**Shell 自动补全** — Tab 键补全命令、标志和子命令

```bash
skillshare completion bash --install   # 也支持：zsh、fish、powershell、nushell
```

**Web 仪表盘** — 可视化控制面板

```bash
skillshare ui
```

[所有命令和指南 →](https://skillshare.runkids.cc/docs/reference/commands)

## 参与贡献

欢迎贡献！请先提交 issue，然后提交带测试的草稿 PR。
查看 [CONTRIBUTING.md](CONTRIBUTING.md) 了解开发环境设置。

```bash
git clone https://github.com/runkids/skillshare.git && cd skillshare
make check  # 格式化 + 代码检查 + 测试
```

> [!TIP]
> 不知道从哪里开始？浏览 [open issues](https://github.com/runkids/skillshare/issues) 或尝试 [Playground](https://skillshare.runkids.cc/docs/learn/with-playground) 获取零配置开发环境。

## 贡献者

感谢所有帮助 skillshare 变得更好的人。

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

---

如果 skillshare 对你有帮助，不妨点个 ⭐ 支持一下

## Star 历史

[![Star 历史图](https://api.star-history.com/svg?repos=runkids/skillshare&type=date&legend=top-left)](https://www.star-history.com/#runkids/skillshare&type=date&legend=top-left)

---

## 许可证

MIT
