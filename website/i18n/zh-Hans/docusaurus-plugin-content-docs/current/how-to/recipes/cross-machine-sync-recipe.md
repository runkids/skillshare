---
sidebar_position: 5
---

# Recipe: Cross-Machine Sync

> 使用 git push/pull 在多台机器之间保持 Skill 同步。

## Scenario

你在台式机和笔记本电脑（或家里和办公室的机器）上工作。你希望在不需要在每台机器上重新运行安装命令的情况下，在任何地方都拥有相同的 Skill 库。

## Solution

### 初始设置（机器 A）

```bash
# 初始化 skillshare
skillshare init

# 安装你的 Skill
skillshare install your-org/team-skills
skillshare install another/repo --into tools

# 将 Source 推送到 git remote
skillshare push
```

`skillshare push` 会将你的 Source 目录提交到一个 git 跟踪的分支，并推送到已配置的 remote。

### 在新机器上设置（机器 B）

```bash
# 安装 skillshare
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

只有安装器显示 PATH 设置提示时，才需要按提示设置后再运行下方命令。没有 PATH 警告就不需要额外设置。

```bash

# 初始化
skillshare init

# 从 remote 拉取
skillshare pull

# Sync 到本地 Targets
skillshare sync
```

### 日常 Sync Workflow

在任意机器上：

```bash
# 拉取其他机器的最新变更
skillshare pull

# Sync 到本地 AI 工具
skillshare sync

# 在本地进行修改后
skillshare push

# 或者一步完成双向同步：合并、push，然后 sync
skillshare push --pull
```

## Verification

- `skillshare push` 以 0 退出并报告已提交的变更
- 在另一台机器上执行 `skillshare pull` 显示已接收的变更
- `skillshare list` 在两台机器上显示相同的 Skill
- `skillshare sync` 在目标机器上创建 symlink

## Variations

- **登录时自动 Sync**：在你的 shell 配置文件（`.bashrc` / `.zshrc`）中添加 `skillshare pull && skillshare sync`
- **冲突解决**：`pull` 会合并两台机器的 commit，并自动解决 `.metadata.json` 的冲突。如果两台机器编辑了同一个 Skill 文件，`pull` 会停止、撤销合并并列出该文件 — 请在 Source 目录中用 git 解决
- **选择性 Sync**：在 `config.yaml` 中使用每个 Target 的 `include` / `exclude` 过滤器，控制哪些 Skill Sync 到每台机器
- **Plugins、MCP 和 hooks**：这些都存储在 `config.yaml` 中，`push` / `pull` 永远不会带走。请参阅 [Plugins, MCP and Hooks](/docs/how-to/sharing/cross-machine-sync#plugins-mcp-hooks)

## Related

- [Cross-machine sync guide](/docs/how-to/sharing/cross-machine-sync)
- [`push` command reference](/docs/reference/commands/push)
- [`pull` command reference](/docs/reference/commands/pull)
