---
sidebar_position: 6
---

# Recipe: Team Onboarding

> 让新成员取得团队共享的 skills 与项目上下文。

## Scenario

一位新开发者加入你的团队。他们需要：
- 组织范围的 Skill（编码规范、审查指南）
- Project 专属的 Skill（领域知识、架构规则）
- 一切在他们的 AI 工具（Claude Code、Pi 等）上正常工作

一位同事用 Claude Code，另一位用 Codex，新成员则用 Pi。三人都需要一份说明哪些旧版 API 应避免使用的审查清单。把清单复制到各个工具，会产生多个版本，随着项目变动逐渐分歧。

将项目的本地 skills、`.skillshare/config.yaml` 与 `.skillshare/skills.lock.json` 放在项目仓库。Config 声明远端 skills 与 targets；lockfile 记录远端 skill 的 commit。每位成员在本地应用这些文件。共享指令提供共同上下文，各工具仍保有自己的权限与行为。

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
skillshare init -g

echo "Installing organization skills..."
skillshare install github.com/your-org/org-skills --track -g

echo "Running security audit..."
skillshare audit -g --threshold high

echo "Syncing to all AI tools..."
skillshare sync -g

echo "Done! Run 'skillshare list -g' to see installed skills."
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

维护者先依[项目设置](/docs/how-to/sharing/project-setup)完成配置，并提交项目 config、本地 skills 与 lockfile。新成员克隆该项目后：

```bash
cd your-project
skillshare install -p
skillshare audit -p --threshold high
skillshare sync -p
```

`install -p` 安装 `.skillshare/config.yaml` 声明的远端 skills；若有锁定的 commit，就使用该版本。`audit -p` 检查项目 skills，包括已提交的本地 skills。`sync -p` 将它们分发到配置的 targets。请按顺序运行；任何一步失败就停止。被 audit 拦截的内容需要先审查，再同步。

Pull 项目更新后，重复这个流程。Git 传输 config 与 lockfile，不会自行安装缺少的远端 skills 或更新 target 副本。有意更新 skill 时，通过 PR 审查，并提交产生的 lockfile 变更。

### Step 4: 验证一切正常

```bash
# 检查 Global Skill
skillshare list -g

# 检查 Project Skill
skillshare list -p

# 检查 Sync 状态
skillshare status -p
```

## Verification

- `skillshare list -g` 显示组织 skills
- `skillshare list -p` 显示项目的本地与已安装的远端 skills
- `skillshare status -p` 显示配置的 Project targets 已同步
- 打开配置的 AI 工具，确认预期的 skills 已加载；用已知的旧版 API 示例试跑审查清单

## Variations

- **Dev container onboarding**：如果你的团队使用 dev container，将 skillshare 添加到 `.devcontainer/Dockerfile` 和 `postCreateCommand` — 容器启动时 Skill 即已就绪
- **基于 Homebrew 的安装**：对于 macOS/Linux 团队，用 `brew install skillshare` 替代 `curl | sh`
- **Hub discovery**：让新员工访问你的 hub：`skillshare search --hub https://your-org.github.io/skillshare-hub.json`

## Related

- [Getting started guide](/docs/getting-started)
- [Organization sharing](/docs/how-to/sharing/organization-sharing)
- [Project setup](/docs/how-to/sharing/project-setup)
- [Dev container guide](/docs/learn/with-devcontainer)
