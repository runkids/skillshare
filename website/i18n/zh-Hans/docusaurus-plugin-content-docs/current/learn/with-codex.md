---
sidebar_position: 3
---

# 在 Codex 中使用 skillshare

> 从安装到首次 Sync — 只需 5 分钟。

## 前置条件

- 已安装并正常运作的 [OpenAI Codex CLI](https://github.com/openai/codex)
- macOS、Linux 或 Windows

## 步骤 1：安装 skillshare

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

只有安装器显示 PATH 设置提示时，才需要按提示设置后再运行下方命令。没有 PATH 警告就不需要额外设置。

## 步骤 2：初始化

```bash
skillshare init
```

Codex 会读取 `~/.agents/skills/`，这是它在文档中说明的用户级 Skill 路径，也是各工具共享的目录。`init` 会通过 Codex 的配置目录（`~/.codex/`）检测到它，并自动为它设置共享的 `universal` Target。

## 步骤 3：安装你的第一个 Skill

```bash
skillshare install runkids/my-skills
```

## 步骤 4：Sync

```bash
skillshare sync
```

Skill 会以符号链接的形式链接到 `~/.agents/skills/`。

## 步骤 5：验证

```bash
ls ~/.agents/skills/
```

你应该会看到已安装的 Skill 以符号链接的形式出现。

## Codex 专属说明

- **Skill 路径**：`~/.agents/skills/`（Global mode）或 `.agents/skills/`（Project mode）
- **既有设置**：如果你的配置中 `codex` 仍指向 `~/.codex/skills`，Codex 依然会读取该目录。但若同时启用了 `universal`，每个 Skill 都会出现两次——请移除 `codex` Target（可先用 `skillshare target remove codex --dry-run` 预览）
- **描述长度限制**：Codex 对 Skill 描述有 1024 字符的限制。请让 `SKILL.md` frontmatter 中的 `description` 字段保持简洁
- **Project mode**：执行 `skillshare init -p` 来管理项目级别的 Codex Skill

## 接下来？

- [管理多个 Skill →](/docs/how-to/daily-tasks/organizing-skills)
- [与团队共享 →](/docs/how-to/sharing/organization-sharing)
- [探索更多 Skill →](/docs/reference/commands/search)
