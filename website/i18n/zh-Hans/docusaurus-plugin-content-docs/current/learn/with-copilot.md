---
sidebar_position: 2
---

# 在 GitHub Copilot 中使用 skillshare

> 从安装到首次 Sync — 只需 5 分钟。

## 前置条件

- 已在 VS Code 或 JetBrains 中启用 [GitHub Copilot](https://github.com/features/copilot) coding agent
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

这会检测到 Copilot 的 Skill 目录（`~/.copilot/skills/`），并自动将其加入为 Target。

## 步骤 3：切换到 copy 模式（推荐）

我们收到反馈，Copilot 有时无法正确跟随符号链接。为避免此问题，请将 Copilot 的 Target 切换为 **copy 模式**：

```bash
skillshare target copilot --mode copy
```

copy 模式会将 Skill 文件实际复制到 `~/.copilot/skills/`，而不是建立符号链接。代价是 Source 的修改不会立即反映——你需要运行 `skillshare sync` 来同步变更。但它在各平台上更加可靠。

:::tip 何时使用 merge（符号链接）模式
如果你使用的是 macOS 或 Linux，且 Copilot 在你的机器上能正确读取符号链接，那么默认的 merge 模式也可以正常运作。你随时可以切换回去：

```bash
skillshare target copilot --mode merge
```
:::

## 步骤 4：安装你的第一个 Skill

```bash
skillshare install runkids/my-skills
```

## 步骤 5：Sync

```bash
skillshare sync
```

Skill 会被复制到 `~/.copilot/skills/`。Copilot 会将它们当作自定义指令来使用。

## 步骤 6：验证

```bash
ls ~/.copilot/skills/
```

你应该会看到已安装的 Skill，它们以实际目录（copy 模式）或符号链接（merge 模式）的形式存在。

## Copilot 专属说明

- **Skill 路径**：`~/.copilot/skills/`（Global mode）或 `.github/skills/`（Project mode）
- **Agent 路径**：`~/.copilot/agents/`（Global mode）或 `.github/agents/`（Project mode）——Copilot CLI 读取的自定义 agent 格式与 skillshare 管理的 `.agent.md` 格式相同，因此 `skillshare sync agents` 可以直接分发它们而无需转换。参见 [Agents](/docs/understand/agents)。
- **Project mode**：执行 `skillshare init -p` 来管理项目级别的 Copilot Skill——它们会被放入与你的代码库并列的 `.github/skills/` 中
- **符号链接问题**：如果 Copilot 没有取用你的 Skill，请检查你的 Target 是否处于 merge 模式（`skillshare status`），并按上述方式切换到 copy 模式
- **`.github/copilot-instructions.md`**：如果你已有现成的指令文件，skillshare 的 Skill 会与它互补——不会取代它

## 接下来？

- [管理多个 Skill →](/docs/how-to/daily-tasks/organizing-skills)
- [与团队共享 →](/docs/how-to/sharing/organization-sharing)
- [探索更多 Skill →](/docs/reference/commands/search)
