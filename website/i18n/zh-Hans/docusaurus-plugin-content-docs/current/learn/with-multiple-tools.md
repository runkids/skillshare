---
sidebar_position: 4
---

# 在多个 AI 工具中使用 skillshare

> 单一真实来源，同步到你所使用的每一个 AI CLI。

## 问题所在

你在工作中使用 Claude Code，业余项目用 Cursor，实验性任务用 Codex。每个工具都有自己的 Skill 目录。手动保持它们同步既繁琐又容易出错。

## 解决方案

skillshare 维护一个单一的 Source 目录，并用一条命令 Sync 到你所有的 Target。

## 步骤 1：安装与初始化

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

只有安装器显示 PATH 设置提示时，才需要按提示设置后再运行下方命令。没有 PATH 警告就不需要额外设置。

```bash
skillshare init
```

`init` 会自动检测所有已安装的 AI 工具，并将它们加入为 Target。

## 步骤 2：检查你的 Target

```bash
skillshare target list
```

示例输出：

```
  claude       ~/.claude/skills (merge)
  cursor       ~/.cursor/skills (merge)
  opencode     ~/.config/opencode/skills (merge)
  universal    ~/.agents/skills (merge)
```

Codex 没有属于自己的一行：它读取共享的 `~/.agents/skills` 目录，因此由 `universal` Target 涵盖。

## 步骤 3：安装 Skill

```bash
skillshare install runkids/my-skills
skillshare install anthropics/courses/prompt-eng
```

## 步骤 4：Sync 全部

```bash
skillshare sync
```

一条命令即可将所有 Skill 推送到每个 Target。每个 Target 都会得到指回你单一 Source 的符号链接。

## 步骤 5：验证

```bash
skillshare status
```

显示所有 Target 的 Sync 状态——哪些 Skill 已同步、缺失或过期。

## 按 Target 设置模式

不同工具有不同需求。你可以为每个 Target 单独设置 Sync 模式：

```bash
# Cursor 能正常跟随符号链接（默认）
skillshare target cursor --mode merge

# 有些工具需要实际文件
skillshare target opencode --mode copy
```

## 接下来？

- [了解 Sync 模式 →](/docs/understand/sync-modes)
- [跨机器同步 →](/docs/how-to/sharing/cross-machine-sync)
- [团队共享 →](/docs/how-to/sharing/organization-sharing)
