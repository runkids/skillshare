---
sidebar_position: 1
---

# 命令

skillshare 全部命令的完整参考。

## 你想做什么？

| 我想要... | Command |
|--------------|---------|
| 第一次设置 skillshare | [`init`](./init.md) |
| 从 GitHub 安装一个 Skill | [`install`](./install.md) |
| 创建我自己的 Skill | [`new`](./new.md) |
| 将 Skill 同步到所有 AI CLI | [`sync`](./sync.md) |
| 检查有哪些内容不同步 | [`status`](./status.md) / [`diff`](./diff.md) |
| 搜索社区 Skill | [`search`](./search.md) |
| 更新已安装的 Skill | 先用 [`check`](./check.md) 再用 [`update`](./update.md) |
| 临时隐藏一个 Skill 而不删除它 | [`enable` / `disable`](./enable.md) |
| 从在别处编辑的链接仓库发现 Skill | [`follow`](./follow.md) / [`unfollow`](./unfollow.md) |
| 用 git 保存或同步变更 | [`commit`](./commit.md) / [`push`](./push.md) / [`pull`](./pull.md) |
| 一次性为每个工具设置 MCP server | [`mcp`](./mcp.md) |
| 在受支持的工具间管理完整的插件 | [`plugin`](./plugin.md) |
| 管理非 Skill 资源（rules、commands） | [`extras`](./extras.md) |
| 管理单文件 `.md` Agent | 大多数命令都支持 `agents` 或 `--kind agent` — 参见 [Agents](/docs/understand/agents) |
| 查看哪些 Skill 占用最多上下文 token | [`analyze`](./analyze.md) |
| 修复问题 | [`doctor`](./doctor.md) |
| 在我的 shell 中启用 Tab 补全 | [`completion`](./completion.md) |
| 打开 Web 控制台 | [`ui`](./ui.md) |

---

## 概览

| Category | Commands |
|----------|----------|
| **Core** | `init`、`install`、`uninstall`、`list`、`search`、`sync`、`status` |
| **Skill Management** | `new`、`check`、`update`、`upgrade`、`enable`、`disable`、`follow`、`unfollow` |
| **MCP Connections** | `mcp`（`add`、`edit`、`import`、`list`、`remove`、`restore`）、`sync mcp` |
| **Plugin Management** | `plugin`（`list`、`discover`、`add`、`import`、`inspect`、`sync`、`check`、`update`、`enable`、`disable`、`remove`） |
| **Target Management** | `target`、`diff` |
| **Extras Management** | `extras`（`init`、`list`、`remove`、`collect`） |
| **Sync Operations** | `collect`、`backup`、`restore`、`trash`、`commit`、`push`、`pull` |
| **Security & Utilities** | `analyze`、`audit`、`hub`、`log`、`doctor`、`tui`、`ui`、`completion`、`version` |

---

## 核心命令

| Command | Description |
|---------|-------------|
| [init](./init.md) | 首次设置 |
| [install](./install.md) | 从仓库或路径添加一个 Skill |
| [uninstall](./uninstall.md) | 移除一个 Skill |
| [list](./list.md) | 列出所有 Skill |
| [search](./search.md) | 搜索 Skill |
| [sync](./sync.md) | 将 Skill 推送到所有 Target |
| [status](./status.md) | 显示同步状态 |

## Skill 管理

| Command | Description |
|---------|-------------|
| [new](./new.md) | 创建一个新的 Skill |
| [check](./check.md) | 检查是否有可用更新 |
| [update](./update.md) | 更新一个 Skill 或 tracked repo |
| [upgrade](./upgrade.md) | 升级 CLI 或内置 Skill |
| [enable / disable](./enable.md) | 临时启用或禁用 Skill |
| [follow](./follow.md) | 跟随 skills source 中的第一层链接 |
| [unfollow](./unfollow.md) | 停止跟随第一层链接 |

## Target 管理

| Command | Description |
|---------|-------------|
| [target](./target.md) | 管理 Target |
| [diff](./diff.md) | 显示 Source 与 Target 之间的差异 |

## Extras 管理

| Command | Description |
|---------|-------------|
| [extras](./extras.md) | 管理非 Skill 资源（rules、commands、prompts） |

## MCP 与 Plugin

| Command | Description |
|---------|-------------|
| [mcp](./mcp.md) | 定义一次 MCP server，并同步到每个工具的原生配置中 |
| [hooks](./hooks.md) | 管理原生 hooks |
| [plugin](./plugin.md) | 安装完整的插件，并选择哪些工具接收它们 |

## Sync 操作

| Command | Description |
|---------|-------------|
| [collect](./collect.md) | 从 Target 收集 Skill 到 Source |
| [backup](./backup.md) | 创建 Target 的备份 |
| [restore](./restore.md) | 从备份恢复 Target |
| [trash](./trash.md) | 管理已卸载 Skill 的回收站 |
| [commit](./commit.md) | 创建本地 git commit 而不推送 |
| [push](./push.md) | 提交并推送到 git 远程仓库 |
| [pull](./pull.md) | 从 git 远程仓库拉取并同步 |

## 安全与实用工具

| Command | Description |
|---------|-------------|
| [analyze](./analyze.md) | 分析上下文窗口的使用情况 |
| [audit](./audit.md) | 扫描 Skill 是否存在安全威胁 |
| [log](./log.md) | 查看操作日志和审计日志 |
| [doctor](./doctor.md) | 诊断问题 |
| [tui](./tui.md) | 切换交互式 TUI 模式 |
| [ui](./ui.md) | 启动 Web 控制台 |
| [hub](./hub.md) | 管理 Skill Hub 来源 |
| [completion](./completion.md) | 生成 shell 补全脚本 |
| [version](./version.md) | 显示 CLI 版本 |

---

## 常用 Flag

大多数命令都支持：

| Flag | Description |
|------|-------------|
| `--dry-run`, `-n` | 预览而不进行任何更改 |
| `--help`, `-h` | 显示帮助 |

---

## 快速参考

```bash
# 设置
skillshare init
skillshare init --remote git@github.com:you/skills.git

# 安装 Skill
skillshare install anthropics/skills/skills/pdf
skillshare install github.com/team/skills --track

# 创建 Skill
skillshare new my-skill

# 同步
skillshare sync
skillshare sync --dry-run

# Git 检查点 / 跨机器同步
skillshare commit -m "Update skill"
skillshare push -m "Add skill"
skillshare pull

# 状态
skillshare status
skillshare list
skillshare diff

# 启用/禁用 Skill
skillshare disable draft-*
skillshare enable draft-*

# 维护
skillshare update --all
skillshare analyze
skillshare audit
skillshare log
skillshare doctor
skillshare backup

# TUI 偏好设置
skillshare tui            # 显示当前状态
skillshare tui off        # 禁用交互式 TUI
skillshare tui on         # 重新启用 TUI

# Web UI
skillshare ui
skillshare ui -p          # Project mode

# Hub
skillshare hub list
skillshare hub add https://hub.example.com/index.json

# 检查更新
skillshare check

# 回收站管理
skillshare trash list
skillshare trash restore my-skill

# Shell 补全
skillshare completion bash --install
skillshare completion zsh --install

# 版本
skillshare version
```

---

## 相关资料

- [Quick Reference](/docs/getting-started/quick-reference) — 命令速查表
- [Workflows](/docs/how-to/daily-tasks) — 常见使用模式
