---
sidebar_position: 6
---

# Recipe: Team Onboarding

> 在 5 分钟内为新团队成员搭建好 AI Skill 环境。

## Scenario

一位新开发者加入你的团队。他们需要：
- 组织范围的 Skill（编码规范、审查指南）
- Project 专属的 Skill（领域知识、架构规则）
- 一切在他们的 AI 工具（Claude Code、Cursor 等）上正常工作

## Solution

### Step 1: 创建一个 onboarding 脚本

将其保存为团队 wiki 或仓库中的 `scripts/setup-skills.sh`：

```bash
#!/bin/bash
set -e

echo "Installing skillshare..."
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
export PATH="$HOME/.local/bin:$PATH"

echo "Initializing..."
skillshare init

echo "Installing organization skills..."
skillshare install your-org/org-skills

echo "Running security audit..."
skillshare audit

echo "Syncing to all AI tools..."
skillshare sync

echo "Done! Run 'skillshare list' to see installed skills."
```

### Step 2: 新员工运行该脚本

```bash
curl -fsSL https://your-org.github.io/setup-skills.sh | sh
```

或者如果该脚本在团队仓库中：

```bash
git clone your-org/team-tools
./team-tools/scripts/setup-skills.sh
```

### Step 3: Project 专属设置

当新员工克隆一个 Project 时：

```bash
cd your-project
skillshare sync -p
```

这会自动获取 Project 范围的 Skill。

### Step 4: 验证一切正常

```bash
# 检查 Global Skill
skillshare list

# 检查 Project Skill
skillshare list -p

# 检查 Sync 状态
skillshare status
```

## Verification

- `skillshare list` 显示组织 Skill
- `skillshare status` 显示所有 Targets 均已 Sync
- 打开 Claude Code / Cursor 显示 Skill 已加载

## Variations

- **Dev container onboarding**：如果你的团队使用 dev container，将 skillshare 添加到 `.devcontainer/Dockerfile` 和 `postCreateCommand` — 容器启动时 Skill 即已就绪
- **基于 Homebrew 的安装**：对于 macOS/Linux 团队，用 `brew install skillshare` 替代 `curl | sh`
- **Hub discovery**：让新员工访问你的 hub：`skillshare search --hub https://your-org.github.io/skillshare-hub.json`

## Related

- [Getting started guide](/docs/getting-started)
- [Organization sharing](/docs/how-to/sharing/organization-sharing)
- [Project setup](/docs/how-to/sharing/project-setup)
- [Dev container guide](/docs/learn/with-devcontainer)
