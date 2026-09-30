---
sidebar_position: 1
---

# 在 Claude Code 中使用 skillshare

> 从安装到首次 Sync — 只需 5 分钟。

## 前置条件

- 已安装并正常运作的 [Claude Code](https://docs.anthropic.com/en/docs/claude-code/overview)
- macOS、Linux 或 Windows（WSL）

## 步骤 1：安装 skillshare

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

只有安装器显示 PATH 设置提示时，才需要按提示设置后再运行下方命令。没有 PATH 警告就不需要额外设置。

## 步骤 2：初始化

```bash
skillshare init
```

这会检测到 Claude Code 的 Skill 目录（`~/.claude/skills/`），并自动将其加入为 Target。

## 步骤 3：安装你的第一个 Skill

```bash
skillshare install anthropics/courses/prompt-eng
```

该 Skill 会被下载、进行安全审计，并加入到你的 Source 目录中。

## 步骤 4：Sync

```bash
skillshare sync
```

这会从你的 Source 建立指向 `~/.claude/skills/` 的符号链接。Claude Code 会立即取用这些 Skill——无需重新启动。

## 步骤 5：验证

```bash
ls ~/.claude/skills/
```

你应该会看到已安装的 Skill 以符号链接的形式出现。

## Claude Code 整合细节

- **Skill 路径**：`~/.claude/skills/`（Global mode）或 `.claude/skills/`（Project mode）
- **CLAUDE.md**：skillshare 的 Skill 使用 `SKILL.md` 格式，Claude Code 原生支持读取
- **Project mode**：在仓库内执行 `skillshare init -p`，即可按项目管理 `.claude/skills/`

## 接下来？

- [管理多个 Skill →](/docs/how-to/daily-tasks/organizing-skills)
- [与团队共享 →](/docs/how-to/sharing/organization-sharing)
- [探索更多 Skill →](/docs/reference/commands/search)
