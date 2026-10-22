---
sidebar_position: 3
---

# Recipe: Private Enterprise Skills

> 使用 token 验证从私有仓库安装 Skill。

## Scenario

你的组织在私有 GitHub/GitLab 仓库中托管内部 Skill。你需要在不将凭证暴露在 config 文件中的情况下安装和更新这些 Skill。

## Solution

### Step 1: 设置身份验证

skillshare 会从环境变量中检测 token，平台专属变量的优先级高于通用的 fallback 变量：

| Platform | Environment Variable |
|----------|---------------------|
| GitHub / GitHub Enterprise | `GITHUB_TOKEN` |
| GitLab / Self-hosted GitLab | `GITLAB_TOKEN` |
| Bitbucket | `BITBUCKET_TOKEN`（+ 可选的 `BITBUCKET_USERNAME`） |
| Azure DevOps | `AZURE_DEVOPS_TOKEN` |
| Gitea / Self-hosted Gitea | `GITEA_TOKEN` |
| CNB | `CNB_TOKEN` |
| 任意平台（fallback） | `SKILLSHARE_GIT_TOKEN` |

```bash
# 选项 A：Git credential helper（推荐用于 GitHub）
gh auth login   # 为 HTTPS 设置 git credential helper

# 选项 B：平台专属环境变量
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxx      # GitHub
export GITLAB_TOKEN=glpat-xxxxxxxxxxxxx    # GitLab
export AZURE_DEVOPS_TOKEN=your-pat-here    # Azure DevOps

# 选项 C：通用 fallback（适用于任何 HTTPS host）
export SKILLSHARE_GIT_TOKEN=your-token-here
```

### Step 2: 从私有仓库安装

```bash
skillshare install your-org/internal-skills --track
```

skillshare 会自动从上述环境变量中检测 token。

### Step 3: 验证 tracking

```bash
skillshare list
```

已安装的仓库会带有 `_` 前缀（tracked repository）：

```
_your-org-internal-skills/
├── code-review/
├── testing-standards/
└── deployment-checklist/
```

### Step 4: 更新周期

```bash
skillshare check    # 检测上游变更
skillshare update   # 拉取最新版本
skillshare sync     # 推送到 Targets
```

## Verification

- `skillshare list` 显示 tracked 仓库
- `skillshare check` 能连接到 remote 并比较哈希值
- `skillshare sync` 在所有 Targets 中创建 symlink

## Variations

- **选择性安装**：`skillshare install your-org/internal-skills --skill code-review` 只安装一个 Skill
- **CI/CD token**：在流水线中，通过 CI secrets 设置对应平台的环境变量（例如 `GITHUB_TOKEN`）
- **Self-hosted GitLab**：设置 `GITLAB_TOKEN` 并使用 HTTPS URL：`skillshare install https://gitlab.internal.com/team/skills.git --track`
- **Self-hosted Gitea**：设置 `GITEA_TOKEN`。如果主机名不包含 `gitea`，还需将其列入 [`gitea_hosts`](/docs/reference/targets/configuration#gitea_hosts)
- **Gitee / AtomGit**：通过 HTTPS URL 配合 `SKILLSHARE_GIT_TOKEN` 支持

## Related

- [`install` command reference](/docs/reference/commands/install)
- [`update` command reference](/docs/reference/commands/update)
- [Organization sharing guide](/docs/how-to/sharing/organization-sharing)
- [URL formats reference](/docs/reference/appendix/url-formats)
